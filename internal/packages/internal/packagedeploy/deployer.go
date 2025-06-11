package packagedeploy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/google/go-containerregistry/pkg/name"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"pkg.package-operator.run/semver"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
	"package-operator.run/internal/adapters"
	"package-operator.run/internal/apis/manifests"
	"package-operator.run/internal/constants"
	"package-operator.run/internal/imageprefix"
	"package-operator.run/internal/packages/internal/packagemanifestvalidation"
	"package-operator.run/internal/packages/internal/packagerender"
	"package-operator.run/internal/packages/internal/packagestructure"
	"package-operator.run/internal/packages/internal/packagetypes"
	"package-operator.run/internal/packages/internal/packagevalidation"
)

var ErrNonExisting = errors.New("unable to validate non existing package")

// PackageDeployer loads package contents from file, wraps it into an ObjectDeployment and deploys it.
type PackageDeployer struct {
	client         client.Client
	uncachedClient client.Client

	scheme *runtime.Scheme

	newObjectDeployment adapters.ObjectDeploymentFactory
	structuralLoader    structuralLoader

	deploymentReconciler deploymentReconciler
	packageValidators    packagevalidation.PackageValidatorList

	imagePrefixOverrides []imageprefix.Override
}

type (
	deploymentReconciler interface {
		Reconcile(ctx context.Context, desiredDeploy adapters.ObjectDeploymentAccessor, chunker objectChunker) error
	}
	structuralLoader interface {
		LoadComponent(
			ctx context.Context, rawPkg *packagetypes.RawPackage, componentName string,
		) (*packagetypes.Package, error)
	}
)

// Returns a new namespace-scoped loader for the Package API.
func NewPackageDeployer(
	c client.Client, uncachedClient client.Client,
	scheme *runtime.Scheme,
	imagePrefixOverrides []imageprefix.Override,
) *PackageDeployer {
	return &PackageDeployer{
		client:         c,
		uncachedClient: uncachedClient,

		scheme: scheme,

		newObjectDeployment: adapters.NewObjectDeployment,
		structuralLoader:    packagestructure.DefaultStructuralLoader,

		deploymentReconciler: newDeploymentReconciler(
			scheme, c,
			adapters.NewObjectDeployment, adapters.NewObjectSlice,
			adapters.NewObjectSliceList, newGenericObjectSetList,
		),
		packageValidators: append(
			packagevalidation.DefaultPackageValidators,
			packagevalidation.PackageScopeValidator(manifests.PackageManifestScopeNamespaced),
		),
		imagePrefixOverrides: imagePrefixOverrides,
	}
}

// Returns a new cluster-scoped loader for the ClusterPackage API.
func NewClusterPackageDeployer(
	c client.Client,
	scheme *runtime.Scheme,
	imagePrefixOverrides []imageprefix.Override,
) *PackageDeployer {
	return &PackageDeployer{
		client: c,
		scheme: scheme,

		newObjectDeployment: adapters.NewClusterObjectDeployment,
		structuralLoader:    packagestructure.DefaultStructuralLoader,

		deploymentReconciler: newDeploymentReconciler(
			scheme,
			c,
			adapters.NewClusterObjectDeployment,
			adapters.NewClusterObjectSlice,
			adapters.NewClusterObjectSliceList,
			newGenericClusterObjectSetList,
		),
		packageValidators: append(
			packagevalidation.DefaultPackageValidators,
			packagevalidation.PackageScopeValidator(manifests.PackageManifestScopeCluster),
		),
		imagePrefixOverrides: imagePrefixOverrides,
	}
}

// ImageWithDigest replaces the tag/digest part of the given reference
// with the digest specified by digest. It does not sanitize the
// reference and expands well known registries.
func ImageWithDigest(reference string, digest string) (string, error) {
	// Parse reference into something we can use.
	ref, err := name.ParseReference(reference)
	if err != nil {
		return "", fmt.Errorf("parse image reference: %w", err)
	}

	// Create a new digest reference from the context of the parsed reference
	// with the parameter digest and return the string.
	return ref.Context().Digest(digest).String(), nil
}

