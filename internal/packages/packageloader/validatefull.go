package packageloader

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"

	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
	"package-operator.run/package-operator/internal/packages"
	"package-operator.run/package-operator/internal/packages/packageadmission"
	"package-operator.run/package-operator/internal/packages/packagecontent"
	"package-operator.run/package-operator/internal/utils"
)

type TemplateTestValidator struct {
	scheme *runtime.Scheme
	// Path to a folder containing the test fixtures for the package.
	fixturesFolderPath string
}

var _ PackageAndFilesValidator = (*TemplateTestValidator)(nil)

func NewTemplateTestValidator(
	scheme *runtime.Scheme, fixturesFolderPath string,
) *TemplateTestValidator {
	return &TemplateTestValidator{
		scheme:             scheme,
		fixturesFolderPath: fixturesFolderPath,
	}
}

func (v TemplateTestValidator) ValidatePackageAndFiles(
	ctx context.Context, pkg *packagecontent.Package, fileMap packagecontent.Files) error {
	log := logr.FromContextOrDiscard(ctx).V(1)

	for _, templateTestCase := range pkg.PackageManifest.Test.Template {
		log.Info("running template test case", "name", templateTestCase.Name)
		if err := v.runTestCase(ctx, fileMap, pkg.PackageManifest, templateTestCase); err != nil {
			return err
		}
	}

	return nil
}

func (v TemplateTestValidator) runTestCase(
	ctx context.Context, fileMap packagecontent.Files,
	manifest *manifestsv1alpha1.PackageManifest,
	testCase manifestsv1alpha1.PackageManifestTestCaseTemplate,
) error {
	log := logr.FromContextOrDiscard(ctx)
	fileMap = utils.CopyMap(fileMap)

	configuration := map[string]interface{}{}
	if testCase.Context.Config != nil {
		if err := json.Unmarshal(testCase.Context.Config.Raw, &configuration); err != nil {
			return err
		}
	}

	if _, err := packageadmission.AdmitPackageConfiguration(ctx, v.scheme, configuration, manifest, nil); err != nil {
		return err
	}

	tt, err := NewTemplateTransformer(TemplateContext{
		Package: testCase.Context.Package,
		Config:  configuration,
	})
	if err != nil {
		return err
	}

	if err := tt.TransformPackageFiles(ctx, fileMap); err != nil {
		return err
	}

	// check if test figures exist
	testFixturePath := filepath.Join(v.fixturesFolderPath, testCase.Name)
	_, err = os.Stat(testFixturePath)
	if errors.Is(err, os.ErrNotExist) {
		// no fixtures generated
		// generate fixtures now
		log.Info("no fixture found for test case, generating...", "name", testCase.Name)
		if err := renderTemplateFiles(testFixturePath, fileMap); err != nil {
			return err
		}
		return nil
	}

	actualPath, err := os.MkdirTemp(
		os.TempDir(), "pko-test-"+testCase.Name+"-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(actualPath)

	if err := renderTemplateFiles(actualPath, fileMap); err != nil {
		return err
	}

	var violations []packages.Violation
	for relPath := range fileMap {
		if !packages.IsTemplateFile(relPath) {
			// template source files are of no interest for the test fixtures.
			continue
		}
		path := packages.StripTemplateSuffix(relPath)

		fixtureFilePath := filepath.Join(testFixturePath, path)
		actualFilePath := filepath.Join(actualPath, path)

		file := filepath.Base(fixtureFilePath)
		diff, err := runDiff(fixtureFilePath, "FIXTURE/"+file, actualFilePath, "ACTUAL/"+file)
		if err != nil {
			return err
		}
		if len(diff) == 0 {
			continue
		}

		violations = append(violations, packages.Violation{
			Reason:  fmt.Sprintf("Test %q: %s", testCase.Name, packages.ViolationReasonFixtureMismatch),
			Details: string(diff),
			Location: &packages.ViolationLocation{
				Path: relPath,
			},
		})
	}

	if len(violations) > 0 {
		return packages.NewInvalidError(violations...)
	}
	return nil
}

func renderTemplateFiles(folder string, fileMap packagecontent.Files) error {
	for relPath := range fileMap {
		if !packages.IsTemplateFile(relPath) {
			// template source files are of no interest for the test fixtures.
			continue
		}
		path := packages.StripTemplateSuffix(relPath)
		content := fileMap[path]

		absPath := filepath.Join(folder, path)
		if err := os.MkdirAll(filepath.Dir(absPath), os.ModePerm); err != nil {
			return err
		}
		if err := os.WriteFile(absPath, content, os.ModePerm); err != nil {
			return err
		}
	}
	return nil
}

func runDiff(fileA, labelA, fileB, labelB string) ([]byte, error) {
	//nolint:gosec
	data, err := exec.
		Command("diff", "-u", "--label="+labelA, "--label="+labelB, fileA, fileB).
		CombinedOutput()
	if len(data) > 0 {
		// diff exits with a non-zero status when the files don't match.
		// Ignore that failure as long as we get output.
		err = nil
	}

	data = bytes.TrimSpace(data)
	return data, err
}
