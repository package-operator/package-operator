package validatecmd

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/spf13/cobra"

	"package-operator.run/package-operator/cmd/kubectl-package/command/cmdutil"
	"package-operator.run/package-operator/internal/packages/packagecontent"
	"package-operator.run/package-operator/internal/packages/packageimport"
	"package-operator.run/package-operator/internal/packages/packageloader"
)

const (
	validateUse     = "validate [--pull] target"
	validateShort   = "validate a package."
	validateLong    = "validate a package. Target may be a source directory, a package in a tar[.gz] or a fully qualified tag if --pull is set."
	validatePullUse = "treat target as image reference and pull it instead of looking on the filesystem"
)

type Validate struct {
	Target          string
	TargetReference name.Reference
	Pull            bool
}

func (v *Validate) Complete(args []string) (err error) {
	switch {
	case len(args) != 1:
		return fmt.Errorf("%w: got %v positional args. Need one argument containing the target", cmdutil.ErrInvalidArgs, len(args))
	case args[0] == "":
		return fmt.Errorf("%w: target path empty", cmdutil.ErrInvalidArgs)
	case v.Pull:
		v.TargetReference, err = name.ParseReference(args[0])
		if err != nil {
			return fmt.Errorf("%w: --pull set and target is not an image reference", cmdutil.ErrInvalidArgs)
		}
	}

	v.Target = args[0]
	return nil
}

func (v Validate) Run(ctx context.Context) (err error) {
	var (
		filemap   packagecontent.Files
		extraOpts []packageloader.Option
	)
	if v.Pull {
		filemap, err = packageimport.PulledImage(ctx, v.TargetReference.String())
		if err != nil {
			return err
		}
	} else {
		filemap, err = packageimport.Folder(ctx, v.Target)
		if err != nil {
			return err
		}

		ttv := packageloader.NewTemplateTestValidator(filepath.Join(v.Target, ".test-fixtures"))
		extraOpts = append(extraOpts, packageloader.WithPackageAndFilesValidators(ttv))
	}

	if _, err := packageloader.New(cmdutil.ValidateScheme, extraOpts...).FromFiles(ctx, filemap); err != nil {
		return err
	}

	return nil
}

func (v *Validate) CobraCommand() *cobra.Command {
	cmd := &cobra.Command{Use: validateUse, Short: validateShort, Long: validateLong}
	f := cmd.Flags()
	f.BoolVar(&v.Pull, "pull", false, validatePullUse)

	cmd.RunE = func(cmd *cobra.Command, args []string) (err error) {
		if err := v.Complete(args); err != nil {
			return err
		}
		return v.Run(cmdutil.NewCobraContext(cmd))
	}

	return cmd
}
