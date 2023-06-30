package packageloader

import (
	"context"
	"fmt"
	"io/fs"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
	"package-operator.run/package-operator/internal/packages"
	"package-operator.run/package-operator/internal/packages/packagecontent"
	"package-operator.run/package-operator/internal/rukpak/convert"
)

const (
	olmManifestFolder = "manifests"
	olmMetadataFolder = "metadata"
)

func IsOLMBundle(_ context.Context, files packagecontent.Files) bool {
	var (
		packageManifestFound bool
		manifestsFolderFound bool
		metadataFolderFound  bool
	)
	for fpath := range files {
		if packages.IsManifestFile(fpath) {
			packageManifestFound = true
		}
		if strings.HasPrefix(fpath, olmManifestFolder+"/") {
			manifestsFolderFound = true
		}
		if strings.HasPrefix(fpath, olmMetadataFolder+"/") {
			metadataFolderFound = true
		}
	}
	return !packageManifestFound && manifestsFolderFound && metadataFolderFound
}

func OLMBundleToPackageContent(ctx context.Context, files packagecontent.Files) (*packagecontent.Package, error) {
	// 1. Do OLM internal conversion
	plainFS, err := convert.RegistryV1ToPlain(files.ToFS())
	if err != nil {
		return nil, fmt.Errorf("converting OLM bundle: %w", err)
	}

	const convertedManifestFile = "manifests/manifest.yaml"
	bundleManifests, err := fs.ReadFile(plainFS, convertedManifestFile)
	if err != nil {
		return nil, fmt.Errorf("reading converted manifests: %w", err)
	}

	objects, err := packagecontent.UnstructuredObjectsFromBytes(convertedManifestFile, bundleManifests)
	if err != nil {
		return nil, err
	}

	for i := range objects {
		obj := &objects[i]
		switch obj.GetObjectKind().GroupVersionKind().GroupKind() {
		case schema.GroupKind{Group: "apiextensions.k8s.io", Kind: "CustomResourceDefinition"},
			schema.GroupKind{Group: "", Kind: "Namespace"}:
			setAnnotation(obj, manifestsv1alpha1.PackagePhaseAnnotation, "crds-namespace")
		case schema.GroupKind{Group: "", Kind: "ServiceAccount"},
			schema.GroupKind{Group: "rbac.authorization.k8s.io", Kind: "ClusterRole"},
			schema.GroupKind{Group: "rbac.authorization.k8s.io", Kind: "ClusterRoleBinding"},
			schema.GroupKind{Group: "rbac.authorization.k8s.io", Kind: "Role"},
			schema.GroupKind{Group: "rbac.authorization.k8s.io", Kind: "RoleBinding"}:
			setAnnotation(obj, manifestsv1alpha1.PackagePhaseAnnotation, "rbac")
		default:
			setAnnotation(obj, manifestsv1alpha1.PackagePhaseAnnotation, "deploy")
		}
	}

	pkg := &packagecontent.Package{
		PackageManifest: &manifestsv1alpha1.PackageManifest{
			Spec: manifestsv1alpha1.PackageManifestSpec{
				Scopes: []manifestsv1alpha1.PackageManifestScope{
					manifestsv1alpha1.PackageManifestScopeCluster,
				},
				Phases: []manifestsv1alpha1.PackageManifestPhase{
					{Name: "crds-namespace"},
					{Name: "rbac"},
					{Name: "deploy"},
				},
				AvailabilityProbes: []corev1alpha1.ObjectSetProbe{
					{
						Selector: corev1alpha1.ProbeSelector{
							Kind: &corev1alpha1.PackageProbeKindSpec{
								Group: "apps",
								Kind:  "Deployment",
							},
						},
						Probes: []corev1alpha1.Probe{
							{
								Condition: &corev1alpha1.ProbeConditionSpec{
									Type:   "Available",
									Status: "True",
								},
							},
							{
								FieldsEqual: &corev1alpha1.ProbeFieldsEqualSpec{
									FieldA: ".status.updatedReplicas",
									FieldB: ".status.replicas",
								},
							},
						},
					},
					{
						Selector: corev1alpha1.ProbeSelector{
							Kind: &corev1alpha1.PackageProbeKindSpec{
								Group: "apiextensions.k8s.io",
								Kind:  "CustomResourceDefinition",
							},
						},
						Probes: []corev1alpha1.Probe{
							{
								Condition: &corev1alpha1.ProbeConditionSpec{
									Type:   "Established",
									Status: "True",
								},
							},
						},
					},
				},
			},
		},
		Objects: map[string][]unstructured.Unstructured{
			convertedManifestFile: objects,
		},
	}

	// manifests/manifest.yaml

	return pkg, nil
}

func setAnnotation(obj *unstructured.Unstructured, key, value string) {
	a := obj.GetAnnotations()
	if a == nil {
		a = map[string]string{}
	}
	a[key] = value
	obj.SetAnnotations(a)
}
