package packagevalidation

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"package-operator.run/internal/apis/manifests"
	"package-operator.run/internal/packages/internal/packagetypes"
)

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
	validatorPath := t.TempDir()
	defer func() {
		err := os.RemoveAll(validatorPath)
		require.NoError(t, err) // start clean
	}()

	packageManifest := &manifests.PackageManifest{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-pkg",
		},
		Spec: manifests.PackageManifestSpec{
			Phases: []manifests.PackageManifestPhase{
				{Name: "tesxx"},
			},
		},
		Test: manifests.PackageManifestTest{
			Template: []manifests.PackageManifestTestCaseTemplate{
				{
					Name: "t1",
					Context: manifests.TemplateContext{
						Package: manifests.TemplateContextPackage{
							TemplateContextObjectMeta: manifests.
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

	ctx := logr.NewContext(t.Context(), testr.New(t))
	ttv := NewTemplateTestValidator(validatorPath)

	originalFileMap := packagetypes.Files{
		"file2.yaml.gotmpl": []byte(testFile2Content),
		"file.yaml.gotmpl":  []byte(testFile1Content),
	}
	originalPkg := &packagetypes.Package{
		Manifest: packageManifest,
		Files:    originalFileMap,
	}
	err := ttv.ValidatePackage(ctx, originalPkg)
	require.NoError(t, err)

	// Assert Fixtures have been setup
	newFileMap := packagetypes.Files{
		"file2.yaml.gotmpl": []byte(testFile2Content),
		"file.yaml.gotmpl":  []byte(testFile1UpdatedContent),
	}
	expectedErr := `File mismatch against fixture in file.yaml: Testcase "t1"
--- FIXTURE/file.yaml
+++ ACTUAL/file.yaml
@@ -4,4 +4,4 @@
   name: testfile1
   annotations:
     package-operator.run/phase: tesxx
-property: pkg-name
+property: pkg-namexxx`
	newPkg := &packagetypes.Package{
		Manifest: packageManifest,
		Files:    newFileMap,
	}
	err = ttv.ValidatePackage(ctx, newPkg)
	require.Equal(t, expectedErr, err.Error())
}

func TestTemplateTestValidator_errorUnknownFile(t *testing.T) {
	t.Parallel()
	validatorPath := t.TempDir()
	defer func() {
		err := os.RemoveAll(validatorPath)
		require.NoError(t, err) // start clean
	}()

	t1FixturePath := filepath.Join(validatorPath, testFixturesFolderName, "t1")
	require.NoError(t, os.MkdirAll(t1FixturePath, os.ModePerm))
	require.NoError(t, os.WriteFile(filepath.Join(t1FixturePath, "banana.yaml"), []byte("xxx"), os.ModePerm))
	require.NoError(t, os.WriteFile(filepath.Join(t1FixturePath, "file.yaml"), []byte("xxx\n"), os.ModePerm))

	packageManifest := &manifests.PackageManifest{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-pkg",
		},
		Spec: manifests.PackageManifestSpec{
			Phases: []manifests.PackageManifestPhase{
				{Name: "tesxx"},
			},
		},
		Test: manifests.PackageManifestTest{
			Template: []manifests.PackageManifestTestCaseTemplate{
				{
					Name: "t1",
					Context: manifests.TemplateContext{
						Package: manifests.TemplateContextPackage{
							TemplateContextObjectMeta: manifests.
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

	ctx := logr.NewContext(t.Context(), testr.New(t))
	ttv := NewTemplateTestValidator(validatorPath)

	originalFileMap := packagetypes.Files{
		"file.yaml.gotmpl": []byte(testFile1Content),
	}
	originalPkg := &packagetypes.Package{
		Manifest: packageManifest,
		Files:    originalFileMap,
	}
	err := ttv.ValidatePackage(ctx, originalPkg)
	expectedErr := `file banana.yaml should not exist, filtered or empty after template render
File mismatch against fixture in file.yaml: Testcase "t1"
--- FIXTURE/file.yaml
+++ ACTUAL/file.yaml
@@ -1 +1,7 @@
-xxx
+apiVersion: v1
+kind: Test
+metadata:
+  name: testfile1
+  annotations:
+    package-operator.run/phase: tesxx
+property: pkg-name`
	require.Equal(t, expectedErr, err.Error())
}

func Test_generateStaticImages(t *testing.T) {
	t.Parallel()
	manifest := &manifests.PackageManifest{
		Spec: manifests.PackageManifestSpec{
			Images: []manifests.PackageManifestImage{
				{
					Name:  "t1",
					Image: "quay.io/example/example",
				},
				{
					Name:  "t2",
					Image: "quay.io/example/example",
				},
			},
		},
	}
	staticImages := generateStaticImages(manifest)
	assert.Equal(t, map[string]string{
		"t1": staticImage,
		"t2": staticImage,
	}, staticImages)
}

func Test_renderTemplateFiles(t *testing.T) {
	t.Parallel()
	path := t.TempDir()
	defer func() {
		require.NoError(t, os.RemoveAll(path))
	}()

	files := packagetypes.Files{
		"test.yaml":  []byte("test: xxx\n---\ntest: yyy\n---\ntest: zzz\n"),
		"test2.yaml": []byte("test: xxx"),
		"test3.yaml": []byte("test: xxx"),
	}
	filteredIndexMap := map[string][]int{
		"test.yaml":  {1},
		"test2.yaml": {0},
		"test3.yaml": nil,
	}

	err := renderTemplateFiles(path, files, filteredIndexMap)
	require.NoError(t, err)

	testContent, err := os.ReadFile(filepath.Join(path, "test.yaml"))
	require.NoError(t, err)
	assert.Equal(t, "test: xxx\n---\ntest: zzz\n", string(testContent))

	_, err = os.ReadFile(filepath.Join(path, "test2.yaml"))
	require.True(t, os.IsNotExist(err), "test2.yaml does not exist")

	_, err = os.ReadFile(filepath.Join(path, "test3.yaml"))
	require.True(t, os.IsNotExist(err), "test3.yaml does not exist")
}
