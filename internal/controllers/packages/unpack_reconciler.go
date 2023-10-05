package packages

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

// Loads/unpack and templates packages into an ObjectDeployment.
func (c *GenericPackageController[P, D]) unpackReconcile(ctx context.Context, pkg *P) (res ctrl.Result, err error) {
	// run back off garbage collection to prevent stale data building up.
	defer c.backoff.GC()

	pkgObj := any(pkg).(client.Object)
	pkgConditions := ConditionsPtr(pkg)
	unpackHash := PackageUnpackHashPtr(*pkg)

	specHash := PackageSpecHash(*pkg, c.packageHashModifier)
	if *unpackHash == specHash {
		// We have already unpacked this package \o/
		return res, nil
	}

	pullStart := time.Now()
	log := logr.FromContextOrDiscard(ctx)
	files, err := c.imagePuller.Pull(ctx, *PackageImagePtr(*pkg))
	if err != nil {
		meta.SetStatusCondition(
			pkgConditions, metav1.Condition{
				Type:               corev1alpha1.PackageUnpacked,
				Status:             metav1.ConditionFalse,
				Reason:             "ImagePullBackOff",
				Message:            err.Error(),
				ObservedGeneration: pkgObj.GetGeneration(),
			})
		backoffID := string(pkgObj.GetUID())
		c.backoff.Next(backoffID, c.backoff.Clock.Now())
		backoff := c.backoff.Get(backoffID)
		log.Error(err, "pulling image", "backoff", backoff)

		return ctrl.Result{RequeueAfter: backoff}, nil
	}

	if err := c.packageDeployer.Load(ctx, GenericPackage(*pkg), files, *c.GetEnvironment()); err != nil {
		return res, fmt.Errorf("deploying package: %w", err)
	}

	if c.recorder != nil {
		c.recorder.RecordPackageLoadMetric(GenericPackage(*pkg), time.Since(pullStart))
	}
	*unpackHash = specHash
	meta.SetStatusCondition(pkgConditions, metav1.Condition{
		Type:               corev1alpha1.PackageUnpacked,
		Status:             metav1.ConditionTrue,
		Reason:             "UnpackSuccess",
		Message:            "Unpack job succeeded",
		ObservedGeneration: pkgObj.GetGeneration(),
	})

	return
}
