package packages

import (
	"context"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
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

// PackageLoader loads an ObjectDeployment from file, wraps it into an ObjectDeployment and updates/creates it on the cluster.
type PackageLoader struct {
	client client.Client
	scheme *runtime.Scheme

	newPackage          genericPackageFactory
	newObjectDeployment genericObjectDeploymentFactory

	folderLoader folderLoader
}

// Returns a new namespace-scoped loader for the Package API.
func NewPackageLoader(c client.Client, scheme *runtime.Scheme) *PackageLoader {
	return &PackageLoader{
		client: c,
		scheme: scheme,

		newPackage:          newGenericPackage,
		newObjectDeployment: newGenericObjectDeployment,

		folderLoader: NewFolderLoader(scheme),
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

	unpackedCondition := meta.FindStatusCondition(*pkg.GetConditions(), corev1alpha1.PackageUnpacked)
	if unpackedCondition == nil {
		return nil
	}
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		log.Info("trying to report Package status...")

		meta.SetStatusCondition(pkg.GetConditions(), *unpackedCondition) // reapply condition after update
		if err := l.client.Status().Update(ctx, pkg.ClientObject()); err != nil {
			return err
		}
		return nil
	})
}

func (l *PackageLoader) load(ctx context.Context, pkg genericPackage, folderPath string) error {
	res, err := l.loadFromFolder(ctx, pkg, folderPath)
	if err != nil {
		meta.SetStatusCondition(pkg.GetConditions(), metav1.Condition{
			Type:               corev1alpha1.PackageUnpacked,
			Status:             metav1.ConditionFalse,
			Reason:             "LoadError",
			Message:            err.Error(),
			ObservedGeneration: pkg.ClientObject().GetGeneration(),
		})
		return nil
	}

	deploy := l.newObjectDeployment(l.scheme)
	err = l.client.Get(ctx, client.ObjectKeyFromObject(pkg.ClientObject()), deploy.ClientObject())
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

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

	if errors.IsNotFound(err) {
		if err := l.client.Create(ctx, deploy.ClientObject()); err != nil {
			return err
		}

		meta.SetStatusCondition(pkg.GetConditions(), metav1.Condition{
			Type:               corev1alpha1.PackageUnpacked,
			Status:             metav1.ConditionTrue,
			Reason:             "LoadSuccess",
			ObservedGeneration: pkg.ClientObject().GetGeneration(),
		})

		return nil
	}

	err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
		if err := l.client.Update(ctx, deploy.ClientObject()); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}

	meta.SetStatusCondition(pkg.GetConditions(), metav1.Condition{
		Type:               corev1alpha1.PackageUnpacked,
		Status:             metav1.ConditionTrue,
		Reason:             "LoadSuccess",
		ObservedGeneration: pkg.ClientObject().GetGeneration(),
	})
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
