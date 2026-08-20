package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"pkg.package-operator.run/cardboard/run"
	"pkg.package-operator.run/cardboard/sh"
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
		return errors.Join(err, dumpBootstrapDiagnostics(ctx, "create-bootstrap-job"))
	}

	// Job ttlSecondsAfterFinished: 0 deletes pods as soon as it finishes/fails.
	// Snapshot logs while the Job is still around or we lose the panic text.
	snapCtx, stopSnap := context.WithCancel(ctx)
	defer stopSnap()
	go snapshotBootstrapJobLogsLoop(snapCtx)

	// Bootstrap job is cleaning itself up after completion, so we can't wait for Condition Completed=True.
	// See self-bootstrap-job .spec.ttlSecondsAfterFinished: 0
	err = cl.Waiter.WaitToBeGone(ctx,
		&batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{Name: "package-operator-bootstrap", Namespace: "package-operator-system"},
		},
		func(client.Object) (done bool, err error) { return },
	)
	stopSnap()
	if err != nil {
		return errors.Join(err, dumpBootstrapDiagnostics(ctx, "wait-job-gone"))
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
		return errors.Join(err, dumpBootstrapDiagnostics(ctx, "clusterpackage-available"))
	}

	return nil
}

func dumpBootstrapDiagnostics(ctx context.Context, stage string) error {
	fmt.Fprintf(os.Stderr, "\n===== bootstrap diagnostics (%s) =====\n", stage)

	kubeconfigPath, err := cluster.KubeconfigPath()
	if err != nil {
		return fmt.Errorf("kubeconfig path: %w", err)
	}

	debugDir := filepath.Join(cacheDir, "integration", "bootstrap-debug")
	if err := os.MkdirAll(debugDir, 0o755); err != nil {
		return fmt.Errorf("mkdir bootstrap debug dir: %w", err)
	}

	kubectl := shr.New(sh.WithEnvironment{"KUBECONFIG": kubeconfigPath})
	cmds := [][]string{
		{"get", "ns"},
		{"get", "all,jobs,events", "-n", "package-operator-system", "-o", "wide"},
		{"get", "clusterpackage,package", "-A", "-o", "yaml"},
		{"describe", "clusterpackage", "package-operator"},
		{"get", "events", "-A", "--sort-by=.lastTimestamp"},
	}
	for _, args := range cmds {
		_ = kubectl.Run(ctx, "kubectl", args...)
	}

	script := fmt.Sprintf(`
set +e
mkdir -p %q
kubectl get ns > %q/namespaces.txt 2>&1
kubectl get all,jobs -n package-operator-system -o wide > %q/pko-system-workload.txt 2>&1
kubectl get clusterpackage,package -A -o yaml > %q/packages.yaml 2>&1
kubectl describe clusterpackage package-operator > %q/clusterpackage.describe.txt 2>&1
kubectl get events -A --sort-by=.lastTimestamp > %q/events.txt 2>&1
kubectl get pods -n package-operator-system --no-headers -o custom-columns=:metadata.name |
while read -r pod; do
  [ -z "$pod" ] && continue
  kubectl logs -n package-operator-system "$pod" --all-containers --prefix --tail=500 \
    > %q/"$pod".log 2>&1
  kubectl logs -n package-operator-system "$pod" --all-containers --prefix --previous --tail=200 \
    > %q/"$pod".previous.log 2>&1
  echo "----- logs: $pod -----"
  cat %q/"$pod".log
done
`, debugDir, debugDir, debugDir, debugDir, debugDir, debugDir, debugDir, debugDir, debugDir)
	_ = kubectl.Bash(ctx, script)

	if err := cluster.ExportLogs(filepath.Join(cacheDir, "integration", "logs")); err != nil {
		fmt.Fprintf(os.Stderr, "kind export logs: %v\n", err)
	}

	fmt.Fprintf(os.Stderr, "===== end bootstrap diagnostics (%s) =====\n", stage)
	printSavedBootstrapJobLogs()
	return nil
}

func snapshotBootstrapJobLogsLoop(ctx context.Context) {
	_ = snapshotBootstrapJobLogs(ctx)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			_ = snapshotBootstrapJobLogs(context.WithoutCancel(ctx))
			return
		case <-ticker.C:
			_ = snapshotBootstrapJobLogs(ctx)
		}
	}
}

func bootstrapDebugDir() string {
	return filepath.Join(cacheDir, "integration", "bootstrap-debug")
}

func snapshotBootstrapJobLogs(ctx context.Context) error {
	kubeconfigPath, err := cluster.KubeconfigPath()
	if err != nil {
		return err
	}
	debugDir := bootstrapDebugDir()
	if err := os.MkdirAll(debugDir, 0o755); err != nil {
		return err
	}
	kubectl := shr.New(sh.WithEnvironment{"KUBECONFIG": kubeconfigPath})
	script := fmt.Sprintf(`
set +e
ns=package-operator-system
kubectl get job,pods,events -n "$ns" -o wide > %q/job-snapshot.txt 2>&1
: > %q/job-logs.txt
kubectl get pods -n "$ns" --no-headers -o custom-columns=:metadata.name 2>/dev/null |
while read -r pod; do
  [ -z "$pod" ] && continue
  echo "===== $pod current =====" >> %q/job-logs.txt
  kubectl logs -n "$ns" "$pod" --all-containers --prefix --tail=200 >> %q/job-logs.txt 2>&1
  echo "===== $pod previous =====" >> %q/job-logs.txt
  kubectl logs -n "$ns" "$pod" --all-containers --prefix --previous --tail=200 >> %q/job-logs.txt 2>&1
done
`, debugDir, debugDir, debugDir, debugDir, debugDir, debugDir)
	return kubectl.Bash(ctx, script)
}

func printSavedBootstrapJobLogs() {
	path := filepath.Join(bootstrapDebugDir(), "job-logs.txt")
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		fmt.Fprintf(os.Stderr, "no captured bootstrap job logs at %s\n", path)
		return
	}
	fmt.Fprintf(os.Stderr, "\n===== captured bootstrap job logs =====\n%s\n===== end captured bootstrap job logs =====\n", data)
}
