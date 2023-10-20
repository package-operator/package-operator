package packages

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/flowcontrol"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
	"package-operator.run/internal/adapters"
	"package-operator.run/internal/controllers"
	"package-operator.run/internal/environment"
	"package-operator.run/internal/metrics"
	"package-operator.run/internal/packages/packagecontent"
	"package-operator.run/internal/packages/packageimport"
)

var (
	ErrInvalidPullSecretType            = errors.New("invalid pull secret type")
	ErrUnresolvablePullSecretNameInSpec = errors.New("unresolvable image pull secret name in spec")
	ErrSecretNotFound                   = errors.New("secrets not found")
)

// Loads/unpack and templates packages into an ObjectDeployment.
type unpackReconciler struct {
	environment.Sink

	imagePuller         imagePuller
	packageDeployer     packageDeployer
	packageLoadRecorder packageLoadRecorder
	reader              client.Reader

	backoff             *flowcontrol.Backoff
	packageHashModifier *int32
}

type packageLoadRecorder interface {
	RecordPackageLoadMetric(
		pkg metrics.GenericPackage, d time.Duration)
}

func newUnpackReconciler(
	reader client.Reader,
	imagePuller imagePuller,
	packageDeployer packageDeployer,
	packageLoadRecorder packageLoadRecorder,
	packageHashModifier *int32,
	opts ...unpackReconcilerOption,
) *unpackReconciler {
	var cfg unpackReconcilerConfig

	cfg.Option(opts...)
	cfg.Default()

	return &unpackReconciler{
		reader:              reader,
		imagePuller:         imagePuller,
		packageDeployer:     packageDeployer,
		packageLoadRecorder: packageLoadRecorder,
		backoff:             cfg.GetBackoff(),
		packageHashModifier: packageHashModifier,
	}
}

type imagePuller interface {
	Pull(ctx context.Context, image string, opts ...packageimport.PullOption) (
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

	specHash := pkg.GetSpecHash(r.packageHashModifier)
	if pkg.GetUnpackedHash() == specHash {
		// We have already unpacked this package \o/
		return res, nil
	}

	log := logr.FromContextOrDiscard(ctx)

	pullOpts, err := r.buildPullOptions(ctx, pkg)
	if err != nil {
		meta.SetStatusCondition(pkg.GetConditions(), metav1.Condition{
			Type:               corev1alpha1.PackageUnpacked,
			Status:             metav1.ConditionFalse,
			Reason:             "PullSecretValidationFailed",
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

	pullStart := time.Now()
	files, err := r.imagePuller.Pull(ctx, pkg.GetImage(), pullOpts...)
	if err != nil {
		meta.SetStatusCondition(pkg.GetConditions(), metav1.Condition{
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

	if err := r.packageDeployer.Load(ctx, pkg, files, *r.GetEnvironment()); err != nil {
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

func (r *unpackReconciler) buildPullOptions(ctx context.Context, pkg adapters.GenericPackageAccessor) ([]packageimport.PullOption, error) {
	pullSecretRef := pkg.GetImagePullSecret()
	if pullSecretRef == nil {
		// No pull secret configured.
		return []packageimport.PullOption{}, nil
	}

	// If this is a scoped Package object then the SecretReference is not allowed to point to another namespace.
	targetNamespace := pkg.ClientObject().GetNamespace()
	if len(targetNamespace) == 0 {
		targetNamespace = pullSecretRef.Namespace
	}

	// Get Secret from API.
	pullSecret := corev1.Secret{}
	if err := r.reader.Get(ctx, types.NamespacedName{
		Name:      pullSecretRef.Name,
		Namespace: targetNamespace,
	}, &pullSecret); err != nil {
		return nil, err
	}

	// Validate secret type.
	if pullSecret.Type != corev1.SecretTypeDockerConfigJson {
		return nil, fmt.Errorf("%w: %s", ErrInvalidPullSecretType, pullSecret.Type)
	}

	// Wrap pull secret in PullOption.
	return []packageimport.PullOption{
		packageimport.WithPullSecret{
			Data: pullSecret.Data[corev1.DockerConfigJsonKey],
		},
	}, nil
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
