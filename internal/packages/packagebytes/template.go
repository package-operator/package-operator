package packagebytes

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/go-logr/logr"

	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
	"package-operator.run/package-operator/internal/packages"
	"package-operator.run/package-operator/internal/utils"
)

// Runs a go-template transformer on all .yml or .yaml files.
type TemplateTransformer struct {
	TemplateContext manifestsv1alpha1.TemplateContext
}

func (t *TemplateTransformer) Transform(ctx context.Context, fileMap FileMap) error {
	for path, content := range fileMap {
		var err error
		content, err = t.transform(ctx, path, content)
		if err != nil {
			return err
		}
		// save back to file map without the template suffix
		fileMap[stripTemplateSuffix(path)] = content
	}

	return nil
}

func (t *TemplateTransformer) transform(ctx context.Context, path string, content []byte) ([]byte, error) {
	if !packages.IsTemplateFile(path) {
		// Not a template file, skip.
		return content, nil
	}

	template, err := template.New("").Parse(string(content))
	if err != nil {
		return nil, fmt.Errorf(
			"parsing template from %s: %w", path, err)
	}

	var doc bytes.Buffer
	if err := template.Execute(&doc, t.TemplateContext); err != nil {
		return nil, fmt.Errorf(
			"executing template from %s: %w", path, err)
	}
	return doc.Bytes(), nil
}

func stripTemplateSuffix(path string) string {
	return strings.TrimSuffix(path, packages.TemplateFileSuffix)
}

type packageContentLoader func(ctx context.Context, fileMap FileMap) error

type packageManifestLoader interface {
	FromFileMap(ctx context.Context, fileMap FileMap) (
		*manifestsv1alpha1.PackageManifest, error,
	)
}

type TemplateTestValidator struct {
	// Path to a folder containing the test fixtures for the package.
	fixturesFolderPath string

	packageContentLoader  packageContentLoader
	packageManifestLoader packageManifestLoader
}

func NewTemplateTestValidator(
	fixturesFolderPath string,
	packageContentLoader packageContentLoader,
	packageManifestLoader packageManifestLoader,
) *TemplateTestValidator {
	return &TemplateTestValidator{
		fixturesFolderPath:    fixturesFolderPath,
		packageContentLoader:  packageContentLoader,
		packageManifestLoader: packageManifestLoader,
	}
}

func (v TemplateTestValidator) Validate(ctx context.Context, fileMap FileMap) error {
	log := logr.FromContextOrDiscard(ctx).V(1)

	// First get the PackageManifest
	pm, err := v.packageManifestLoader.FromFileMap(ctx, fileMap)
	if err != nil {
		return err
	}

	for _, templateTestCase := range pm.Test.Template {
		log.Info("running template test case", "name", templateTestCase.Name)
		if err := v.runTestCase(ctx, fileMap, templateTestCase); err != nil {
			return err
		}
	}

	return nil
}

func (v TemplateTestValidator) runTestCase(
	ctx context.Context, fileMap FileMap,
	testCase manifestsv1alpha1.PackageManifestTestCaseTemplate,
) error {
	log := logr.FromContextOrDiscard(ctx)
	fileMap = utils.CopyMap(fileMap)

	tt := TemplateTransformer{
		TemplateContext: testCase.Context,
	}
	if err := tt.Transform(ctx, fileMap); err != nil {
		return err
	}

	// Load package contents for structural validation.
	if err := v.packageContentLoader(ctx, fileMap); err != nil {
		return err
	}

	// check if test figures exist
	testFixturePath := filepath.Join(v.fixturesFolderPath, testCase.Name)
	_, err := os.Stat(testFixturePath)
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
		path := stripTemplateSuffix(relPath)

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

func renderTemplateFiles(folder string, fileMap FileMap) error {
	for relPath := range fileMap {
		if !packages.IsTemplateFile(relPath) {
			// template source files are of no interest for the test fixtures.
			continue
		}
		path := stripTemplateSuffix(relPath)
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
