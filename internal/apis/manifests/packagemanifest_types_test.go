package manifests

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

func TestPackageManifest_Structure(t *testing.T) {
	t.Parallel()

	pm := &PackageManifest{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "manifests.package-operator.run/v1alpha1",
			Kind:       "PackageManifest",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-package",
			Namespace: "test-namespace",
		},
		Spec: PackageManifestSpec{
			Scopes: []PackageManifestScope{
				PackageManifestScopeCluster,
				PackageManifestScopeNamespaced,
			},
			Phases: []PackageManifestPhase{
				{
					Name:  "phase1",
					Class: "default",
				},
			},
		},
	}

	assert.Equal(t, "test-package", pm.Name)
	assert.Equal(t, "test-namespace", pm.Namespace)
	assert.Len(t, pm.Spec.Scopes, 2)
	assert.Contains(t, pm.Spec.Scopes, PackageManifestScopeCluster)
	assert.Contains(t, pm.Spec.Scopes, PackageManifestScopeNamespaced)
	assert.Len(t, pm.Spec.Phases, 1)
	assert.Equal(t, "phase1", pm.Spec.Phases[0].Name)
}

func TestPackageManifestScope_Constants(t *testing.T) {
	t.Parallel()

	assert.Equal(t, PackageManifestScopeCluster, PackageManifestScope("Cluster"))
	assert.Equal(t, PackageManifestScopeNamespaced, PackageManifestScope("Namespaced"))
}

func TestPlatformName_Constants(t *testing.T) {
	t.Parallel()

	assert.Equal(t, Kubernetes, PlatformName("Kubernetes"))
	assert.Equal(t, OpenShift, PlatformName("OpenShift"))
}

func TestPackageManifestSpec_AllFields(t *testing.T) {
	t.Parallel()

	spec := PackageManifestSpec{
		Scopes: []PackageManifestScope{PackageManifestScopeCluster},
		Phases: []PackageManifestPhase{
			{Name: "deploy", Class: "default"},
		},
		AvailabilityProbes: []corev1alpha1.ObjectSetProbe{
			{
				Probes: []corev1alpha1.Probe{
					{
						Condition: &corev1alpha1.ProbeConditionSpec{
							Type:   "Available",
							Status: "True",
						},
					},
				},
				Selector: corev1alpha1.ProbeSelector{
					Kind: &corev1alpha1.PackageProbeKindSpec{
						Group: "apps",
						Kind:  "Deployment",
					},
				},
			},
		},
		Config: PackageManifestSpecConfig{
			OpenAPIV3Schema: &apiextensions.JSONSchemaProps{
				Type: "object",
			},
		},
		Images: []PackageManifestImage{
			{
				Name:  "app",
				Image: "nginx:latest",
			},
		},
		Components: &PackageManifestComponentsConfig{},
		Filters: PackageManifestFilter{
			Conditions: []PackageManifestNamedCondition{
				{
					Name:       "isOpenShift",
					Expression: "has(environment.openShift)",
				},
			},
			Paths: []PackageManifestPath{
				{
					Glob:       "*/openshift/*",
					Expression: "isOpenShift",
				},
			},
		},
		Constraints: []PackageManifestConstraint{
			{
				Platform: []PlatformName{OpenShift},
				PlatformVersion: &PackageManifestPlatformVersionConstraint{
					Name:  OpenShift,
					Range: ">=4.10",
				},
				UniqueInScope: &PackageManifestUniqueInScopeConstraint{},
			},
		},
		Repositories: []PackageManifestRepository{
			{
				File: "../repo.yaml",
			},
			{
				Image: "quay.io/example/repo:latest",
			},
		},
		Dependencies: []PackageManifestDependency{
			{
				Image: &PackageManifestDependencyImage{
					Name:    "dependency",
					Package: "dep.example",
					Range:   ">=1.0",
				},
			},
		},
	}

	assert.Len(t, spec.Scopes, 1)
	assert.Len(t, spec.Phases, 1)
	assert.Len(t, spec.AvailabilityProbes, 1)
	assert.NotNil(t, spec.Config.OpenAPIV3Schema)
	assert.Len(t, spec.Images, 1)
	assert.NotNil(t, spec.Components)
	assert.Len(t, spec.Filters.Conditions, 1)
	assert.Len(t, spec.Filters.Paths, 1)
	assert.Len(t, spec.Constraints, 1)
	assert.Len(t, spec.Repositories, 2)
	assert.Len(t, spec.Dependencies, 1)

	assert.Equal(t, "isOpenShift", spec.Filters.Conditions[0].Name)
	assert.Equal(t, "has(environment.openShift)", spec.Filters.Conditions[0].Expression)
	assert.Equal(t, "*/openshift/*", spec.Filters.Paths[0].Glob)
	assert.Equal(t, "isOpenShift", spec.Filters.Paths[0].Expression)
}

