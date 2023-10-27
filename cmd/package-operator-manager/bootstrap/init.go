package bootstrap

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"github.com/mt-sre/devkube/dev"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
	"package-operator.run/internal/controllers"
	"package-operator.run/internal/controllers/objectdeployments"
)

const (
	packageOperatorClusterPackageName              = "package-operator"
	packageOperatorPackageCheckInterval            = 2 * time.Second
	packageOperatorDeploymentDeletionCheckInterval = 2 * time.Second
	packageOperatorDeploymentDeletionTimeout       = 2 * time.Minute
)

// Initializes PKO on the cluster by installing CRDs and
// ensuring a package-operator ClusterPackage is present.
// Will shut down PKO prior to bootstrapping if the ClusterPackage was updated to ensure that new "non-buggy" PKO code will handle the migration.
type initializer struct {
	client    client.Client
	scheme    *runtime.Scheme
	loader    packageObjectLoader
	pullImage bootstrapperPullImageFn

	// config
	packageOperatorNamespace string
	selfBootstrapImage       string
	selfConfig               string
}

func newInitializer(
	client client.Client,
	scheme *runtime.Scheme,
	loader packageObjectLoader,
	pullImage bootstrapperPullImageFn,

	// config
	packageOperatorNamespace string,
	selfBootstrapImage string,
	selfConfig string,
) *initializer {
	return &initializer{
		client:    client,
		scheme:    scheme,
		loader:    loader,
		pullImage: pullImage,

		packageOperatorNamespace: packageOperatorNamespace,
		selfBootstrapImage:       selfBootstrapImage,
		selfConfig:               selfConfig,
	}
}

func (init *initializer) Init(ctx context.Context) (
	needsBootstrap bool, err error,
) {
	crds, err := init.crdsFromPackage(ctx)
	if err != nil {
		return false, err
	}
	if err := init.ensureCRDs(ctx, crds); err != nil {
		return false, err
	}

	return init.ensureUpdatedPKO(ctx)
}

func (init *initializer) newPKOClusterPackage() *corev1alpha1.ClusterPackage {
	return &corev1alpha1.ClusterPackage{
		ObjectMeta: metav1.ObjectMeta{
			Name: packageOperatorClusterPackageName,
		},
		Spec: corev1alpha1.PackageSpec{
			Image:  init.selfBootstrapImage,
			Config: init.config(),
		},
	}
}

// ensureUpdatedPKO compares new and old PKO ClusterPackages, looks at PKO availability, it handles eventual PKO shutdown, update of the PKO ClusterPackage and decides if bootstrap should be executed or not.
func (init *initializer) ensureUpdatedPKO(ctx context.Context) (bool, error) {
	bootstrapClusterPackage := init.newPKOClusterPackage()

	existingClusterPackage := &corev1alpha1.ClusterPackage{}
	if err := init.client.Get(ctx, client.ObjectKey{
		Name: packageOperatorClusterPackageName,
	}, existingClusterPackage); errors.IsNotFound(err) {
		// ClusterPackage not found. Create it and let bootstrapper run.
		return true, init.client.Create(ctx, bootstrapClusterPackage)
	} else if err != nil {
		return false, err
	}

	log := logr.FromContextOrDiscard(ctx)
	if bootstrapClusterPackage.Spec.Image != existingClusterPackage.Spec.Image {
		log.Info("image has been updated",
			"from", existingClusterPackage.Spec.Image,
			"to", bootstrapClusterPackage.Spec.Image)

		// ClusterPackage specs differ. Shut down PKO, update ClusterPackage and run bootstrapper.
		if err := init.ensurePKODeploymentGone(ctx); err != nil {
			return false, err
		}
		if err := init.ensurePKORevisionsPaused(ctx, bootstrapClusterPackage); err != nil {
			return false, err
		}
	}

	if !equality.Semantic.DeepEqual(
		bootstrapClusterPackage.Spec, existingClusterPackage.Spec) {
		log.Info("patching PackageOperator ClusterPackage")
		if err := init.client.Patch(ctx, bootstrapClusterPackage, client.Merge); err != nil {
			return false, err
		}
	}

	isAvailable, err := isPKOAvailable(ctx, init.client, init.packageOperatorNamespace)
	if err != nil {
		return false, err
	}
	if isAvailable {
		// PKO is available. Skip bootstrap.
		log.Info("PackageOperator is available")
		return false, nil
	}
	log.Info("PackageOperator is NOT available")
	return true, nil
}

