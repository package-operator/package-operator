package bootstrap

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/package-operator/internal/controllers"
	"package-operator.run/package-operator/internal/packages/packagecontent"
)

const (
	packageOperatorClusterPackageName   = "package-operator"
	packageOperatorPackageCheckInterval = 2 * time.Second
)

// Initializes PKO on the cluster by installing CRDs and
// ensuring a package-operator ClusterPackage is present.
type initializer struct {
	client    client.Client
	loader    packageLoader
	loadFiles bootstrapperLoadFilesFn

	// config
	selfBootstrapImage string
	selfConfig         string
}

func newInitializer(
	client client.Client,
	loader packageLoader,
	loadFiles bootstrapperLoadFilesFn,

	// config
	selfBootstrapImage string,
	selfConfig string,
) *initializer {
	return &initializer{
		client:    client,
		loader:    loader,
		loadFiles: loadFiles,

		selfBootstrapImage: selfBootstrapImage,
		selfConfig:         selfConfig,
	}
}

func (init *initializer) Init(ctx context.Context) (
	*corev1alpha1.ClusterPackage, error,
) {
	crds, err := init.crdsFromPackage(ctx)
	if err != nil {
		return nil, err
	}
	if err := init.ensureCRDs(ctx, crds); err != nil {
		return nil, err
	}
	return init.ensureClusterPackage(ctx)
}

func (init *initializer) ensureClusterPackage(ctx context.Context) (
	*corev1alpha1.ClusterPackage, error,
) {
	pkoPackage := &corev1alpha1.ClusterPackage{
		ObjectMeta: metav1.ObjectMeta{
			Name: packageOperatorClusterPackageName,
		},
		Spec: corev1alpha1.PackageSpec{
			Image:  init.selfBootstrapImage,
			Config: init.config(),
		},
	}
	err := init.client.Create(ctx, pkoPackage)
	if errors.IsAlreadyExists(err) {
		return pkoPackage, init.updatePKOPackage(ctx, pkoPackage)
	}
	if err != nil {
		return nil, fmt.Errorf("creating Package Operator ClusterPackage: %w", err)
	}
	return pkoPackage, nil
}

func (init *initializer) updatePKOPackage(
	ctx context.Context, packageOperatorPackage *corev1alpha1.ClusterPackage,
) error {
	packageOperatorPackage.Spec.Image = init.selfBootstrapImage
	packageOperatorPackage.Spec.Config = init.config()
	return init.client.Patch(ctx, packageOperatorPackage, client.Merge)
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
	files, err := init.loadFiles(ctx, "/package")
	if err != nil {
		return nil, err
	}

	packgeContent, err := init.loader.FromFiles(ctx, files)
	if err != nil {
		return nil, err
	}

	// Install CRDs or the manager won't start.
	templateSpec := packagecontent.TemplateSpecFromPackage(packgeContent)
	return crdsFromTemplateSpec(templateSpec), nil
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

func crdsFromTemplateSpec(templateSpec corev1alpha1.ObjectSetTemplateSpec) []unstructured.Unstructured {
	var crds []unstructured.Unstructured
	for _, phase := range templateSpec.Phases {
		for _, obj := range phase.Objects {
			gk := obj.Object.GetObjectKind().GroupVersionKind().GroupKind()
			if gk != crdGK {
				continue
			}

			crds = append(crds, obj.Object)
		}
	}
	return crds
}