func (l *PackageDeployer) Deploy(
	ctx context.Context,
	apiPkg adapters.PackageAccessor,
	rawPkg *packagetypes.RawPackage,
	env manifests.PackageEnvironment,
) error {
	pkg, err := l.structuralLoader.LoadComponent(ctx, rawPkg, apiPkg.GetSpecComponent())
	if err != nil {
		setInvalidConditionBasedOnLoadError(apiPkg, err)
		// Explicitly do not return an error here to avoid re-pulling
		// and deploying a broken package over and over again.
		return nil
	}

	// Check constraints
	if err := validateConstraints(ctx, l.uncachedClient, apiPkg, pkg.Manifest, env); err != nil {
		setInvalidConditionBasedOnLoadError(apiPkg, err)
		return err
	}

	// prepare package render/template context
	tmplCtx := apiPkg.GetSpecTemplateContext()
	configuration := map[string]any{}
	if tmplCtx.Config != nil {
		if err := json.Unmarshal(tmplCtx.Config.Raw, &configuration); err != nil {
			return fmt.Errorf("unmarshal config: %w", err)
		}
	}
	validationErrors, err := packagemanifestvalidation.AdmitPackageConfiguration(
		ctx, configuration, pkg.Manifest, field.NewPath("spec", "config"))
	if err != nil {
		return fmt.Errorf("validate Package configuration: %w", err)
	}
	if len(validationErrors) > 0 {
		aggregateErr := validationErrors.ToAggregate()
		setInvalidConditionBasedOnLoadError(apiPkg, aggregateErr)
		return aggregateErr
	}
	images := map[string]string{}
	if pkg.ManifestLock != nil {
		for _, packageImage := range pkg.ManifestLock.Spec.Images {
			replacedImage := imageprefix.Replace(packageImage.Image, l.imagePrefixOverrides)

			resolvedImage, err := ImageWithDigest(replacedImage, packageImage.Digest)
			if err != nil {
				return err
			}
			images[packageImage.Name] = resolvedImage
		}
	}

	// render package instance
	pkgInstance, err := packagerender.RenderPackageInstance(
		ctx, pkg,
		packagetypes.PackageRenderContext{
			Package:     tmplCtx.Package,
			Config:      configuration,
			Images:      images,
			Environment: env,
		}, l.packageValidators, packagevalidation.DefaultObjectValidators)
	if err != nil {
		setInvalidConditionBasedOnLoadError(apiPkg, err)
		return err
	}

	desiredDeploy, err := l.desiredObjectDeployment(ctx, apiPkg, pkgInstance)
	if err != nil {
		return fmt.Errorf("creating desired ObjectDeployment: %w", err)
	}

	chunker := determineChunkingStrategyForPackage(apiPkg)
	if err := l.deploymentReconciler.Reconcile(ctx, desiredDeploy, chunker); err != nil {
		return fmt.Errorf("reconciling ObjectDeployment: %w", err)
	}

	// Load success
	meta.RemoveStatusCondition(apiPkg.GetSpecConditions(), corev1alpha1.PackageInvalid)
	return nil
}

func (l *PackageDeployer) desiredObjectDeployment(
	_ context.Context, pkg adapters.PackageAccessor, pkgInstance *packagetypes.PackageInstance,
) (deploy adapters.ObjectDeploymentAccessor, err error) {
	labels := map[string]string{
		manifestsv1alpha1.PackageLabel:         pkgInstance.Manifest.Name,
		manifestsv1alpha1.PackageInstanceLabel: pkg.ClientObject().GetName(),
	}

	configJSON, err := json.Marshal(pkg.GetSpecTemplateContext().Config)
	if err != nil {
		return nil, fmt.Errorf("marshalling config for package-config annotation: %w", err)
	}

	annotations := map[string]string{
		manifestsv1alpha1.PackageSourceImageAnnotation: imageprefix.Replace(pkg.GetSpecImage(), l.imagePrefixOverrides),
		manifestsv1alpha1.PackageConfigAnnotation:      string(configJSON),
		constants.ChangeCauseAnnotation: fmt.Sprintf(
			"Installing %s package.", pkgInstance.Manifest.Name),
	}

	deploy = l.newObjectDeployment(l.scheme)
	deploy.ClientObject().SetLabels(labels)
	deploy.ClientObject().SetAnnotations(annotations)

	deploy.ClientObject().SetName(pkg.ClientObject().GetName())
	deploy.ClientObject().SetNamespace(pkg.ClientObject().GetNamespace())

	deploy.SetSpecTemplateSpec(packagerender.RenderObjectSetTemplateSpec(pkgInstance))
	deploy.SetSpecSelector(labels)

	if err := controllerutil.SetControllerReference(
		pkg.ClientObject(), deploy.ClientObject(), l.scheme); err != nil {
		return nil, err
	}

	return deploy, nil
}