// Pauses all existing PKO ObjectSets to ensure we don't install an old version again,
// before moving on to the actual latest version.
func (init *initializer) ensurePKORevisionsPaused(ctx context.Context, pkg *corev1alpha1.ClusterPackage) error {
	cosList := &corev1alpha1.ClusterObjectSetList{}
	err := init.client.List(
		ctx, cosList,
		client.MatchingLabels{
			objectdeployments.ObjectSetObjectDeploymentLabel: "package-operator",
		},
	)
	if err != nil {
		return fmt.Errorf("listing PKO ClusterObjectSets: %w", err)
	}

	for i := range cosList.Items {
		cos := &cosList.Items[i]

		var updateNeeded bool
		cosPackageSource := cos.Annotations[manifestsv1alpha1.PackageSourceImageAnnotation]
		if cosPackageSource == pkg.Spec.Image &&
			cos.Spec.LifecycleState == corev1alpha1.ObjectSetLifecycleStatePaused {
			// This revision is supposed to be present, but has been paused for some reason.
			cos.Spec.LifecycleState = corev1alpha1.ObjectSetLifecycleStateActive
			updateNeeded = true
		} else if cos.Spec.LifecycleState == corev1alpha1.ObjectSetLifecycleStateActive {
			cos.Spec.LifecycleState = corev1alpha1.ObjectSetLifecycleStatePaused
			updateNeeded = true
		}

		if !updateNeeded {
			continue
		}
		if err := init.client.Update(ctx, cos); err != nil {
			return fmt.Errorf("pausing PKO ClusterObjectSet: %w", err)
		}
	}

	return nil
}

func (init *initializer) ensurePKODeploymentGone(ctx context.Context) error {
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: init.packageOperatorNamespace,
			Name:      packageOperatorDeploymentName,
		},
	}

	// use foreground deletion to ensure that all pods are gone when the deployment object vanishes from the apiserver
	err := init.client.Delete(ctx, deployment,
		client.PropagationPolicy(metav1.DeletePropagationForeground),
		client.GracePeriodSeconds(0))
	if errors.IsNotFound(err) {
		// object is already gone
		return nil
	} else if err != nil {
		return err
	}

	// wait for object to be fully deleted
	waiter := dev.NewWaiter(init.client, init.scheme,
		dev.WithInterval(packageOperatorDeploymentDeletionCheckInterval),
		dev.WithTimeout(packageOperatorDeploymentDeletionTimeout))
	return waiter.WaitToBeGone(ctx, deployment, func(obj client.Object) (done bool, err error) { return false, nil })
}

func (init *initializer) config() *runtime.RawExtension {
	var packageConfig *runtime.RawExtension
	if len(init.selfConfig) > 0 {
		packageConfig = &runtime.RawExtension{
			Raw: []byte(init.selfConfig),
		}
	}
	return packageConfig
}

func (init *initializer) crdsFromPackage(ctx context.Context) (
	crds []unstructured.Unstructured, err error,
) {
	rawPkg, err := init.pullImage(ctx, init.selfBootstrapImage)
	if err != nil {
		return nil, err
	}

	objs, err := init.loader.FromPkg(ctx, rawPkg)
	if err != nil {
		return nil, err
	}

	// Install CRDs or the manager won't start.
	return crdsFromObjects(objs), nil
}

// ensure all CRDs are installed on the cluster.
func (init *initializer) ensureCRDs(ctx context.Context, crds []unstructured.Unstructured) error {
	log := logr.FromContextOrDiscard(ctx)
	for i := range crds {
		crd := &crds[i]

		// Set cache label.
		labels := crd.GetLabels()
		if labels == nil {
			labels = map[string]string{}
		}
		labels[controllers.DynamicCacheLabel] = "True"
		crd.SetLabels(labels)

		log.Info("ensuring CRD", "name", crd.GetName())
		if err := init.client.Create(ctx, crd); err != nil &&
			!errors.IsAlreadyExists(err) {
			return err
		}
	}
	return nil
}

// GroupKind for CRDs.
var crdGK = schema.GroupKind{
	Group: "apiextensions.k8s.io",
	Kind:  "CustomResourceDefinition",
}

func crdsFromObjects(objs []unstructured.Unstructured) (crds []unstructured.Unstructured) {
	for _, obj := range objs {
		gk := obj.GetObjectKind().GroupVersionKind().GroupKind()
		if gk != crdGK {
			continue
		}

		crds = append(crds, obj)
	}
	return crds
}
