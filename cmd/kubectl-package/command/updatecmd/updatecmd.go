package updatecmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/spf13/cobra"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	"package-operator.run/apis/manifests/v1alpha1"
	"package-operator.run/package-operator/cmd/kubectl-package/command/cmdutil"
	"package-operator.run/package-operator/internal/packages"
	"package-operator.run/package-operator/internal/packages/packagecontent"
	"package-operator.run/package-operator/internal/packages/packageimport"
	"package-operator.run/package-operator/internal/packages/packageloader"
)

const (
	updateUse   = "update source_path"
	updateShort = "updates image digests of the specified package"
	updateLong  = "updates image digests of the specified package storing them in the manifest.lock file"
)

type Update struct {
	Target string
}

func (u *Update) Complete(args []string) (err error) {
	switch {
	case len(args) != 1:
		return fmt.Errorf("%w: got %v positional args. Need one argument containing the target", cmdutil.ErrInvalidArgs, len(args))
	case args[0] == "":
		return fmt.Errorf("%w: target path empty", cmdutil.ErrInvalidArgs)
	}

	u.Target = args[0]
	return nil
}

func (u Update) Run(ctx context.Context, out io.Writer) (err error) {
	var filemap packagecontent.Files

	filemap, err = packageimport.Folder(ctx, u.Target)
	if err != nil {
		return err
	}

	pkg, err := packageloader.New(cmdutil.ValidateScheme).FromFiles(ctx, filemap)
	if err != nil {
		return err
	}

	lockimages := []v1alpha1.PackageManifestLockImage{}
	for _, img := range pkg.PackageManifest.Spec.Images {
		digest, err := crane.Digest(img.Image)
		if err != nil {
			return err
		}
		lockimages = append(lockimages, v1alpha1.PackageManifestLockImage{Name: img.Name, Image: img.Image, Digest: digest})
	}

	manifestLock := &v1alpha1.PackageManifestLock{
		TypeMeta: v1.TypeMeta{
			Kind:       packages.PackageManifestLockGroupKind.Kind,
			APIVersion: v1alpha1.GroupVersion.String(),
		},
		ObjectMeta: v1.ObjectMeta{
			CreationTimestamp: v1.Now(),
		},
		Spec: v1alpha1.PackageManifestLockSpec{
			Images: lockimages,
		},
	}

	if pkg.PackageManifestLock != nil && manifestLock.Spec.Equals(&pkg.PackageManifestLock.Spec) {
		return nil
	}

	manifestLockYaml, err := yaml.Marshal(manifestLock)
	if err != nil {
		return err
	}

	err = os.WriteFile(filepath.Join(u.Target, packages.PackageManifestLockFile), manifestLockYaml, 0644)
	if err != nil {
		return err
	}

	return nil
}

func (u *Update) CobraCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   updateUse,
		Short: updateShort,
		Long:  updateLong,
	}

	cmd.RunE = func(cmd *cobra.Command, args []string) (err error) {
		if err := u.Complete(args); err != nil {
			return err
		}
		return u.Run(cmdutil.NewCobraContext(cmd), cmd.OutOrStdout())
	}

	return cmd
}
