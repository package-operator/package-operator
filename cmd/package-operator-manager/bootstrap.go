package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/package-operator/internal/controllers"
	"package-operator.run/package-operator/internal/packages/packagecontent"
	"package-operator.run/package-operator/internal/packages/packageimport"
	"package-operator.run/package-operator/internal/packages/packageloader"
)

const (
	pkoConfigEnvVar = "PKO_CONFIG"

	packageOperatorClusterPackageName   = "package-operator"
	packageOperatorPackageCheckInterval = 2 * time.Second
)

type packageLoader interface {
	FromFiles(ctx context.Context, files packagecontent.Files, opts ...packageloader.Option) (*packagecontent.Package, error)
}
type bootstrapperRunManagerFn func(ctx context.Context) error

type bootstrapperLoadFilesFn func(
	ctx context.Context, path string) (packagecontent.Files, error)

type bootstrapper struct {
	log                logr.Logger
	scheme             *runtime.Scheme
	loader             packageLoader
	selfBootstrapImage string
	pkoNamespace       string
	runManager         bootstrapperRunManagerFn
	loadFiles          bootstrapperLoadFilesFn

	client client.Client
}

func newBootstrapper(
	log logr.Logger, scheme *runtime.Scheme,
	selfBootstrapImage, pkoNamespace string,
	runManagerFn bootstrapperRunManagerFn,
) (*bootstrapper, error) {
	c, err := client.New(ctrl.GetConfigOrDie(), client.Options{
		Scheme: scheme,
	})
	if err != nil {
		return nil, fmt.Errorf("creating client: %w", err)
	}

	return &bootstrapper{
		log:                log,
		scheme:             scheme,
		loader:             packageloader.New(scheme, packageloader.WithDefaults),
		selfBootstrapImage: selfBootstrapImage,
		pkoNamespace:       pkoNamespace,
		runManager:         runManagerFn,
		loadFiles:          packageimport.Folder,

		client: c,
	}, nil
}

func (b *bootstrapper) Bootstrap(ctx context.Context) error {
	ctx = logr.NewContext(ctx, b.log)

	packageOperatorPackage := &corev1alpha1.ClusterPackage{}
	err := b.client.Get(ctx, client.ObjectKey{
		Name: packageOperatorClusterPackageName,
	}, packageOperatorPackage)
	if err == nil {
		// Package Operator is already installed.
		b.log.Info("Package Operator already installed, updating via in-cluster Package Operator")
		return b.updatePKOPackage(ctx, packageOperatorPackage)
	}

	// Retry error via Job.
	if err != nil && !errors.IsNotFound(err) && !meta.IsNoMatchError(err) {
		return fmt.Errorf("error looking up Package Operator ClusterPackage: %w", err)
	}

	b.log.Info("Package Operator NOT Available, self-bootstrapping")
	return b.selfBootstrap(ctx)
}

func (b *bootstrapper) updatePKOPackage(
	ctx context.Context, packageOperatorPackage *corev1alpha1.ClusterPackage,
) error {
	packageOperatorPackage.Spec.Image = b.selfBootstrapImage
	packageOperatorPackage.Spec.Config = getPKOConfigFromEnvironment()
	return b.client.Patch(ctx, packageOperatorPackage, client.Merge)
}

func getPKOConfigFromEnvironment() *runtime.RawExtension {
	pkoConfig := os.Getenv(pkoConfigEnvVar)
	var packageConfig *runtime.RawExtension
	if len(pkoConfig) > 0 {
		packageConfig = &runtime.RawExtension{Raw: []byte(pkoConfig)}
	}
	return packageConfig
}

func (b *bootstrapper) selfBootstrap(ctx context.Context) error {
	files, err := b.loadFiles(ctx, "/package")
	if err != nil {
		return err
	}

	packgeContent, err := b.loader.FromFiles(ctx, files)
	if err != nil {
		return err
	}

	// Install CRDs or the manager won't start.
	templateSpec := packagecontent.TemplateSpecFromPackage(packgeContent)
	crds := crdsFromTemplateSpec(templateSpec)
	if err := b.ensureCRDs(ctx, crds); err != nil {
		return err
	}

	if _, err = b.createPKOPackage(ctx); err != nil {
		return err
	}

	// Stop when Package Operator is installed.
	ctx, cancel := context.WithCancel(ctx)
	go b.cancelWhenPackageAvailable(ctx, cancel)

	// Force Adoption of objects during initial bootstrap to take ownership of
	// CRDs, Namespace, ServiceAccount and ClusterRoleBinding.
	if err := os.Setenv("PKO_FORCE_ADOPTION", "1"); err != nil {
		return err
	}

	return b.runManager(ctx)
}

func (b *bootstrapper) cancelWhenPackageAvailable(
	ctx context.Context, cancel context.CancelFunc,
) {
	log := logr.FromContextOrDiscard(ctx)
	err := wait.PollImmediateUntilWithContext(
		ctx, packageOperatorPackageCheckInterval,
		func(ctx context.Context) (done bool, err error) {
			return b.isPackageAvailable(ctx)
		})
	if err != nil {
		panic(err)
	}

	log.Info("Package Operator bootstrapped successfully!")
	cancel()
}

func (b *bootstrapper) isPackageAvailable(ctx context.Context) (
	available bool, err error,
) {
	packageOperatorPackage := &corev1alpha1.ClusterPackage{}
	err = b.client.Get(ctx, client.ObjectKey{
		Name: packageOperatorClusterPackageName,
	}, packageOperatorPackage)
	if err != nil {
		return false, err
	}

	if meta.IsStatusConditionTrue(
		packageOperatorPackage.Status.Conditions,
		corev1alpha1.PackageAvailable,
	) {
		return true, nil
	}
	return false, nil
}

// create PackageOperator ClusterPackage.
func (b *bootstrapper) createPKOPackage(ctx context.Context) (*corev1alpha1.ClusterPackage, error) {
	packageOperatorPackage := &corev1alpha1.ClusterPackage{
		ObjectMeta: metav1.ObjectMeta{
			Name: packageOperatorClusterPackageName,
		},
		Spec: corev1alpha1.PackageSpec{
			Image:  b.selfBootstrapImage,
			Config: getPKOConfigFromEnvironment(),
		},
	}

	if err := b.client.Create(ctx, packageOperatorPackage); err != nil && !errors.IsAlreadyExists(err) {
		return nil, fmt.Errorf("creating Package Operator ClusterPackage: %w", err)
	}
	return packageOperatorPackage, nil
}

// ensure all CRDs are installed on the cluster.
func (b *bootstrapper) ensureCRDs(ctx context.Context, crds []unstructured.Unstructured) error {
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
		if err := b.client.Create(ctx, crd); err != nil && !errors.IsAlreadyExists(err) {
			return err
		}
	}
	return nil
}

// GroupKind for CRDs.
var crdGK = schema.GroupKind{Group: "apiextensions.k8s.io", Kind: "CustomResourceDefinition"}

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
