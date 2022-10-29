package packages

import (
	"bytes"
	"fmt"
	"html/template"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
)

type folderLoadOperation struct {
	log             logr.Logger
	rootPath        string
	manifest        *manifestsv1alpha1.PackageManifest
	templateContext FolderLoaderTemplateContext
	objectsByPhase  map[string][]unstructured.Unstructured
}

func newFolderLoadOperation(
	log logr.Logger, rootPath string,
	manifest *manifestsv1alpha1.PackageManifest,
	templateContext FolderLoaderTemplateContext,
) *folderLoadOperation {
	return &folderLoadOperation{
		manifest:        manifest,
		log:             log,
		rootPath:        rootPath,
		templateContext: templateContext,
		objectsByPhase:  map[string][]unstructured.Unstructured{},
	}
}

func (l *folderLoadOperation) Load() error {
	if err := filepath.WalkDir(l.rootPath, l.walkPackageFolder); err != nil {
		return err
	}
	return nil
}

func (l *folderLoadOperation) walkPackageFolder(path string, d fs.DirEntry, err error) error {
	if err != nil {
		return err
	}

	if strings.HasPrefix(d.Name(), ".") {
		// skip dot-folders and dot-files.
		l.log.Info("skipping dot-file or dot-folder", "path", path)
		if d.IsDir() {
			// skip dir to prevent loading the directory content.
			return filepath.SkipDir
		}
		return nil
	}

	if d.IsDir() {
		// not interested in directories.
		return nil
	}

	switch filepath.Ext(d.Name()) {
	case ".yaml", ".yml":
	default:
		// skip non-yaml files
		l.log.Info("skipping non .yaml/.yml file", "path", path)
		return nil
	}

	objs, err := l.loadKubernetesObjectsFromFile(path)
	if err != nil {
		return fmt.Errorf("parsing yaml from %s: %w", path, err)
	}

	for i := range objs {
		obj := objs[i]

		if obj.GetObjectKind().GroupVersionKind().GroupKind() == packageManifestGroupKind {
			// skip PackageManifest objects
			continue
		}

		if obj.GetAnnotations() == nil ||
			len(obj.GetAnnotations()[manifestsv1alpha1.PackagePhaseAnnotation]) == 0 {
			return &PackageObjectInvalidError{
				FilePath: path,
				Reason:   fmt.Sprintf("missing %q annotation", manifestsv1alpha1.PackagePhaseAnnotation),
			}
		}

		obj.SetLabels(mergeKeysFrom(obj.GetLabels(), commonLabels(l.manifest, l.templateContext.Package.Name)))

		annotations := obj.GetAnnotations()
		phase := annotations[manifestsv1alpha1.PackagePhaseAnnotation]
		delete(annotations, manifestsv1alpha1.PackagePhaseAnnotation)
		obj.SetAnnotations(annotations)
		l.objectsByPhase[phase] = append(l.objectsByPhase[phase], obj)
	}

	return nil
}

// Loads kubernetes objects from the given file.
func (l *folderLoadOperation) loadKubernetesObjectsFromFile(filePath string) ([]unstructured.Unstructured, error) {
	fileYaml, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", filePath, err)
	}

	return l.loadKubernetesObjectsFromBytes(fileYaml)
}

// Loads kubernetes objects from given bytes.
// A single file may contain multiple objects separated by "---\n".
func (l *folderLoadOperation) loadKubernetesObjectsFromBytes(fileYaml []byte) ([]unstructured.Unstructured, error) {
	// Trim empty starting and ending objects
	fileYaml = bytes.Trim(fileYaml, "---\n")

	var objects []unstructured.Unstructured
	// Split for every included yaml document.
	for i, yamlDocument := range bytes.Split(fileYaml, []byte("---\n")) {
		// templating
		t, err := template.New(fmt.Sprintf("yaml#%d", i)).Parse(string(yamlDocument))
		if err != nil {
			return nil, fmt.Errorf(
				"parsing template from yaml document at index %d: %w", i, err)
		}

		var doc bytes.Buffer
		if err := t.Execute(&doc, l.templateContext); err != nil {
			return nil, fmt.Errorf(
				"executing template from yaml document at index %d: %w", i, err)
		}

		obj := unstructured.Unstructured{}
		if err := yaml.Unmarshal(doc.Bytes(), &obj); err != nil {
			return nil, fmt.Errorf(
				"unmarshalling yaml document at index %d: %w", i, err)
		}

		if len(obj.Object) == 0 {
			continue
		}

		objects = append(objects, obj)
	}

	return objects, nil
}
