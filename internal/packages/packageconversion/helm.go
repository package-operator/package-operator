package packageconversion

import (
	"context"
	"encoding/json"
	"fmt"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
	"package-operator.run/internal/adapters"
	"package-operator.run/internal/packages/packagecontent"
)

const manifestsFile = "manifest.yaml"

func Helm(
	ctx context.Context,
	pkg adapters.GenericPackageAccessor,
	helmFiles packagecontent.Files,
) (content *packagecontent.Package, err error) {
	client := action.NewInstall(&action.Configuration{})
	client.DryRun = true
	client.ReleaseName = pkg.ClientObject().GetName()
	client.Namespace = pkg.ClientObject().GetNamespace()
	client.Replace = true
	client.IsUpgrade = pkg.GetStatusRevision() != 0
	client.ClientOnly = true

	chart, err := loader.LoadFiles(toBufferedFiles(helmFiles))
	if err != nil {
		return nil, err
	}

	configuration := map[string]interface{}{}
	tmplCtx := pkg.TemplateContext()
	if tmplCtx.Config != nil {
		if err := json.Unmarshal(tmplCtx.Config.Raw, &configuration); err != nil {
			return nil, fmt.Errorf("unmarshal config: %w", err)
		}
	}

	res, err := client.RunWithContext(ctx, chart, configuration)
	if err != nil {
		return nil, err
	}

	objects, err := packagecontent.UnstructuredFromFile(
		"manifest.yaml", []byte(res.Manifest))
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

	content = &packagecontent.Package{
		PackageManifest: &manifestsv1alpha1.PackageManifest{
			ObjectMeta: metav1.ObjectMeta{
				Name: chart.Metadata.Name,
			},
			Spec: manifestsv1alpha1.PackageManifestSpec{
				Scopes: []manifestsv1alpha1.PackageManifestScope{
					manifestsv1alpha1.PackageManifestScopeCluster,
					manifestsv1alpha1.PackageManifestScopeNamespaced,
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
			manifestsFile: objects,
		},
	}
	return
}

func toBufferedFiles(f packagecontent.Files) (out []*loader.BufferedFile) {
	out = make([]*loader.BufferedFile, len(f))
	var i int
	for p, d := range f {
		out[i] = &loader.BufferedFile{
			Name: p,
			Data: d,
		}
		i++
	}
	return
}

func setAnnotation(obj *unstructured.Unstructured, key, value string) {
	a := obj.GetAnnotations()
	if a == nil {
		a = map[string]string{}
	}
	a[key] = value
	obj.SetAnnotations(a)
}
