package packages

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/flowcontrol"
	ctrl "sigs.k8s.io/controller-runtime"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
	"package-operator.run/package-operator/internal/adapters"
	"package-operator.run/package-operator/internal/controllers"
	"package-operator.run/package-operator/internal/environment"
	"package-operator.run/package-operator/internal/metrics"
	"package-operator.run/package-operator/internal/packages/packagecontent"
)

// Loads/unpack and templates packages into an ObjectDeployment.
type unpackReconciler struct {
	environment.Sink

	imagePuller         imagePuller
	packageDeployer     packageDeployer
	packageLoadRecorder packageLoadRecorder

	backoff *flowcontrol.Backoff
}

type packageLoadRecorder interface {
	RecordPackageLoadMetric(
		pkg metrics.GenericPackage, d time.Duration)
}

func newUnpackReconciler(
	imagePuller imagePuller,
	packageDeployer packageDeployer,
	packageLoadRecorder packageLoadRecorder,
	opts ...unpackReconcilerOption,
) *unpackReconciler {
	var cfg unpackReconcilerConfig

	cfg.Option(opts...)
	cfg.Default()

	return &unpackReconciler{
		imagePuller:         imagePuller,
		packageDeployer:     packageDeployer,
		packageLoadRecorder: packageLoadRecorder,
		backoff:             cfg.GetBackoff(),
	}
}

type imagePuller interface {
	Pull(ctx context.Context, image string) (
		packagecontent.Files, error)
}

type packageDeployer interface {
	Load(
		ctx context.Context, pkg adapters.GenericPackageAccessor,
		files packagecontent.Files, env manifestsv1alpha1.PackageEnvironment,
	) error
}

func (r *unpackReconciler) Reconcile(
	ctx context.Context, pkg adapters.GenericPackageAccessor,
) (res ctrl.Result, err error) {
	// run back off garbage collection to prevent stale data building up.
	defer r.backoff.GC()

	specHash := pkg.GetSpecHash()
	if pkg.GetUnpackedHash() == specHash {
		// We have already unpacked this package \o/
		return res, nil
	}

	pullStart := time.Now()
	log := logr.FromContextOrDiscard(ctx)
	files, err := r.imagePuller.Pull(ctx, pkg.GetImage())
	if err != nil {
		meta.SetStatusCondition(
			pkg.GetConditions(), metav1.Condition{
				Type:               corev1alpha1.PackageUnpacked,
				Status:             metav1.ConditionFalse,
				Reason:             "ImagePullBackOff",
				Message:            err.Error(),
				ObservedGeneration: pkg.ClientObject().GetGeneration(),
			})
		backoffID := string(pkg.ClientObject().GetUID())
		r.backoff.Next(backoffID, r.backoff.Clock.Now())
		backoff := r.backoff.Get(backoffID)
		log.Error(err, "pulling image", "backoff", backoff)

		return ctrl.Result{
			RequeueAfter: backoff,
		}, nil
	}

	if err := r.packageDeployer.Load(ctx, pkg, files, r.GetEnvironment()); err != nil {
		return res, fmt.Errorf("deploying package: %w", err)
	}

	if r.packageLoadRecorder != nil {
		r.packageLoadRecorder.RecordPackageLoadMetric(
			pkg, time.Since(pullStart))
	}
	pkg.SetUnpackedHash(specHash)
	meta.SetStatusCondition(
		pkg.GetConditions(), metav1.Condition{
			Type:               corev1alpha1.PackageUnpacked,
			Status:             metav1.ConditionTrue,
			Reason:             "UnpackSuccess",
			Message:            "Unpack job succeeded",
			ObservedGeneration: pkg.ClientObject().GetGeneration(),
		})

	return
}

type unpackReconcilerConfig struct {
	controllers.BackoffConfig
}

func (c *unpackReconcilerConfig) Option(opts ...unpackReconcilerOption) {
	for _, opt := range opts {
		opt.ConfigureUnpackReconciler(c)
	}
}

func (c *unpackReconcilerConfig) Default() {
	c.BackoffConfig.Default()
}

type unpackReconcilerOption interface {
	ConfigureUnpackReconciler(c *unpackReconcilerConfig)
}
