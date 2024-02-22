package packagevalidation

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-logr/logr"
	"github.com/hexops/gotextdiff"
	"github.com/hexops/gotextdiff/myers"
	"github.com/hexops/gotextdiff/span"

	"package-operator.run/internal/apis/manifests"
	"package-operator.run/internal/packages/internal/packagemanifestvalidation"
	"package-operator.run/internal/packages/internal/packagerender"
	"package-operator.run/internal/packages/internal/packagetypes"
)

// Creates or updates template test fixtures.
type TemplateTestRenderer struct {
	// Path to the folder containing the package.
	packagePath string
}

// Creates a new TemplateTestRenderer instance.
func NewTemplateTestRenderer(packagePath string) *TemplateTestRenderer {
	return &TemplateTestRenderer{
		packagePath: packagePath,
	}
}

func (r TemplateTestRenderer) UpsertFixtures(
	ctx context.Context, pkg *packagetypes.Package,
) error {
	pkg = pkg.DeepCopy()
	log := logr.FromContextOrDiscard(ctx).V(1)

	for _, templateTestCase := range pkg.Manifest.Test.Template {
		log.Info("generating template test case", "name", templateTestCase.Name)
		if err := r.renderTestFixtures(ctx, pkg, templateTestCase); err != nil {
			return err
		}
	}
	return nil
}

func (r TemplateTestRenderer) renderTestFixtures(
	ctx context.Context, pkg *packagetypes.Package,
	testCase manifests.PackageManifestTestCaseTemplate,
) error {
	_, err := renderTemplates(ctx, pkg, testCase)
	if err != nil {
		return err
	}
	return writeRenderedTemplateFiles(
		filepath.Join(r.packagePath, packagetypes.PackageTestFixturesFolder, testCase.Name),
		pkg.Files,
	)
}

// Runs the template test suites.
type TemplateTestValidator struct{}

func (v TemplateTestValidator) ValidatePackage(
	ctx context.Context, pkg *packagetypes.Package,
) error {
	log := logr.FromContextOrDiscard(ctx).V(1)

	kcV, err := kubeconformValidatorFromManifest(pkg.Manifest)
	if err != nil {
		return err
	}

	for _, templateTestCase := range pkg.Manifest.Test.Template {
		log.Info("running template test case", "name", templateTestCase.Name)
		if err := v.runTestCase(ctx, pkg, templateTestCase, kcV); err != nil {
			return err
		}
	}

	for path, file := range pkg.Files {
		if verrs, err := runKubeconformForFile(path, file, kcV); err != nil {
			return err
		} else if len(verrs) > 0 {
			return errors.Join(verrs...)
		}
	}

	return nil
}

func renderTemplates(
	ctx context.Context,
	pkg *packagetypes.Package,
	testCase manifests.PackageManifestTestCaseTemplate,
) (tmplCtx packagetypes.PackageRenderContext, err error) {
	configuration := map[string]any{}
	if testCase.Context.Config != nil {
		if err := json.Unmarshal(testCase.Context.Config.Raw, &configuration); err != nil {
			return tmplCtx, err
		}
	}

	if _, err := packagemanifestvalidation.AdmitPackageConfiguration(ctx, configuration, pkg.Manifest, nil); err != nil {
		return tmplCtx, err
	}

	tmplCtx = packagetypes.PackageRenderContext{
		Package:     testCase.Context.Package,
		Config:      configuration,
		Images:      generateStaticImages(pkg.Manifest),
		Environment: testCase.Context.Environment,
	}
	if err := packagerender.RenderTemplates(ctx, pkg, tmplCtx); err != nil {
		return tmplCtx, err
	}
	return tmplCtx, nil
}

func (v TemplateTestValidator) runTestCase(
	ctx context.Context, pkg *packagetypes.Package,
	testCase manifests.PackageManifestTestCaseTemplate,
	kcV kubeconformValidator,
) (rErr error) {
	pkg = pkg.DeepCopy()

	// check if test figures exist
	if !hasTestFixtures(pkg.Files, testCase) {
		// no fixtures generated
		return packagetypes.ViolationError{
			Reason:  packagetypes.ViolationReasonTestFixturesForTestCaseMissing,
			Details: fmt.Sprintf("test case %q", testCase.Name),
		}
	}

	tmplCtx, err := renderTemplates(ctx, pkg, testCase)
	if err != nil {
		return err
	}
	if _, err = packagerender.RenderObjects(ctx, pkg, tmplCtx, DefaultObjectValidators); err != nil {
		return err
	}

	violations := make([]error, 0, len(pkg.Files))
	for relPath := range pkg.Files {
		if !packagetypes.IsTemplateFile(relPath) {
			// template source files are of no interest for the test fixtures.
			continue
		}
		if strings.HasPrefix(relPath, packagetypes.PackageTestFixturesFolder) {
			// skip test-fixtures.
			continue
		}
		path := packagetypes.StripTemplateSuffix(relPath)

		verrs, err := runKubeconformForFile(path, pkg.Files[path], kcV)
		if err != nil {
			return err
		}
		violations = append(violations, verrs...)

		testFixturePath := testFixtureForPath(path, testCase)
		actual := pkg.Files[path]
		fixture, fixtureOk := pkg.Files[testFixtureForPath(path, testCase)]
		if !fixtureOk && len(bytes.TrimSpace(actual)) == 0 {
			// No fixture exist and actual file is empty.
			// -> content probably excluded via template IF condition.
			continue
		}
		diff, err := runDiff(fixture, "FIXTURE/"+testFixturePath, actual, "ACTUAL/"+path)
		if err != nil {
			return err
		}
		if len(diff) == 0 {
			continue
		}

		violations = append(violations, packagetypes.ViolationError{
			Reason:  packagetypes.ViolationReasonFixtureMismatch,
			Details: fmt.Sprintf("Testcase %q\n%s", testCase.Name, diff),
			Path:    relPath,
		})
	}

	return errors.Join(violations...)
}

func testFixtureForPath(path string, testCase manifests.PackageManifestTestCaseTemplate) string {
	return filepath.Join(packagetypes.PackageTestFixturesFolder, testCase.Name, path)
}

func hasTestFixtures(files packagetypes.Files, testCase manifests.PackageManifestTestCaseTemplate) bool {
	prefix := filepath.Join(packagetypes.PackageTestFixturesFolder, testCase.Name)
	for path := range files {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

func writeRenderedTemplateFiles(folder string, fileMap packagetypes.Files) error {
	for relPath := range fileMap {
		if !packagetypes.IsTemplateFile(relPath) {
			// template source files are of no interest for the test fixtures.
			continue
		}

		path := packagetypes.StripTemplateSuffix(relPath)
		content := fileMap[path]
		if len(bytes.TrimSpace(content)) == 0 {
			// don't process empty files
			continue
		}

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

func runDiff(fileAContent []byte, labelA string, fileBContent []byte, labelB string) (string, error) {
	edits := myers.ComputeEdits(span.URIFromPath(labelA), string(fileAContent), string(fileBContent))
	return strings.TrimSpace(fmt.Sprint(gotextdiff.ToUnified(labelA, labelB, string(fileAContent), edits))), nil
}

const staticImage = "registry.package-operator.run/static-image"

// generateStaticImages generates a static set of images to be used for tests and other purposes.
func generateStaticImages(manifest *manifests.PackageManifest) map[string]string {
	images := map[string]string{}
	for _, v := range manifest.Spec.Images {
		images[v.Name] = staticImage
	}
	return images
}
