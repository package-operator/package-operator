package clustertreecmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	internalcmd "package-operator.run/internal/cmd"
)

func NewClusterTreeCmd(clientFactory internalcmd.ClientFactory) *cobra.Command {
	const (
		cmdUse   = "clustertree"
		cmdShort = "outputs a logical tree view of the package contents and provide arguments in resource/name "
		cmdLong  = "outputs a logical tree view of the package contents of either clusterpackage or package"
	)

	cmd := &cobra.Command{
		Use:   cmdUse,
		Short: cmdShort,
		Long:  cmdLong,
		Args:  cobra.RangeArgs(1, 2),
	}

	var opts options

	opts.AddFlags(cmd.Flags())

	cmd.RunE = func(cmd *cobra.Command, rawArgs []string) error {
		args, err := getArgs(rawArgs)
		if err != nil {
			return err
		}
		clientL, err := clientFactory.Client()
		if err != nil {
			return err
		}
		switch strings.ToLower(args.Resource) {
		case "clusterpackage":
			Package, err := clientL.GetPackage(cmd.Context(), args.Name)
			if err != nil {
				return err
			}
			tree, err := handleClusterPackage(clientL, Package, cmd)
			if err != nil {
				return err
			}
			_, err = fmt.Fprint(cmd.OutOrStdout(), tree)
			if err != nil {
				return err
			}

		case "package":
			if opts.Namespace == "" {
				return errors.New("--namespace is required as its a namespaced Resource type") //nolint: goerr113
			}
			ns := opts.Namespace
			Package, err := clientL.GetPackage(cmd.Context(), args.Name, internalcmd.WithNamespace(ns))
			if err != nil {
				return err
			}
			tree, err := handlePackage(clientL, Package, cmd)
			if err != nil {
				return err
			}
			_, err = fmt.Fprint(cmd.OutOrStdout(), tree)
			if err != nil {
				return err
			}

		default:
			return errInvalidResourceType
		}
		return nil
	}

	return cmd
}

var errInvalidResourceType = errors.New("invalid resource type")

func getArgs(args []string) (*arguments, error) {
	switch len(args) {
	case 1:
		parts := strings.SplitN(args[0], "/", 2)
		if len(parts) < 2 {
			return nil, fmt.Errorf(
				"%w: arguments in resource/name form must have a single resource and name",
				internalcmd.ErrInvalidArgs,
			)
		}

		return &arguments{
			Resource: parts[0],
			Name:     parts[1],
		}, nil
	case 2:
		return &arguments{
			Resource: args[0],
			Name:     args[1],
		}, nil
	default:
		return nil, fmt.Errorf(
			"%w: no less than 1 and no more than 2 arguments may be provided",
			internalcmd.ErrInvalidArgs,
		)
	}
}

type arguments struct {
	Resource string
	Name     string
}

type options struct {
	Namespace string
}

func (o *options) AddFlags(flags *pflag.FlagSet) {
	flags.StringVarP(
		&o.Namespace,
		"namespace",
		"n",
		o.Namespace,
		"If present, the namespace scope for this CLI request",
	)
}