func TestPackageManifestTest_Structure(t *testing.T) {
	t.Parallel()

	test := PackageManifestTest{
		Template: []PackageManifestTestCaseTemplate{
			{
				Name: "basic-test",
				Context: TemplateContext{
					Package: TemplateContextPackage{
						TemplateContextObjectMeta: TemplateContextObjectMeta{
							Name:      "test-package",
							Namespace: "default",
							Labels: map[string]string{
								"app": "test",
							},
							Annotations: map[string]string{
								"version": "1.0",
							},
						},
						Image: "example.com/test:v1.0.0",
					},
					Config: &runtime.RawExtension{
						Raw: []byte(`{"replicas": 3}`),
					},
					Environment: PackageEnvironment{
						Kubernetes: PackageEnvironmentKubernetes{
							Version: "1.25.0",
						},
						OpenShift: &PackageEnvironmentOpenShift{
							Version: "4.12",
							Managed: &PackageEnvironmentManagedOpenShift{
								Data: map[string]string{
									"provider": "rosa",
								},
							},
						},
						Proxy: &PackageEnvironmentProxy{
							HTTPProxy:  "http://proxy:8080",
							HTTPSProxy: "https://proxy:8443",
							NoProxy:    "localhost,127.0.0.1",
						},
						HyperShift: &PackageEnvironmentHyperShift{
							HostedCluster: &PackageEnvironmentHyperShiftHostedCluster{
								TemplateContextObjectMeta: TemplateContextObjectMeta{
									Name:      "hosted-cluster",
									Namespace: "clusters",
									Labels: map[string]string{
										"cluster-type": "hosted",
									},
								},
								HostedClusterNamespace: "clusters-hosted-cluster",
								NodeSelector: map[string]string{
									"node-role.kubernetes.io/worker": "",
								},
							},
						},
					},
				},
			},
		},
		Kubeconform: &PackageManifestTestKubeconform{
			KubernetesVersion: "1.25.0",
			SchemaLocations: []string{
				//nolint:lll
				"https://raw.githubusercontent.com/yannh/kubernetes-json-schema/master/v1.25.0-standalone-strict/deployment-apps-v1.json",
			},
		},
	}

	assert.Len(t, test.Template, 1)
	assert.Equal(t, "basic-test", test.Template[0].Name)
	assert.Equal(t, "test-package", test.Template[0].Context.Package.Name)
	assert.Equal(t, "default", test.Template[0].Context.Package.Namespace)
	assert.Equal(t, "example.com/test:v1.0.0", test.Template[0].Context.Package.Image)
	assert.NotNil(t, test.Template[0].Context.Config)
	assert.Equal(t, "1.25.0", test.Template[0].Context.Environment.Kubernetes.Version)
	assert.NotNil(t, test.Template[0].Context.Environment.OpenShift)
	assert.Equal(t, "4.12", test.Template[0].Context.Environment.OpenShift.Version)
	assert.NotNil(t, test.Template[0].Context.Environment.OpenShift.Managed)
	assert.Equal(t, "rosa", test.Template[0].Context.Environment.OpenShift.Managed.Data["provider"])
	assert.NotNil(t, test.Template[0].Context.Environment.Proxy)
	assert.Equal(t, "http://proxy:8080", test.Template[0].Context.Environment.Proxy.HTTPProxy)
	assert.NotNil(t, test.Template[0].Context.Environment.HyperShift)
	assert.NotNil(t, test.Template[0].Context.Environment.HyperShift.HostedCluster)
	assert.Equal(t, "hosted-cluster", test.Template[0].Context.Environment.HyperShift.HostedCluster.Name)
	assert.Equal(
		t,
		"clusters-hosted-cluster",
		test.Template[0].Context.Environment.HyperShift.HostedCluster.HostedClusterNamespace,
	)

	assert.NotNil(t, test.Kubeconform)
	assert.Equal(t, "1.25.0", test.Kubeconform.KubernetesVersion)
	assert.Len(t, test.Kubeconform.SchemaLocations, 1)
}

func TestTemplateContextObjectMeta(t *testing.T) {
	t.Parallel()

	meta := TemplateContextObjectMeta{
		Name:      "test-object",
		Namespace: "test-ns",
		Labels: map[string]string{
			"app":     "test",
			"version": "v1",
		},
		Annotations: map[string]string{
			"description": "test object",
			"owner":       "team-a",
		},
	}

	assert.Equal(t, "test-object", meta.Name)
	assert.Equal(t, "test-ns", meta.Namespace)
	assert.Equal(t, "test", meta.Labels["app"])
	assert.Equal(t, "v1", meta.Labels["version"])
	assert.Equal(t, "test object", meta.Annotations["description"])
	assert.Equal(t, "team-a", meta.Annotations["owner"])
}

func TestPackageManifestConstants(t *testing.T) {
	t.Parallel()

	// Test that constants are properly set
	assert.NotEmpty(t, PackagePhaseAnnotation)
	assert.NotEmpty(t, PackageConditionMapAnnotation)
	assert.NotEmpty(t, PackageCELConditionAnnotation)
	assert.NotEmpty(t, PackageLabel)
	assert.NotEmpty(t, PackageSourceImageAnnotation)
	assert.NotEmpty(t, PackageConfigAnnotation)
	assert.NotEmpty(t, PackageInstanceLabel)
}
