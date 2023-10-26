package packageloader_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	apis "package-operator.run/apis"
	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
	"package-operator.run/internal/packages/packagecontent"
	"package-operator.run/internal/packages/packageimport"
	"package-operator.run/internal/packages/packageloader"
)

var testScheme = runtime.NewScheme()

func init() {
	builder := runtime.SchemeBuilder{
		apis.AddToScheme,
		apiextensions.AddToScheme,
		apiextensionsv1.AddToScheme,
	}
	if err := builder.AddToScheme(testScheme); err != nil {
		panic(err)
	}
}

func TestLoader(t *testing.T) {
	t.Parallel()

	foobarValue := "planeplane123"

	transformer, err := packageloader.NewTemplateTransformer(
		packageloader.PackageFileTemplateContext{
			Package: manifestsv1alpha1.TemplateContextPackage{
				TemplateContextObjectMeta: manifestsv1alpha1.TemplateContextObjectMeta{Namespace: "test123-ns"},
			},
			Config: map[string]interface{}{"foobar": foobarValue},
		},
	)
	require.NoError(t, err)

	l := packageloader.New(testScheme, packageloader.WithDefaults, packageloader.WithFilesTransformers(transformer))

	ctx := logr.NewContext(context.Background(), testr.New(t))
	files, err := packageimport.Folder(ctx, "testdata")
	require.NoError(t, err)

	pc, err := l.FromFiles(ctx, files)
	require.NoError(t, err)
	expectedProbes := []corev1alpha1.ObjectSetProbe{
		{
			Selector: corev1alpha1.ProbeSelector{
				Kind: &corev1alpha1.PackageProbeKindSpec{Group: "apps", Kind: "Deployment"},
			},
			Probes: []corev1alpha1.Probe{
				{Condition: &corev1alpha1.ProbeConditionSpec{Type: "Available", Status: "True"}},
				{FieldsEqual: &corev1alpha1.ProbeFieldsEqualSpec{FieldA: ".status.updatedReplicas", FieldB: ".status.replicas"}},
			},
		},
	}

	assert.Equal(t, &manifestsv1alpha1.PackageManifest{
		ObjectMeta: metav1.ObjectMeta{Name: "cool-package"},
		Spec: manifestsv1alpha1.PackageManifestSpec{
			Scopes:             []manifestsv1alpha1.PackageManifestScope{manifestsv1alpha1.PackageManifestScopeNamespaced},
			Phases:             []manifestsv1alpha1.PackageManifestPhase{{Name: "pre-requisites"}, {Name: "main-stuff"}, {Name: "empty"}},
			AvailabilityProbes: expectedProbes,
			Config: manifestsv1alpha1.PackageManifestSpecConfig{
				OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{
					Properties: map[string]apiextensionsv1.JSONSchemaProps{
						"foobar": {
							Description: "some string of some sort",
							Type:        "string",
							Default:     &apiextensionsv1.JSON{Raw: []byte(fmt.Sprintf("\"%s\"", foobarValue))},
						},
					},
					Type: "object",
				},
			},
		},
	}, pc.PackageManifest)

	spec := packagecontent.TemplateSpecFromPackage(pc)
	assert.Equal(t, expectedProbes, spec.AvailabilityProbes)
	assert.Equal(t, []corev1alpha1.ObjectSetTemplatePhase{
		{
			Name: "pre-requisites",
			Objects: []corev1alpha1.ObjectSetObject{
				{
					Object: unstructured.Unstructured{
						Object: map[string]interface{}{
							"apiVersion": "v1",
							"kind":       "ConfigMap",
							"metadata":   map[string]interface{}{"name": "some-configmap"},
							"data":       map[string]interface{}{"foo": "bar", "hello": "world"},
						},
					},
				},
				{
					Object: unstructured.Unstructured{
						Object: map[string]interface{}{
							"apiVersion": "v1",
							"kind":       "ServiceAccount",
							"metadata":   map[string]interface{}{"name": "some-service-account"},
						},
					},
				},
			},
		},
		{
			Name: "main-stuff",
			Objects: []corev1alpha1.ObjectSetObject{
				{
					Object: unstructured.Unstructured{
						Object: map[string]interface{}{
							"apiVersion": "apps/v1",
							"kind":       "Deployment",
							"metadata": map[string]interface{}{
								"name": "controller-manager", "namespace": "test123-ns",
								"annotations": map[string]interface{}{
									"other-test-helper":  "other-test-helper",
									"test-helper":        "test-helper",
									"foobar-from-config": foobarValue,
									"include-test":       "\nKEY1: VAL1\nKEY2: VAL2",
									"fileGet":            "lorem ipsum...\n",
									"certificate-key":    "-----BEGIN CERTIFICATE-----\nMIIEvgIBADANBgkqhkiG9w0BAQEFAASCBKgwggSkAgEAAoIBAQDBj08sp5++4anG\n-----END CERTIFICATE-----\n",
								},
							},
							"spec": map[string]interface{}{"replicas": int64(1)},
						},
					},
				},
				{
					Object: unstructured.Unstructured{
						Object: map[string]interface{}{
							"apiVersion": "apps/v1",
							"kind":       "StatefulSet",
							"metadata":   map[string]interface{}{"name": "some-stateful-set-1"},
							"spec":       map[string]interface{}{},
						},
					},
				},
			},
		},
	}, spec.Phases)
}