func setInvalidConditionBasedOnLoadError(pkg adapters.PackageAccessor, err error) {
	reason := "LoadError"

	// Can not be determined more precisely
	meta.SetStatusCondition(pkg.GetSpecConditions(), metav1.Condition{
		Type:               corev1alpha1.PackageInvalid,
		Status:             metav1.ConditionTrue,
		Reason:             reason,
		Message:            err.Error(),
		ObservedGeneration: pkg.ClientObject().GetGeneration(),
	})
}

var uniqueLock = sync.Mutex{}

func validateUnique(
	ctx context.Context, uncachedClient client.Client,
	apiPkg adapters.PackageAccessor, manifest *manifests.PackageManifest,
) ([]string, error) {
	hasUnique := false
	for _, c := range manifest.Spec.Constraints {
		if c.UniqueInScope != nil {
			hasUnique = true
			break
		}
	}
	if !hasUnique {
		return nil, nil
	}

	uniqueLock.Lock()
	defer uniqueLock.Unlock()

	s := labels.NewSelector()
	req, err := labels.NewRequirement(manifests.PackageLabel, selection.Equals, []string{manifest.Name})
	if err != nil {
		return nil, fmt.Errorf("create package label selector for validate unique constraint: %w", err)
	}
	s.Add(*req)

	var l int

	if apiPkg.ClientObject().GetNamespace() == "" {
		dst := &corev1alpha1.ClusterPackageList{}

		if err := uncachedClient.List(ctx, dst, &client.ListOptions{LabelSelector: s}); err != nil {
			return nil, fmt.Errorf("list clusterpackage for validate unique constraint: %w", err)
		}

		l = len(dst.Items)
	} else {
		dst := &corev1alpha1.PackageList{}

		if err := uncachedClient.List(ctx, dst, &client.ListOptions{LabelSelector: s}); err != nil {
			return nil, fmt.Errorf("list package for validate unique constraint: %w", err)
		}

		l = len(dst.Items)
	}

	switch l {
	case 0:
		return nil, ErrNonExisting
	case 1:
		return nil, nil
	default:
		return []string{
			"package has unique constraint set but another package in the same namespace uses the same manifest",
		}, nil
	}
}

func validateConstraints(
	ctx context.Context,
	uncachedClient client.Client,
	apiPkg adapters.PackageAccessor, manifest *manifests.PackageManifest, env manifests.PackageEnvironment,
) error {
	var messages []string
	for _, constraint := range manifest.Spec.Constraints {
		if len(constraint.Platform) > 0 {
			if msg, success := platformConstraintMet(constraint.Platform, env); !success {
				messages = append(messages, msg)
			}
		}

		if constraint.PlatformVersion != nil {
			rangeConstraint, err := semver.NewConstraint(constraint.PlatformVersion.Range)
			if err != nil {
				return err
			}
			pv := constraint.PlatformVersion
			var version semver.Version
			ok := true
			switch {
			case pv.Name == manifests.Kubernetes:
				version, err = semver.NewVersion(env.Kubernetes.Version)
			case pv.Name == manifests.OpenShift && env.OpenShift != nil:
				version, err = semver.NewVersion(env.OpenShift.Version)
			default:
				ok = false
			}
			if err != nil {
				return err
			}
			if !ok {
				continue
			}
			if !rangeConstraint.Check(version) {
				messages = append(messages, fmt.Sprintf(
					"%s %s does not meet constraint %s", string(pv.Name), version.String(), pv.Range),
				)
			}
		}
	}

	extra, err := validateUnique(ctx, uncachedClient, apiPkg, manifest)
	if err != nil {
		return err
	}

	messages = append(messages, extra...)

	if len(messages) > 0 {
		meta.SetStatusCondition(apiPkg.GetSpecConditions(), metav1.Condition{
			Type:               corev1alpha1.PackageInvalid,
			Status:             metav1.ConditionTrue,
			Reason:             "ConstraintsFailed",
			Message:            "Constraints not met: " + strings.Join(messages, ", "),
			ObservedGeneration: apiPkg.ClientObject().GetGeneration(),
		})
	}

	return nil
}

func platformConstraintMet(
	pns []manifests.PlatformName, env manifests.PackageEnvironment,
) (message string, success bool) {
	for _, pn := range pns {
		if pn == manifests.OpenShift && env.OpenShift == nil {
			// Constrained to OpenShift platform, but OpenShift not detected.
			return "OpenShift platform", false
		}
	}
	return "", true
}
