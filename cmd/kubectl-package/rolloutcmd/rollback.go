package rolloutcmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"

	internalcmd "package-operator.run/internal/cmd"
)

func NewRollbackCmd(clientFactory internalcmd.ClientFactory) *cobra.Command {
	const (
		cmdUse   = "rollback"
		cmdShort = "Rollback to a previous rollout revisions"
		cmdLong  = "view the history of rollout revisions for a package or object deployment"
	)

	cmd := &cobra.Command{
		Use:   cmdUse,
		Short: cmdShort,
		Long:  cmdLong,
		Args:  cobra.RangeArgs(1, 2),
	}

	var opts rollbckoptions

	opts.AddRollbackFlags(cmd.Flags())

	cmd.RunE = func(cmd *cobra.Command, rawArgs []string) error {
		args, err := getArgs(rawArgs)
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

		/*printer := newPrinter(
			cli.NewPrinter(cli.WithOut{Out: cmd.OutOrStdout()}),
		)*/

		if opts.Revision > 0 {
			os, found := list.FindRevision(opts.Revision)
			if !found {
				return errRevisionsNotFound
			}

			obs := os.GetClusterObjectsettype()
			if obs.Status.Phase == corev1alpha1.ObjectSetStatusPhaseArchived {
				fmt.Println("Name of archive ", obs.Name)
				// as the cluster object set of that revision is Archived lets do the logic for rollback
			}

			phasename := obs.Spec.Phases[0].Name
			fmt.Printf("phas name  %s", phasename)

			//return printer.PrintObjectSet(os, opts)
			return nil
		}

		list.Sort()

		//return printer.PrintObjectSetList(list, opts)
		return nil
	}

	return cmd
}

type rollbckoptions struct {
	Namespace string
	Revision  int64
}

func (o *rollbckoptions) AddRollbackFlags(flags *pflag.FlagSet) {
	flags.StringVarP(
		&o.Namespace,
		"namespace",
		"n",
		o.Namespace,
		"If present, the namespace scope for this CLI request",
	)
	flags.Int64Var(
		&o.Revision,
		"revision",
		o.Revision,
		"See the details, including object set of the revision specified",
	)
}