var testPackageManifestContent = `apiVersion: manifests.package-operator.run/v1alpha1
kind: PackageManifest
metadata:
  name: test
spec:
  scopes:
  - Cluster
  phases:
  - name: test
  availabilityProbes:
  - probes:
    - {}
`

var testFile1Content = `apiVersion: v1
kind: Test
metadata:
  name: testfile1
  annotations:
    package-operator.run/phase: tesxx
property: {{.package.metadata.name}}
`

var testFile1UpdatedContent = `apiVersion: v1
kind: Test
metadata:
  name: testfile1
  annotations:
    package-operator.run/phase: tesxx
property: {{.package.metadata.name}}xxx
`

var testFile2Content = `apiVersion: v1
kind: Test
metadata:
  name: testfile2
  annotations:
    package-operator.run/phase: tesxx
property: {{.package.metadata.namespace}}
`

func TestTemplateTestValidator(t *testing.T) {
	t.Parallel()
	fixturesPath := t.TempDir()
	defer func() {
		err := os.RemoveAll(fixturesPath)
		require.NoError(t, err) // start clean
	}()

	packageManifest := &manifestsv1alpha1.PackageManifest{
		Test: manifestsv1alpha1.PackageManifestTest{
			Template: []manifestsv1alpha1.PackageManifestTestCaseTemplate{
				{
					Name: "t1",
					Context: manifestsv1alpha1.TemplateContext{
						Package: manifestsv1alpha1.TemplateContextPackage{
							TemplateContextObjectMeta: manifestsv1alpha1.
								TemplateContextObjectMeta{
								Name:      "pkg-name",
								Namespace: "pkg-namespace",
							},
						},
					},
				},
			},
		},
	}

	pc := &packagecontent.Package{PackageManifest: packageManifest}

	ctx := logr.NewContext(context.Background(), testr.New(t))
	ttv := packageloader.NewTemplateTestValidator(testScheme, fixturesPath)

	originalFileMap := packagecontent.Files{
		"manifest.yaml":     []byte(testPackageManifestContent),
		"file2.yaml.gotmpl": []byte(testFile2Content), "file.yaml.gotmpl": []byte(testFile1Content),
	}
	err := ttv.ValidatePackageAndFiles(ctx, pc, originalFileMap)
	require.NoError(t, err)

	// Assert Fixtures have been setup
	newFileMap := packagecontent.Files{
		"manifest.yaml":     []byte(testPackageManifestContent),
		"file2.yaml.gotmpl": []byte(testFile2Content), "file.yaml.gotmpl": []byte(testFile1UpdatedContent),
	}
	expectedErr := `File mismatch against fixture in file.yaml.gotmpl: Testcase "t1"
--- FIXTURE/file.yaml
+++ ACTUAL/file.yaml
@@ -4,4 +4,4 @@
   name: testfile1
   annotations:
     package-operator.run/phase: tesxx
-property: pkg-name
+property: pkg-namexxx`
	err = ttv.ValidatePackageAndFiles(ctx, pc, newFileMap)
	require.Equal(t, expectedErr, err.Error())
}

