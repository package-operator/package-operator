package packagevalidation

import (
	"context"
	"os"
	"testing"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

	ctx := logr.NewContext(context.Background(), testr.New(t))
	ttv := NewTemplateTestValidator(validatorPath)

	originalFileMap := packagetypes.Files{
		"file2.yaml.gotmpl": []byte(testFile2Content), "file.yaml.gotmpl": []byte(testFile1Content),
	}
	originalPkg := &packagetypes.Package{
		Manifest: packageManifest,
		Files:    originalFileMap,
	}
	err := ttv.ValidatePackage(ctx, originalPkg)
	require.NoError(t, err)

	// Assert Fixtures have been setup
	newFileMap := packagetypes.Files{
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
	newPkg := &packagetypes.Package{
		Manifest: packageManifest,
		Files:    newFileMap,
	}
	err = ttv.ValidatePackage(ctx, newPkg)
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
