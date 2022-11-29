package packagedeploy

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
	"package-operator.run/package-operator/internal/adapters"
	"package-operator.run/package-operator/internal/packages/packagebytes"
	"package-operator.run/package-operator/internal/packages/packagestructure"
)

type packageContentLoader interface {
	Load(ctx context.Context, path string, opts ...packagestructure.LoaderOption) (
		*packagestructure.PackageContent, error)
}

type deploymentReconciler interface {
	Reconcile(
		ctx context.Context, desiredDeploy adapters.ObjectDeploymentAccessor,
		chunker objectChunker,
	) error
}

// PackageDeployer loads package contents from file, wraps it into an ObjectDeployment and deploys it.
type PackageDeployer struct {
	client client.Client
	scheme *runtime.Scheme

	newPackage          genericPackageFactory
	newObjectDeployment adapters.ObjectDeploymentFactory

	deploymentReconciler deploymentReconciler
	packageContentLoader packageContentLoader
}

// Returns a new namespace-scoped loader for the Package API.
func NewPackageDeployer(c client.Client, scheme *runtime.Scheme) *PackageDeployer {
	return &PackageDeployer{
		client: c,
		scheme: scheme,

		newPackage:          newGenericPackage,
		newObjectDeployment: adapters.NewObjectDeployment,

		packageContentLoader: packagestructure.NewLoader(
			scheme, packagestructure.WithManifestValidators(
				packagestructure.PackageScopeValidator(manifestsv1alpha1.PackageManifestScopeNamespaced),
				&packagestructure.ObjectPhaseAnnotationValidator{},
			),
		),

		deploymentReconciler: newDeploymentReconciler(
			scheme, c,
			adapters.NewObjectDeployment, adapters.NewObjectSlice,
			adapters.NewObjectSliceList, newGenericObjectSetList,
		),
	}
}

// Returns a new cluster-scoped loader for the ClusterPackage API.
func NewClusterPackageDeployer(c client.Client, scheme *runtime.Scheme) *PackageDeployer {
	return &PackageDeployer{
		client: c,
		scheme: scheme,

		newPackage:          newGenericClusterPackage,
		newObjectDeployment: adapters.NewClusterObjectDeployment,

		packageContentLoader: packagestructure.NewLoader(
			scheme, packagestructure.WithManifestValidators(
				packagestructure.PackageScopeValidator(manifestsv1alpha1.PackageManifestScopeCluster),
				&packagestructure.ObjectPhaseAnnotationValidator{},
			),
		),

		deploymentReconciler: newDeploymentReconciler(scheme, c, adapters.NewClusterObjectDeployment, adapters.NewClusterObjectSlice,
			adapters.NewClusterObjectSliceList, newGenericClusterObjectSetList,
		),
	}
}

func (l *PackageDeployer) Load(ctx context.Context, packageKey client.ObjectKey, folderPath string) error {
	log := logr.FromContextOrDiscard(ctx)

	pkg := l.newPackage(l.scheme)
	if err := l.client.Get(ctx, packageKey, pkg.ClientObject()); err != nil {
		return err
	}

	if err := l.load(ctx, pkg, folderPath); err != nil {
		return err
	}

	invalidCondition := meta.FindStatusCondition(*pkg.GetConditions(), corev1alpha1.PackageInvalid)
	if invalidCondition == nil {
		return nil
	}
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		log.Info("trying to report Package status...")

		meta.SetStatusCondition(pkg.GetConditions(), *invalidCondition) // reapply condition after update
		err := l.client.Status().Update(ctx, pkg.ClientObject())
		if err == nil {
			return nil
		}

		if apierrors.IsConflict(err) {
			// Get latest version of the ObjectDeployment to resolve conflict.
			if err := l.client.Get(
				ctx,
				client.ObjectKeyFromObject(pkg.ClientObject()),
				pkg.ClientObject(),
			); err != nil {
				return fmt.Errorf("getting ObjectDeployment to resolve conflict: %w", err)
			}
		}

		return err
	})
}

func (l *PackageDeployer) load(ctx context.Context, pkg genericPackage, folderPath string) error {
	packageContent, err := l.packageContentLoader.Load(
		ctx, folderPath,
		packagestructure.WithByteTransformers(
			&packagebytes.TemplateTransformer{
				TemplateContext: pkg.TemplateContext(),
			},
		),
		packagestructure.WithManifestTransformers(
			&packagestructure.CommonObjectLabelsTransformer{
				Package: pkg.ClientObject(),
			},
		))
	if err != nil {
		setInvalidConditionBasedOnLoadError(pkg, err)
		return nil
	}

	desiredDeploy, err := l.desiredObjectDeployment(ctx, pkg, packageContent)
	if err != nil {
		return fmt.Errorf("creating desired ObjectDeployment: %w", err)
	}

	chunker := determineChunkingStrategyForPackage(pkg)
	if err := l.deploymentReconciler.Reconcile(ctx, desiredDeploy, chunker); err != nil {
		return fmt.Errorf("reconciling ObjectDeployment: %w", err)
	}

	meta.SetStatusCondition(pkg.GetConditions(), metav1.Condition{
		Type:               corev1alpha1.PackageInvalid,
		Status:             metav1.ConditionFalse,
		Reason:             "LoadSuccess",
		ObservedGeneration: pkg.ClientObject().GetGeneration(),
	})
	return nil
}

func (l *PackageDeployer) desiredObjectDeployment(
	ctx context.Context, pkg genericPackage, packageContent *packagestructure.PackageContent,
) (deploy adapters.ObjectDeploymentAccessor, err error) {
	labels := map[string]string{
		manifestsv1alpha1.PackageLabel:         packageContent.PackageManifest.Name,
		manifestsv1alpha1.PackageInstanceLabel: pkg.ClientObject().GetName(),
	}

	deploy = l.newObjectDeployment(l.scheme)
	deploy.ClientObject().SetLabels(labels)
	deploy.ClientObject().SetName(pkg.ClientObject().GetName())
	deploy.ClientObject().SetNamespace(pkg.ClientObject().GetNamespace())

	deploy.SetTemplateSpec(packageContent.ToTemplateSpec())
	deploy.SetSelector(labels)

	if err := controllerutil.SetControllerReference(
		pkg.ClientObject(), deploy.ClientObject(), l.scheme); err != nil {
		return nil, err
	}

	return deploy, nil
}

func setInvalidConditionBasedOnLoadError(pkg genericPackage, err error) {
	reason := "LoadError"

	// Can not be determined more precisely
	meta.SetStatusCondition(pkg.GetConditions(), metav1.Condition{
		Type:               corev1alpha1.PackageInvalid,
		Status:             metav1.ConditionTrue,
		Reason:             reason,
		Message:            err.Error(),
		ObservedGeneration: pkg.ClientObject().GetGeneration(),
	})
}
