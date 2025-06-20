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
	"package-operator.run/internal/packages/internal/packageimport"
	"package-operator.run/internal/packages/internal/packagemanifestvalidation"
	"package-operator.run/internal/packages/internal/packagerender"
	"package-operator.run/internal/packages/internal/packagetypes"
)

const testFixturesFolderName = ".test-fixtures"

// Runs the template test suites.
type TemplateTestValidator struct {
	// Path to a folder containing the test fixtures for the package.
	packageBaseFolderPath string
}

// Creates a new TemplateTestValidator instance.
func NewTemplateTestValidator(
	packageBaseFolderPath string,
) *TemplateTestValidator {
	return &TemplateTestValidator{
		packageBaseFolderPath: packageBaseFolderPath,
	}
}

func (v TemplateTestValidator) ValidatePackage(
	ctx context.Context, pkg *packagetypes.Package,
) error {
	return packagetypes.ValidateEachComponent(ctx, pkg, v.doValidatePackage)
}

func (v TemplateTestValidator) doValidatePackage(
	ctx context.Context, pkg *packagetypes.Package, isComponent bool,
) error {
	log := logr.FromContextOrDiscard(ctx).V(1)

	kcV, err := kubeconformValidatorFromManifest(pkg.Manifest)
	if err != nil {
		return err
	}

	subDir := ""
	if isComponent {
		subDir = filepath.Join("components", pkg.Manifest.Name)
	}

	for _, templateTestCase := range pkg.Manifest.Test.Template {
		log.Info("running template test case", "name", templateTestCase.Name)
		if err := v.runTestCase(ctx, pkg, templateTestCase, kcV, subDir); err != nil {
			return err
		}
	}

	for path, file := range pkg.Files {
		if verrs := runKubeconformForFile(path, file, kcV); len(verrs) > 0 {
			return errors.Join(verrs...)
		}
	}

	return nil
}

func (v TemplateTestValidator) runTestCase(
	ctx context.Context, pkg *packagetypes.Package,
	testCase manifests.PackageManifestTestCaseTemplate,
	kcV kubeconformValidator,
	subDir string,
) (rErr error) {
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
	pathObjects, pathFilteredIndex, err := packagerender.RenderObjectsWithFilterInfo(
		ctx, pkg, tmplCtx, DefaultObjectValidators)
	if err != nil {
		return err
	}

	// check if test figures exist
	testFixturePath := filepath.Join(
		v.packageBaseFolderPath, subDir,
		testFixturesFolderName, testCase.Name)
	_, err = os.Stat(testFixturePath)
	if errors.Is(err, os.ErrNotExist) {
		// no fixtures generated
		// generate fixtures now
		log.Info("no fixture found for test case, generating...", "name", testCase.Name)
		return renderTemplateFiles(testFixturePath, pkg.Files, pathFilteredIndex)
	}

	actualPath, err := os.MkdirTemp(os.TempDir(), "pko-test-"+testCase.Name+"-")
	if err != nil {
		return err
	}
	defer func() {
		if cErr := os.RemoveAll(actualPath); rErr == nil && cErr != nil {
			rErr = cErr
		}
	}()

	if err := renderTemplateFiles(actualPath, pkg.Files, pathFilteredIndex); err != nil {
		return err
	}

	violations := make([]error, 0, len(pkg.Files))

	// check for unknown files
	fixturesFiles, err := packageimport.Index(testFixturePath)
	if err != nil {
		return err
	}
	for fixturesPath := range fixturesFiles {
		if _, ok := pathObjects[fixturesPath]; !ok {
			violations = append(violations, &unknownFileFoundInFixturesFolderError{file: fixturesPath})
		}
	}

	for path := range pathObjects {
		if packagetypes.IsTemplateFile(path) {
			// template source files are of no interest for the test fixtures.
			continue
		}

		verrs := runKubeconformForFile(path, pkg.Files[path], kcV)
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
			Path:    path,
		})
	}

	return errors.Join(violations...)
}

func renderTemplateFiles(
	folder string,
	fileMap packagetypes.Files,
	pathFilteredIndex map[string][]int,
) error {
	for path := range fileMap {
		if packagetypes.IsTemplateFile(path) ||
			!packagetypes.IsYAMLFile(path) {
			// template source files are of no interest for the test fixtures.
			continue
		}

		content := fileMap[path]
		if filteredIndexes, ok := pathFilteredIndex[path]; ok && filteredIndexes == nil {
			// all documents where filtered at the given path
			content = nil
		} else if ok {
			// some documents where filtered at the given path.
			indexMap := map[int]struct{}{}
			for _, i := range filteredIndexes {
				indexMap[i] = struct{}{}
			}
			var documents [][]byte
			for i, doc := range packagetypes.SplitYAMLDocuments(content) {
				if _, ok := indexMap[i]; ok {
					continue
				}
				documents = append(documents, doc)
			}
			content = packagetypes.JoinYAMLDocuments(documents)
		}
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

type unknownFileFoundInFixturesFolderError struct {
	file string
}

func (e *unknownFileFoundInFixturesFolderError) Error() string {
	return fmt.Sprintf("file %s should not exist, filtered or empty after template render", e.file)
}

type fileNotFoundInFixturesFolderError struct {
	file string
}

func (e *fileNotFoundInFixturesFolderError) Error() string {
	return fmt.Sprintf("file %s not found in fixtures folder", e.file)
}

func runDiff(fileA, labelA, fileB, labelB string) ([]byte, error) {
	_, fileAStatErr := os.Stat(fileA)
	_, fileBStatErr := os.Stat(fileB)
	if os.IsNotExist(fileAStatErr) && os.IsNotExist(fileBStatErr) {
		return nil, nil
	}
	if os.IsNotExist(fileAStatErr) && fileBStatErr == nil {
		return nil, &fileNotFoundInFixturesFolderError{file: fileA}
	}

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
