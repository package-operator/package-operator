package packages

import (
	"context"
	"fmt"
	"log"

	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type finalizer struct {
	name    string
	cleaner func(ctx context.Context, c client.Client, obj genericPackage, pkoNamespace string) error
}

var packageFinalizers []finalizer

func init() {
	packageFinalizers = []finalizer{
		{
			name: "package-operator.run/job",
			cleaner: func(ctx context.Context, c client.Client, pkg genericPackage, pkoNamespace string) error {
				jobName, jobNamespace := "job-"+pkg.ClientObject().GetName(), pkoNamespace
				job := &batchv1.Job{}
				if err := c.Get(ctx, types.NamespacedName{Namespace: jobNamespace, Name: jobName}, job); err != nil {
					return client.IgnoreNotFound(err)
				}
				if err := c.Delete(ctx, job); err != nil {
					return fmt.Errorf("failed to delete the Job: %w", err)
				}
				return nil
			},
		},
	}

	finTracker := map[string]bool{}
	for _, fin := range packageFinalizers {
		if finTracker[fin.name] {
			log.Fatal(fmt.Errorf("duplicate finalizers found with the name '%s'", fin.name)) // nolint:goerr113
		}
		finTracker[fin.name] = true
	}
}

func findFinalizerByName(name string) (finalizer, bool) {
	for _, fin := range packageFinalizers {
		if fin.name == name {
			return fin, true
		}
	}
	return finalizer{}, false
}

func getPackageFinalizerNames() []string {
	names := []string{}
	for _, fin := range packageFinalizers {
		names = append(names, fin.name)
	}
	return names
}
