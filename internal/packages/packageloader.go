package packages

import (
	"context"
	"errors"
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
)

type folderLoader interface {
	Load(
		ctx context.Context, rootPath string,
		templateContext FolderLoaderTemplateContext,
	) (res FolderLoaderResult, err error)
}

type deploymentReconciler interface {
	Reconcile(
		ctx context.Context, desiredDeploy genericObjectDeployment,
		chunker objectChunker,
	) error
}

// PackageLoader loads an ObjectDeployment from file, wraps it into an ObjectDeployment and updates/creates it on the cluster.
type PackageLoader struct {
	client client.Client
	scheme *runtime.Scheme

	newPackage          genericPackageFactory
	newObjectDeployment genericObjectDeploymentFactory

	deploymentReconciler deploymentReconciler
	folderLoader         folderLoader
}

// Returns a new namespace-scoped loader for the Package API.
func NewPackageLoader(c client.Client, scheme *runtime.Scheme) *PackageLoader {
	return &PackageLoader{
		client: c,
		scheme: scheme,

		newPackage:          newGenericPackage,
		newObjectDeployment: newGenericObjectDeployment,

		folderLoader: NewFolderLoader(scheme),
		deploymentReconciler: newDeploymentReconciler(
			scheme, c,
			newGenericObjectDeployment, newGenericObjectSlice,
			newGenericObjectSliceList, newGenericObjectSetList,
		),
	}
}

// Returns a new cluster-scoped loader for the ClusterPackage API.
func NewClusterPackageLoader(c client.Client, scheme *runtime.Scheme) *PackageLoader {
	return &PackageLoader{
		client: c,
		scheme: scheme,

		newPackage:          newGenericClusterPackage,
		newObjectDeployment: newGenericClusterObjectDeployment,

		folderLoader: NewFolderLoader(scheme),
		deploymentReconciler: newDeploymentReconciler(scheme, c, newGenericClusterObjectDeployment, newGenericClusterObjectSlice,
			newGenericClusterObjectSliceList, newGenericClusterObjectSetList,
		),
	}
}

func (l *PackageLoader) Load(ctx context.Context, packageKey client.ObjectKey, folderPath string) error {
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

func (l *PackageLoader) load(ctx context.Context, pkg genericPackage, folderPath string) error {
	res, err := l.loadFromFolder(ctx, pkg, folderPath)
	if err != nil {
		setInvalidConditionBasedOnLoadError(pkg, err)
		return nil
	}

	desiredDeploy, err := l.desiredObjectDeployment(ctx, pkg, res)
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

func (l *PackageLoader) desiredObjectDeployment(
	ctx context.Context, pkg genericPackage, res FolderLoaderResult,
) (deploy genericObjectDeployment, err error) {
	deploy = l.newObjectDeployment(l.scheme)
	return deploy, l.setObjectDeploymentFields(ctx, pkg, res, deploy)
}

// Sets fields in the ObjectDeployment to the desired values.
func (l *PackageLoader) setObjectDeploymentFields(
	ctx context.Context, pkg genericPackage, res FolderLoaderResult,
	deploy genericObjectDeployment,
) error {
	annotations := mergeKeysFrom(deploy.ClientObject().GetAnnotations(), res.Annotations)
	labels := mergeKeysFrom(deploy.ClientObject().GetLabels(), res.Labels)

	deploy.ClientObject().SetAnnotations(annotations)
	deploy.ClientObject().SetLabels(labels)
	deploy.ClientObject().SetName(pkg.ClientObject().GetName())
	deploy.ClientObject().SetNamespace(pkg.ClientObject().GetNamespace())

	deploy.SetTemplateSpec(res.TemplateSpec)
	deploy.SetSelector(map[string]string{
		manifestsv1alpha1.PackageLabel:         res.Manifest.Name,
		manifestsv1alpha1.PackageInstanceLabel: pkg.ClientObject().GetName(),
	})

	if err := controllerutil.SetControllerReference(
		pkg.ClientObject(), deploy.ClientObject(), l.scheme); err != nil {
		return err
	}
	return nil
}

func (l *PackageLoader) loadFromFolder(
	ctx context.Context, pkg genericPackage, folderPath string,
) (res FolderLoaderResult, err error) {
	res, err = l.folderLoader.Load(ctx, folderPath, pkg.TemplateContext())
	if err != nil {
		return res, err
	}

	if !contains(res.Manifest.Spec.Scopes, pkg.Scope()) {
		// Package does not support installation in this context scope.
		return res, &PackageInvalidScopeError{
			RequiredScope:   pkg.Scope(),
			SupportedScopes: res.Manifest.Spec.Scopes,
		}
	}

	return res, nil
}

func contains[T comparable](elems []T, v T) bool {
	for _, s := range elems {
		if v == s {
			return true
		}
	}
	return false
}

func setInvalidConditionBasedOnLoadError(pkg genericPackage, err error) {
	reason := "LoadError"

	var (
		notFoundErr        *PackageManifestNotFoundError
		invalidManifestErr *PackageManifestInvalidError
		invalidScopeErr    *PackageInvalidScopeError
		objectInvalidErr   *PackageObjectInvalidError
	)
	switch {
	case errors.As(err, &notFoundErr):
		reason = "PackageManifestNotFound"
	case errors.As(err, &invalidManifestErr):
		reason = "PackageManifestInvalid"
	case errors.As(err, &invalidScopeErr):
		reason = "InvalidScope"
	case errors.As(err, &objectInvalidErr):
		reason = "InvalidObject"
	}

	// Can not be determined more precisely
	meta.SetStatusCondition(pkg.GetConditions(), metav1.Condition{
		Type:               corev1alpha1.PackageInvalid,
		Status:             metav1.ConditionTrue,
		Reason:             reason,
		Message:            err.Error(),
		ObservedGeneration: pkg.ClientObject().GetGeneration(),
	})
}

func mergeKeysFrom(base, additional map[string]string) map[string]string {
	if base == nil {
		base = map[string]string{}
	}
	for k, v := range additional {
		base[k] = v
	}
	if len(base) == 0 {
		return nil
	}
	return base
}
