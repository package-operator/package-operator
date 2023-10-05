package packages

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/flowcontrol"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"

	pkocore "package-operator.run/apis/core/v1alpha1"
	pkomanifests "package-operator.run/apis/manifests/v1alpha1"
	"package-operator.run/internal/adapters"
	"package-operator.run/internal/controllers"
	"package-operator.run/internal/environment"
	"package-operator.run/internal/metrics"
	"package-operator.run/internal/packages/packagecontent"
	"package-operator.run/internal/packages/packagedeploy"

	machinerymeta "k8s.io/apimachinery/pkg/api/meta"
)

const loaderJobFinalizer = "package-operator.run/loader-job"

type (
	imagePuller interface {
		Pull(ctx context.Context, image string) (packagecontent.Files, error)
	}

	packageDeployer interface {
		Load(ctx context.Context, pkg adapters.GenericPackageAccessor, files packagecontent.Files, env pkomanifests.PackageEnvironment) error
	}

	/*
		packageTypes interface {
			pkocore.Package | pkocore.ClusterPackage
		}

		objectDeploymentTypes interface {
			pkocore.ObjectDeployment | pkocore.ClusterObjectDeployment
		}
	*/

	metricsRecorder interface {
		RecordPackageMetrics(pkg metrics.GenericPackage)
		RecordPackageLoadMetric(pkg metrics.GenericPackage, d time.Duration)
	}
)

// Generic reconciler for both Package and ClusterPackage objects.
type GenericPackageController[P pkocore.ClusterPackage | pkocore.Package, D pkocore.ClusterObjectDeployment | pkocore.ObjectDeployment] struct {
	environment.Sink
	recorder            metricsRecorder
	client              client.Client
	log                 logr.Logger
	backoff             *flowcontrol.Backoff
	imagePuller         imagePuller
	packageHashModifier *int32
	packageDeployer     packageDeployer
}

func NewPackageController[P pkocore.ClusterPackage | pkocore.Package, D pkocore.ClusterObjectDeployment | pkocore.ObjectDeployment](
	client client.Client, log logr.Logger,
	scheme *runtime.Scheme,
	imagePuller imagePuller,
	metricsRecorder metricsRecorder,
	packageHashModifier *int32,
) *GenericPackageController[P, D] {
	var cfg controllers.BackoffConfig
	cfg.Default()

	controller := &GenericPackageController[P, D]{
		environment.Sink{},
		metricsRecorder,
		client,
		log,
		cfg.GetBackoff(),
		imagePuller,
		packageHashModifier,
		nil,
	}

	switch any(P{}).(type) {
	case pkocore.Package:
		controller.packageDeployer = packagedeploy.NewPackageDeployer(client, scheme)
	case pkocore.ClusterPackage:
		controller.packageDeployer = packagedeploy.NewClusterPackageDeployer(client, scheme)
	default:
		panic("invalid type")
	}

	return controller
}

func (c *GenericPackageController[P, D]) SetupWithManager(mgr ctrl.Manager) error {
	var d D
	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(controller.Options{MaxConcurrentReconciles: 5}).
		For(any(&P{}).(client.Object)).
		Owns(any(&d).(client.Object)).
		Complete(c)
}

func (c *GenericPackageController[P, D]) Reconcile(ctx context.Context, req ctrl.Request) (res ctrl.Result, err error) {
	log := c.log.WithValues("Package", req.String())
	defer log.Info("reconciled")
	ctx = logr.NewContext(ctx, log)

	pkg := &P{}
	obj := any(pkg).(client.Object)

	if err := c.client.Get(
		ctx, req.NamespacedName, obj); err != nil {
		return res, client.IgnoreNotFound(err)
	}
	defer func() {
		if err != nil {
			return
		}
		if c.recorder != nil {
			c.recorder.RecordPackageMetrics(GenericPackage(*pkg))
		}
	}()

	if !obj.GetDeletionTimestamp().IsZero() {
		// Remove finalizer from previous versions of PKO.
		if err := controllers.RemoveFinalizer(
			ctx, c.client, obj, loaderJobFinalizer); err != nil {
			return res, err
		}

		if err := c.client.Update(ctx, obj); err != nil {
			return res, err
		}
		return res, nil
	}

	res, err = c.unpackReconcile(ctx, pkg)
	if err != nil {
		return
	}

	if res.IsZero() {
		res, err = c.reconcileObjectDeploymentStatus(ctx, pkg)
		if err != nil {
			return
		}
	}

	c.setPackagePhase(pkg)
	if err := c.client.Status().Update(ctx, obj); err != nil {
		return res, fmt.Errorf("updating Package status: %w", err)
	}
	return res, nil
}

func (c *GenericPackageController[P, D]) setPackagePhase(pkg *P) {
	conditions := ConditionsPtr(pkg)
	phase := PackagePhasePtr(pkg)

	switch {
	case machinerymeta.IsStatusConditionTrue(*conditions, pkocore.PackageInvalid):
		*phase = pkocore.PackagePhaseInvalid
	case machinerymeta.FindStatusCondition(*conditions, pkocore.PackageUnpacked) == nil:
		*phase = pkocore.PackagePhaseUnpacking
	case machinerymeta.IsStatusConditionTrue(*conditions, pkocore.PackageProgressing):
		*phase = pkocore.PackagePhaseProgressing
	case machinerymeta.IsStatusConditionTrue(*conditions, pkocore.PackageAvailable):
		*phase = pkocore.PackagePhaseAvailable
	default:
		*phase = pkocore.PackagePhaseNotReady
	}
}
