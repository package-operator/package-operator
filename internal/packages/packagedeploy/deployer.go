package packagedeploy

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/docker/distribution/reference"
	"github.com/opencontainers/go-digest"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
	"package-operator.run/package-operator/internal/adapters"
	"package-operator.run/package-operator/internal/controllers"
	"package-operator.run/package-operator/internal/packages/packageadmission"
	"package-operator.run/package-operator/internal/packages/packagecontent"
	"package-operator.run/package-operator/internal/packages/packageloader"
)

// PackageDeployer loads package contents from file, wraps it into an ObjectDeployment and deploys it.
type PackageDeployer struct {
	client client.Client
	scheme *runtime.Scheme

	newObjectDeployment adapters.ObjectDeploymentFactory

	deploymentReconciler deploymentReconciler
	packageContentLoader packageContentLoader
}

type (
	packageContentLoader interface {
		FromFiles(ctx context.Context, path packagecontent.Files, opts ...packageloader.Option) (*packagecontent.Package, error)
	}

	deploymentReconciler interface {
		Reconcile(ctx context.Context, desiredDeploy adapters.ObjectDeploymentAccessor, chunker objectChunker) error
	}
)

// Returns a new namespace-scoped loader for the Package API.
func NewPackageDeployer(c client.Client, scheme *runtime.Scheme) *PackageDeployer {
	return &PackageDeployer{
		client: c,
		scheme: scheme,

		newObjectDeployment: adapters.NewObjectDeployment,

		packageContentLoader: packageloader.New(
			scheme,
			packageloader.WithDefaults,
			packageloader.WithValidators(packageloader.PackageScopeValidator(manifestsv1alpha1.PackageManifestScopeNamespaced)),
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

		newObjectDeployment: adapters.NewClusterObjectDeployment,

		packageContentLoader: packageloader.New(
			scheme, packageloader.WithValidators(
				packageloader.PackageScopeValidator(manifestsv1alpha1.PackageManifestScopeCluster),
				&packageloader.ObjectPhaseAnnotationValidator{},
			),
		),

		deploymentReconciler: newDeploymentReconciler(scheme, c, adapters.NewClusterObjectDeployment, adapters.NewClusterObjectSlice,
			adapters.NewClusterObjectSliceList, newGenericClusterObjectSetList,
		),
	}
}

func ImageWithDigest(image string, imageDigest string) (string, error) {
	ref, err := reference.ParseDockerRef(image)
	if err != nil {
		return "", fmt.Errorf("image \"%s\" with digest \"%s\": %w", image, imageDigest, err)
	}

	canonical, err := reference.WithDigest(reference.TrimNamed(ref), digest.Digest(imageDigest))
	if err != nil {
		return "", fmt.Errorf("image \"%s\" with digest \"%s\": %w", image, imageDigest, err)
	}

	return canonical.String(), nil
}

func (l *PackageDeployer) Load(
	ctx context.Context, pkg adapters.GenericPackageAccessor,
	files packagecontent.Files, env manifestsv1alpha1.PackageEnvironment,
) error {
	packageContent, err := l.packageContentLoader.FromFiles(ctx, files)
	if err != nil {
		setInvalidConditionBasedOnLoadError(pkg, err)
		return nil
	}

	tmplCtx := pkg.TemplateContext()
	tmplCtx.Environment = env
	configuration := map[string]interface{}{}

	if tmplCtx.Config != nil {
		if err := json.Unmarshal(tmplCtx.Config.Raw, &configuration); err != nil {
			return fmt.Errorf("unmarshal config: %w", err)
		}
	}
	validationErrors, err := packageadmission.AdmitPackageConfiguration(
		ctx, l.scheme, configuration, packageContent.PackageManifest, field.NewPath("spec", "config"))
	if err != nil {
		return fmt.Errorf("validate Package configuration: %w", err)
	}
	if len(validationErrors) > 0 {
		setInvalidConditionBasedOnLoadError(pkg, validationErrors.ToAggregate())
		return nil
	}

	images := map[string]string{}
	if packageContent.PackageManifestLock != nil {
		for _, packageImage := range packageContent.PackageManifestLock.Spec.Images {
			resolvedImage, err := ImageWithDigest(packageImage.Image, packageImage.Digest)
			if err != nil {
				return err
			}
			images[packageImage.Name] = resolvedImage
		}
	}

	tt, err := packageloader.NewTemplateTransformer(packageloader.PackageFileTemplateContext{
		Package: tmplCtx.Package,
		Config:  configuration,
		Images:  images,
	})
	if err != nil {
		return err
	}
	packageContent, err = l.packageContentLoader.FromFiles(
		ctx, files,
		packageloader.WithFilesTransformers(tt),
		packageloader.WithTransformers(&packageloader.PackageTransformer{Package: pkg.ClientObject()}))
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

	// Load success
	meta.RemoveStatusCondition(pkg.GetConditions(), corev1alpha1.PackageInvalid)
	return nil
}

func (l *PackageDeployer) desiredObjectDeployment(
	_ context.Context, pkg adapters.GenericPackageAccessor, packageContent *packagecontent.Package,
) (deploy adapters.ObjectDeploymentAccessor, err error) {
	labels := map[string]string{
		manifestsv1alpha1.PackageLabel:         packageContent.PackageManifest.Name,
		manifestsv1alpha1.PackageInstanceLabel: pkg.ClientObject().GetName(),
	}

	configJSON, err := json.Marshal(pkg.TemplateContext().Config)
	if err != nil {
		return nil, fmt.Errorf("marshalling config for package-config annotation: %w", err)
	}
	annotations := map[string]string{
		manifestsv1alpha1.PackageSourceImageAnnotation: pkg.GetImage(),
		manifestsv1alpha1.PackageConfigAnnotation:      string(configJSON),
		controllers.ChangeCauseAnnotation: fmt.Sprintf(
			"Installing %s package.", packageContent.PackageManifest.Name),
	}

	deploy = l.newObjectDeployment(l.scheme)
	deploy.ClientObject().SetLabels(labels)
	deploy.ClientObject().SetAnnotations(annotations)

	deploy.ClientObject().SetName(pkg.ClientObject().GetName())
	deploy.ClientObject().SetNamespace(pkg.ClientObject().GetNamespace())

	deploy.SetTemplateSpec(packagecontent.TemplateSpecFromPackage(packageContent))
	deploy.SetSelector(labels)

	if err := controllerutil.SetControllerReference(
		pkg.ClientObject(), deploy.ClientObject(), l.scheme); err != nil {
		return nil, err
	}

	return deploy, nil
}

func setInvalidConditionBasedOnLoadError(pkg adapters.GenericPackageAccessor, err error) {
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