func TestCommonObjectLabelsTransformer(t *testing.T) {
	t.Parallel()

	colt := &packageloader.PackageTransformer{
		Package: &metav1.ObjectMeta{Name: "sepp"},
	}

	packageContent := &packagecontent.Package{
		PackageManifest: &manifestsv1alpha1.PackageManifest{
			ObjectMeta: metav1.ObjectMeta{Name: "my-cool-pkg"},
		},
		Objects: map[string][]unstructured.Unstructured{
			"test.yaml": {{}},
		},
	}

	ctx := context.Background()
	err := colt.TransformPackage(ctx, packageContent)
	require.NoError(t, err)
	obj := packageContent.Objects["test.yaml"][0]
	assert.Equal(t, map[string]string{
		manifestsv1alpha1.PackageInstanceLabel: "sepp",
		manifestsv1alpha1.PackageLabel:         "my-cool-pkg",
	}, obj.GetLabels())
}

func TestTemplateTransformer(t *testing.T) {
	t.Parallel()
	t.Run("success", func(t *testing.T) {
		t.Parallel()

		tt, err := packageloader.NewTemplateTransformer(
			packageloader.PackageFileTemplateContext{
				Package: manifestsv1alpha1.TemplateContextPackage{
					TemplateContextObjectMeta: manifestsv1alpha1.TemplateContextObjectMeta{Name: "test"},
				},
			},
		)
		require.NoError(t, err)

		template := []byte("#{{.package.metadata.name}}#")
		fm := packagecontent.Files{
			"something":        template,
			"something.yaml":   template,
			"test.yaml.gotmpl": template,
			"test.yml.gotmpl":  template,
		}

		ctx := context.Background()
		err = tt.TransformPackageFiles(ctx, fm)
		require.NoError(t, err)

		templateResult := "#test#"
		assert.Equal(t, templateResult, string(fm["test.yaml"]))
		assert.Equal(t, templateResult, string(fm["test.yml"]))
		// only touches YAML files
		assert.Equal(t, string(template), string(fm["something"]))
	})

	t.Run("invalid template", func(t *testing.T) {
		t.Parallel()
		tt, err := packageloader.NewTemplateTransformer(
			packageloader.PackageFileTemplateContext{
				Package: manifestsv1alpha1.TemplateContextPackage{
					TemplateContextObjectMeta: manifestsv1alpha1.TemplateContextObjectMeta{Name: "test"},
				},
			},
		)
		require.NoError(t, err)

		template := []byte("#{{.package.metadata.name}#")
		fm := packagecontent.Files{"test.yaml.gotmpl": template}

		ctx := context.Background()
		err = tt.TransformPackageFiles(ctx, fm)
		require.Error(t, err)
	})

	t.Run("execution template error", func(t *testing.T) {
		t.Parallel()

		tt, err := packageloader.NewTemplateTransformer(
			packageloader.PackageFileTemplateContext{
				Package: manifestsv1alpha1.TemplateContextPackage{
					TemplateContextObjectMeta: manifestsv1alpha1.TemplateContextObjectMeta{Name: "test"},
				},
			},
		)
		require.NoError(t, err)

		template := []byte("#{{.Package.Banana}}#")
		fm := packagecontent.Files{"test.yaml.gotmpl": template}

		ctx := context.Background()
		err = tt.TransformPackageFiles(ctx, fm)
		require.Error(t, err)
	})
}

func TestObjectPhaseAnnotationValidator(t *testing.T) {
	t.Parallel()

	opav := &packageloader.ObjectPhaseAnnotationValidator{}

	okObj := unstructured.Unstructured{}
	okObj.SetAnnotations(map[string]string{
		manifestsv1alpha1.PackagePhaseAnnotation: "something",
	})
	packageContent := &packagecontent.Package{
		Objects: map[string][]unstructured.Unstructured{
			"test.yaml": {{}, okObj},
		},
	}

	ctx := context.Background()
	err := opav.ValidatePackage(ctx, packageContent)
	require.EqualError(t, err, "Missing package-operator.run/phase Annotation in test.yaml idx 0")
}

func TestObjectDuplicateValidator(t *testing.T) {
	t.Parallel()

	odv := &packageloader.ObjectDuplicateValidator{}

	obj := unstructured.Unstructured{}
	obj.SetAnnotations(map[string]string{
		manifestsv1alpha1.PackagePhaseAnnotation: "something",
	})
	packageContent := &packagecontent.Package{
		Objects: map[string][]unstructured.Unstructured{
			"test.yaml": {{}, obj},
		},
	}

	ctx := context.Background()
	err := odv.ValidatePackage(ctx, packageContent)
	require.EqualError(t, err, "Duplicate Object in test.yaml idx 1")
}

