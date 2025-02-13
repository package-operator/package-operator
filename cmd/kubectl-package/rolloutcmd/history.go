package rolloutcmd

import (
	"errors"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"package-operator.run/cmd/kubectl-package/util"
	"package-operator.run/internal/cli"
	internalcmd "package-operator.run/internal/cmd"
)

func NewHistoryCmd(clientFactory internalcmd.ClientFactory) *cobra.Command {
	const (
		cmdUse   = "history"
		cmdShort = "view the history of rollout revisions"
		cmdLong  = "view the history of rollout revisions for a package or object deployment"
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
		args, err := util.ParseResourceName(rawArgs)
		if err != nil {
			return err
		}

		client, err := clientFactory.Client()
		if err != nil {
			return err
		}

		getter := newObjectSetGetter(client)
		list, err := getter.GetObjectSets(cmd.Context(), args.Resource, args.Name, opts.Namespace)
		if err != nil {
			return err
		}

		printer := newPrinter(
			cli.NewPrinter(cli.WithOut{Out: cmd.OutOrStdout()}),
		)

		if opts.Revision > 0 {
			os, found := list.FindRevision(opts.Revision)
			if !found {
				return errRevisionsNotFound
			}

			return printer.PrintObjectSet(os, opts)
		}

		list.Sort()

		return printer.PrintObjectSetList(list, opts)
	}

	return cmd
}

var errRevisionsNotFound = errors.New("revision not found")

type options struct {
	Namespace string
	Output    string
	Revision  int64
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
		&o.Output,
		"output",
		"o",
		o.Output,
		"Output format. One of: json|yaml",
	)
	flags.Int64Var(
		&o.Revision,
		"revision",
		o.Revision,
		"See the details, including object set of the revision specified",
	)
}
