package main

import (
	"context"
	"path/filepath"

	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"pkg.package-operator.run/cardboard/run"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

func bootstrap(ctx context.Context) error {
	self := run.Fn1(bootstrap, ctx)

	err := mgr.ParallelDeps(ctx, self,
		run.Meth(cluster, cluster.create),
		run.Meth(generate, generate.All),
	)
	if err != nil {
		return err
	}

	cl, err := cluster.Clients()
	if err != nil {
		return err
	}

	err = cl.CreateAndWaitFromFiles(ctx, []string{filepath.Join("config", "self-bootstrap-job-local.yaml")})
	if err != nil {
		return err
	}

	// Bootstrap job is cleaning itself up after completion, so we can't wait for Condition Completed=True.
	// See self-bootstrap-job .spec.ttlSecondsAfterFinished: 0
	err = cl.Waiter.WaitToBeGone(ctx,
		&batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{Name: "package-operator-bootstrap", Namespace: "package-operator-system"},
		},
		func(client.Object) (done bool, err error) { return },
	)
	if err != nil {
		return err
	}

	err = cl.Waiter.WaitForCondition(ctx,
		&corev1alpha1.ClusterPackage{
			ObjectMeta: metav1.ObjectMeta{
				Name: "package-operator",
			},
		},
		corev1alpha1.PackageAvailable,
		metav1.ConditionTrue,
	)
	if err != nil {
		return err
	}

	return nil
}
