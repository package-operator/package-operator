package packagevalidation

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

	"package-operator.run/internal/apis/manifests"
	"package-operator.run/internal/packages/internal/packagemanifestvalidation"
	"package-operator.run/internal/packages/internal/packagerender"
	"package-operator.run/internal/packages/internal/packagetypes"
)

// Runs the template test suites.
type TemplateTestValidator struct {
	// Path to a folder containing the test fixtures for the package.
	fixturesFolderPath string
}

// Creates a new TemplateTestValidator instance.
func NewTemplateTestValidator(
	fixturesFolderPath string,
) *TemplateTestValidator {
	return &TemplateTestValidator{
		fixturesFolderPath: fixturesFolderPath,
	}
}

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

func (v TemplateTestValidator) runTestCase(
	ctx context.Context, pkg *packagetypes.Package,
	testCase manifests.PackageManifestTestCaseTemplate,
	kcV kubeconformValidator,
) error {
	log := logr.FromContextOrDiscard(ctx)
	pkg = pkg.DeepCopy()

	configuration := map[string]any{}
	if testCase.Context.Config != nil {
		if err := json.Unmarshal(testCase.Context.Config.Raw, &configuration); err != nil {
			return err
		}
	}

	if _, err := packagemanifestvalidation.AdmitPackageConfiguration(ctx, configuration, pkg.Manifest, nil); err != nil {
		return err
	}

	tmplCtx := packagetypes.PackageRenderContext{
		Package:     testCase.Context.Package,
		Config:      configuration,
		Images:      generateStaticImages(pkg.Manifest),
		Environment: testCase.Context.Environment,
	}
	if err := packagerender.RenderTemplates(ctx, pkg, tmplCtx); err != nil {
		return err
	}
	_, err := packagerender.RenderObjects(ctx, pkg, tmplCtx, DefaultObjectValidators)
	if err != nil {
		return err
	}

	// check if test figures exist
	testFixturePath := filepath.Join(v.fixturesFolderPath, testCase.Name)
	_, err = os.Stat(testFixturePath)
	if errors.Is(err, os.ErrNotExist) {
		// no fixtures generated
		// generate fixtures now
		log.Info("no fixture found for test case, generating...", "name", testCase.Name)
		return renderTemplateFiles(testFixturePath, pkg.Files)
	}

	actualPath, err := os.MkdirTemp(
		os.TempDir(), "pko-test-"+testCase.Name+"-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(actualPath)

	if err := renderTemplateFiles(actualPath, pkg.Files); err != nil {
		return err
	}

	violations := make([]error, 0, len(pkg.Files))
	for relPath := range pkg.Files {
		if !packagetypes.IsTemplateFile(relPath) {
			// template source files are of no interest for the test fixtures.
			continue
		}
		path := packagetypes.StripTemplateSuffix(relPath)

		verrs, err := runKubeconformForFile(path, pkg.Files[path], kcV)
		if err != nil {
			return err
		}
		violations = append(violations, verrs...)

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

		violations = append(violations, packagetypes.ViolationError{
			Reason:  packagetypes.ViolationReasonFixtureMismatch,
			Details: fmt.Sprintf("Testcase %q\n%s", testCase.Name, string(diff)),
			Path:    relPath,
		})
	}

	return errors.Join(violations...)
}

func renderTemplateFiles(folder string, fileMap packagetypes.Files) error {
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

func runDiff(fileA, labelA, fileB, labelB string) ([]byte, error) {
	_, fileAStatErr := os.Stat(fileA)
	_, fileBStatErr := os.Stat(fileB)
	if os.IsNotExist(fileAStatErr) && os.IsNotExist(fileBStatErr) {
		return nil, nil
	}

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

const staticImage = "registry.package-operator.run/static-image"

// generateStaticImages generates a static set of images to be used for tests and other purposes.
func generateStaticImages(manifest *manifests.PackageManifest) map[string]string {
	images := map[string]string{}
	for _, v := range manifest.Spec.Images {
		images[v.Name] = staticImage
	}
	return images
}
