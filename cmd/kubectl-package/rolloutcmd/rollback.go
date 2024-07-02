package rolloutcmd

import (
	"fmt"
	"strings"

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
		args, err := getRollbackArgs(rawArgs)
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
		/*if opts.Revision < 1 {
			fmt.Errorf("no rollout history found for the Objectdeployment")
		}*/
		if opts.Revision > 0 {
			os, found := list.FindRevision(opts.Revision)
			if !found {
				return errRevisionsNotFound
			}

			obs := os.GetClusterObjectsettype()
			//this means that the objectset of that revision is currently in available
			if obs.Status.Phase == corev1alpha1.ObjectSetStatusPhaseAvailable {
				fmt.Println("Can not rollback from an available ObjectSet")
			}
			if obs.Status.Phase == corev1alpha1.ObjectSetStatusPhaseArchived {

				fmt.Println("Name of archive ", obs.Name)

				// as the cluster object set of that revision is Archived lets do the logic for rollback
				//patch only if Objcls deployment is available and revision is > 1

				objd, err := getter.client.GetObjectDeployment(cmd.Context(), args.Name)
				fmt.Println("gonna rollback this [cluster]ObjectDeployment : ", objd.Name())
				//call patch of deployment

				getter.client.PatchClusterObjectDeployment(cmd.Context(), objd.Name(), *obs)
				if err != nil {
					return err
				}

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

func getRollbackArgs(args []string) (*arguments, error) {
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