func TestObjectGVKValidator(t *testing.T) {
	t.Parallel()

	ogvkv := &packageloader.ObjectGVKValidator{}

	okObj := unstructured.Unstructured{}
	okObj.SetGroupVersionKind(schema.GroupVersionKind{
		Version: "v1",
		Kind:    "Secret",
	})
	packageContent := &packagecontent.Package{
		Objects: map[string][]unstructured.Unstructured{
			"test.yaml": {{}, okObj},
		},
	}

	ctx := context.Background()
	err := ogvkv.ValidatePackage(ctx, packageContent)
	require.EqualError(t, err, "GroupVersionKind not set in test.yaml idx 0")
}

func TestObjectLabelsValidator(t *testing.T) {
	t.Parallel()

	olv := &packageloader.ObjectLabelsValidator{}

	failObj := unstructured.Unstructured{}
	failObj.SetLabels(map[string]string{"/123": "test"})

	packageContent := &packagecontent.Package{
		Objects: map[string][]unstructured.Unstructured{
			"test.yaml": {{}, failObj},
		},
	}
	ctx := context.Background()
	err := olv.ValidatePackage(ctx, packageContent)
	errString := `Labels invalid in test.yaml idx 1: metadata.labels: Invalid value: "/123": prefix part must be non-empty`
	require.EqualError(t, err, errString)
}

func TestPackageScopeValidator(t *testing.T) {
	t.Parallel()

	scopeV := packageloader.PackageScopeValidator(manifestsv1alpha1.PackageManifestScopeCluster)

	ctx := context.Background()
	err := scopeV.ValidatePackage(ctx, &packagecontent.Package{
		PackageManifest: &manifestsv1alpha1.PackageManifest{
			Spec: manifestsv1alpha1.PackageManifestSpec{
				Scopes: []manifestsv1alpha1.PackageManifestScope{manifestsv1alpha1.PackageManifestScopeNamespaced},
			},
		},
	})
	require.EqualError(t, err, "Package unsupported scope in manifest.yaml")
}

func TestLoaderOnMultiComponentPackageWithConfig(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		Directory   string
		Component   string
		ObjectNames []string
		Config      map[string]interface{}
	}{
		"simple": {
			Directory:   "simple-with-config",
			ObjectNames: []string{"configmap.yaml", "deployment.yaml", "service.yaml"},
			Config:      map[string]interface{}{"apiBaseUrl": "http://localhost:12345"},
		},
		"multi/root": {
			Directory:   "multi-with-config",
			Component:   "",
			ObjectNames: []string{"backend-package.yaml", "frontend-package.yaml"},
		},
		"multi/backend": {
			Directory:   "multi-with-config",
			Component:   "backend",
			ObjectNames: []string{"deployment.yaml", "service.yaml"},
		},
		"multi/frontend": {
			Directory:   "multi-with-config",
			Component:   "frontend",
			ObjectNames: []string{"configmap.yaml", "deployment.yaml", "service.yaml"},
			Config:      map[string]interface{}{"apiBaseUrl": "http://localhost:12345"},
		},
	} {
		tc := tc

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			transformer, err := packageloader.NewTemplateTransformer(
				packageloader.PackageFileTemplateContext{
					Package: manifestsv1alpha1.TemplateContextPackage{
						TemplateContextObjectMeta: manifestsv1alpha1.TemplateContextObjectMeta{Namespace: "test123-ns"},
					},
					Config: tc.Config,
				},
			)
			require.NoError(t, err)

			l := packageloader.New(testScheme, packageloader.WithDefaults, packageloader.WithFilesTransformers(transformer), packageloader.WithComponent(tc.Component))

			ctx := logr.NewContext(context.Background(), testr.New(t))
			files, err := packageimport.Folder(ctx, filepath.Join("..", "..", "testutil", "testdata", tc.Directory))
			require.NoError(t, err)

			pkg, err := l.FromFiles(ctx, files)
			require.NoError(t, err)

			for _, obj := range tc.ObjectNames {
				require.Contains(t, pkg.Objects, obj)
			}
		})
	}
}
