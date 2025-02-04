package pausecmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"package-operator.run/cmd/kubectl-package/util"

	internalcmd "package-operator.run/internal/cmd"
)

var errInvalidResource = errors.New("invalid resource name")

func NewGenericPauseCmd(clientFactory internalcmd.ClientFactory, pause bool) *cobra.Command {
	verb := "pause"
	if !pause {
		verb = "unpause"
	}
	cmd := &cobra.Command{
		Use:   verb,
		Short: verb + " a package",
		Long:  verb + " the reconciliation of a package and its objects",
		Args:  cobra.RangeArgs(1, 2),
	}

	var opts options
	opts.AddFlags(cmd.Flags())

	cmd.RunE = func(cmd *cobra.Command, rawArgs []string) error {
		args, err := util.ParseResourceName(rawArgs)
		if err != nil {
			return err
		}
		client, err := clientFactory.Client()
		if err != nil {
			return err
		}
		scheme, err := internalcmd.NewScheme()
		if err != nil {
			return err
		}

		switch strings.ToLower(args.Resource) {
		case "package":
			if opts.Namespace == "" {
				opts.Namespace = "default"
			}
		case "clusterpackage":
		default:
			return fmt.Errorf("%w: expected `clusterpackage`/`package`, got `%s`", errInvalidResource, args.Resource)
		}

		return client.PackageSetPaused(cmd.Context(), internalcmd.NewWaiter(client, scheme),
			args.Name, opts.Namespace, pause, opts.Message,
		)
	}

	return cmd
}

type options struct {
	Namespace string
	Message   string
}

func (o *options) AddFlags(flags *pflag.FlagSet) {
	flags.StringVarP(
		&o.Namespace,
		"namespace",
		"n",
		o.Namespace,
		"If present, the namespace scope for this CLI request",
	)
	flags.StringVarP(
		&o.Message,
		"message",
		"m",
		o.Message,
		fmt.Sprintf("Reason for pausing, will be stored in the %s annotation.",
			internalcmd.PauseMessageAnnotation),
	)
}
