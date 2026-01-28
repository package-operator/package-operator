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
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/internal/adapters"
	"package-operator.run/internal/apis/manifests"
	"package-operator.run/internal/controllers"
	"package-operator.run/internal/imageprefix"
	"package-operator.run/internal/metrics"
	"package-operator.run/internal/packages"
	"package-operator.run/internal/utils"
)

// Loads/unpack and templates packages into an ObjectDeployment.
type unpackReconciler struct {
	environmentSink

	uncachedClient client.Client

	imagePuller         imagePuller
	packageDeployer     packageDeployer
	packageLoadRecorder packageLoadRecorder

	backoff              *flowcontrol.Backoff
	packageHashModifier  *int32
	imagePrefixOverrides []imageprefix.Override
}

type environmentSink interface {
	GetEnvironment(ctx context.Context, namespace string) (*manifests.PackageEnvironment, error)
	SetEnvironment(env *manifests.PackageEnvironment)
}

type packageLoadRecorder interface {
	RecordPackageLoadMetric(
		pkg metrics.GenericPackage, d time.Duration)
}

func newUnpackReconciler(
	uncachedClient client.Client,
	imagePuller imagePuller,
	packageDeployer packageDeployer,
	packageLoadRecorder packageLoadRecorder,
	environmentSink environmentSink,
	packageHashModifier *int32,
	imagePrefixOverrides []imageprefix.Override,
) *unpackReconciler {
	var cfg unpackReconcilerConfig

	cfg.Default()

	return &unpackReconciler{
		environmentSink,

		uncachedClient,
		imagePuller,
		packageDeployer,
		packageLoadRecorder,
		cfg.GetBackoff(),
		packageHashModifier,
		imagePrefixOverrides,
	}
}

type imagePuller interface {
	Pull(ctx context.Context, image string) (*packages.RawPackage, error)
}

type packageDeployer interface {
	Deploy(
		ctx context.Context,
		apiPkg adapters.PackageAccessor,
		rawPkg *packages.RawPackage,
		env manifests.PackageEnvironment,
	) error
}

func (r *unpackReconciler) Reconcile(
	ctx context.Context, pkg adapters.PackageAccessor,
) (res ctrl.Result, err error) {
	// run back off garbage collection to prevent stale data building up.
	defer r.backoff.GC()

	specHash := pkg.GetSpecHash(r.packageHashModifier)
	if len(r.imagePrefixOverrides) > 0 {
		// upgrading from PKO without overrides won't cause repulls
		specHash += utils.ComputeSHA256Hash(r.imagePrefixOverrides, nil)
	}

	if pkg.GetStatusUnpackedHash() == specHash {
		if meta.IsStatusConditionFalse(*pkg.GetStatusConditions(), corev1alpha1.PackageUnpacked) {
			// Covers this case: unpack success -> unpack of new image failed -> rollback.
			meta.SetStatusCondition(
				pkg.GetStatusConditions(), metav1.Condition{
					Type:               corev1alpha1.PackageUnpacked,
					Status:             metav1.ConditionTrue,
					Reason:             "AlreadyUnpacked",
					Message:            "Package already unpacked",
					ObservedGeneration: pkg.ClientObject().GetGeneration(),
				})
		}

		// We have already unpacked this package \o/
		return res, nil
	}

	pullStart := time.Now()
	log := logr.FromContextOrDiscard(ctx)
	rawPkg, err := r.imagePuller.Pull(ctx, pkg.GetSpecImage())
	if err != nil {
		meta.SetStatusCondition(
			pkg.GetStatusConditions(), metav1.Condition{
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

	env, err := r.GetEnvironment(ctx, pkg.ClientObject().GetNamespace())
	if err != nil {
		return res, fmt.Errorf("getting environment: %w", err)
	}

	if err := r.packageDeployer.Deploy(ctx, pkg, rawPkg, *env); err != nil {
		return res, fmt.Errorf("deploying package: %w", err)
	}

	if r.packageLoadRecorder != nil {
		r.packageLoadRecorder.RecordPackageLoadMetric(
			pkg, time.Since(pullStart))
	}
	pkg.SetStatusUnpackedHash(specHash)
	meta.SetStatusCondition(
		pkg.GetStatusConditions(), metav1.Condition{
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
