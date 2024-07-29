package rolloutcmd

import (
	"errors"
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
		if opts.Revision == 0 {
			return errors.New("--revision is required as the rollback requires to provide a revision") //nolint: goerr113
		}
		getter := newObjectSetGetter(client)
		// this function gets the objectset list from [cluster]package/[cluster]ObjectDeployment
		list, err := getter.GetObjectSets(cmd.Context(), args.Resource, args.Name, opts.Namespace)
		if err != nil {
			return err
		}
		if opts.Revision > 0 {
			os, found := list.FindRevision(opts.Revision)
			if !found {
				return errRevisionsNotFound
			}
			//Once we find the objectset
			switch strings.ToLower(args.Resource) {
			case "clusterobjectdeployment", "clusterpackage":
				obs := os.GetClusterObjectsettype()
				if obs == nil {
					if _, err := fmt.Fprint(cmd.OutOrStdout(), "Error while returning ClusterObjectset"); err != nil {
						return err
					}

				}
				//this means that the objectset of that revision is currently in available
				if obs.Status.Phase == corev1alpha1.ObjectSetStatusPhaseAvailable {
					if _, err := fmt.Fprint(cmd.OutOrStdout(), "Can not rollback from an available ClusterObjectSet Type"); err != nil {
						return err
					}
				}
				if obs.Status.Phase == corev1alpha1.ObjectSetStatusPhaseArchived {
					objd, err := getter.client.GetObjectDeployment(cmd.Context(), args.Name)
					if err != nil {
						return err
					}
					if _, err := fmt.Fprintf(cmd.OutOrStdout(), "gonna rollback this ClusterObjectDeployment : %v with objectset %v ", objd.Name(), obs.Name); err != nil {
						return err
					}
					errs := getter.client.PatchClusterObjectDeployment(cmd.Context(), objd.Name(), *obs)
					if errs != nil {
						return errs
					}
					if _, err := fmt.Fprint(cmd.OutOrStdout(), "Successfully rolled back"); err != nil {
						return err
					}
				}

			case "objectdeployment", "package":

				obs := os.GetObjectsettype()
				if obs == nil {
					if _, err := fmt.Fprint(cmd.OutOrStdout(), "Error while returning Objectset"); err != nil {
						return err
					}
				}
				//this means that the objectset of that revision is currently in available
				if obs.Status.Phase == corev1alpha1.ObjectSetStatusPhaseAvailable {
					if _, err := fmt.Fprint(cmd.OutOrStdout(), "Can not rollback from an available ObjectSet Type"); err != nil {
						return err
					}
				}
				if obs.Status.Phase == corev1alpha1.ObjectSetStatusPhaseArchived {

					objd, err := getter.client.GetObjectDeployment(cmd.Context(), args.Name, internalcmd.WithNamespace(opts.Namespace))
					if err != nil {
						return err
					}

					if _, err := fmt.Fprintf(cmd.OutOrStdout(), "gonna rollback this ObjectDeployment : %v in ns : %v with Objectset : %v ", objd.Name(), objd.Namespace(), obs.Name); err != nil {
						return err
					}
					errs := getter.client.PatchObjectDeployment(cmd.Context(), objd.Name(), objd.Namespace(), *obs)
					if errs != nil {
						return errs
					}
					if _, err := fmt.Fprint(cmd.OutOrStdout(), "Successfully rolled back"); err != nil {
						return err
					}
				}

			default:
				return errInvalidResourceType
			}

			//return printer.PrintObjectSet(os, opts)
			return nil
		} else {
			_, err = fmt.Fprint(cmd.OutOrStdout(), "Invalid revision specified")
			if err != nil {
				return err
			}
			return nil
		}
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
