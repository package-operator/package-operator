package updatecmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/spf13/cobra"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	"package-operator.run/apis/manifests/v1alpha1"
	"package-operator.run/package-operator/cmd/cmdutil"
	"package-operator.run/package-operator/internal/packages"
	"package-operator.run/package-operator/internal/packages/packagecontent"
	"package-operator.run/package-operator/internal/packages/packageimport"
	"package-operator.run/package-operator/internal/packages/packageloader"
	"package-operator.run/package-operator/internal/utils"
)

const (
	updateUse   = "update source_path"
	updateShort = "updates image digests of the specified package"
	updateLong  = "updates image digests of the specified package storing them in the manifest.lock file"
)

var Default = Update{
	LoadPackage: func(ctx context.Context, target string) (*packagecontent.Package, error) {
		var filemap packagecontent.Files

		filemap, err := packageimport.Folder(ctx, target)
		if err != nil {
			return nil, err
		}

		pkg, err := packageloader.New(cmdutil.Scheme).FromFiles(ctx, filemap)
		if err != nil {
			return nil, err
		}

		return pkg, nil
	},
	RetrieveDigest: crane.Digest,
	WriteLockFile: func(path string, data []byte) error {
		return os.WriteFile(path, data, 0o644)
	},
}

type Update struct {
	LoadPackage    func(ctx context.Context, target string) (*packagecontent.Package, error)
	RetrieveDigest func(ref string, opt ...crane.Option) (string, error)
	WriteLockFile  func(path string, data []byte) error
	Target         string
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

func (u Update) Run(ctx context.Context) (err error) {
	pkg, err := u.LoadPackage(ctx, u.Target)
	if err != nil {
		return err
	}

	var lockImages []v1alpha1.PackageManifestLockImage
	for _, img := range pkg.PackageManifest.Spec.Images {
		overriddenImage, err := utils.ImageURLWithOverride(img.Image)
		if err != nil {
			return err
		}
		digest, err := u.RetrieveDigest(overriddenImage)
		if err != nil {
			return err
		}
		lockImages = append(lockImages, v1alpha1.PackageManifestLockImage{Name: img.Name, Image: img.Image, Digest: digest})
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
			Images: lockImages,
		},
	}

	if pkg.PackageManifestLock != nil && lockSpecsAreEqual(&manifestLock.Spec, &pkg.PackageManifestLock.Spec) {
		return nil
	}

	manifestLockYaml, err := yaml.Marshal(manifestLock)
	if err != nil {
		return err
	}

	err = u.WriteLockFile(filepath.Join(u.Target, packages.PackageManifestLockFile), manifestLockYaml)
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
		return u.Run(cmdutil.NewCobraContext(cmd))
	}

	return cmd
}

func lockSpecsAreEqual(spec *v1alpha1.PackageManifestLockSpec, other *v1alpha1.PackageManifestLockSpec) bool {
	thisImages := map[string]v1alpha1.PackageManifestLockImage{}
	for _, image := range spec.Images {
		thisImages[image.Name] = image
	}

	otherImages := map[string]v1alpha1.PackageManifestLockImage{}
	for _, image := range other.Images {
		otherImages[image.Name] = image
	}

	if len(thisImages) != len(otherImages) {
		return false
	}

	for name, image := range thisImages {
		otherImage, exists := otherImages[name]
		if !exists || otherImage != image {
			return false
		}
	}

	return true
}
