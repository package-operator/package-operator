package kickstart

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"pkg.package-operator.run/cardboard/kubeutils/kubemanifests"
	"sigs.k8s.io/yaml"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
)

type Kickstarter struct {
	stdin io.Reader
}

func NewKickstarter(stdin io.Reader) *Kickstarter {
	return &Kickstarter{
		stdin: stdin,
	}
}

func (k *Kickstarter) KickStart(
	ctx context.Context,
	pkgName string,
	inputs []string,
) (msg string, err error) {
	if err := os.Mkdir(pkgName, os.ModePerm); err != nil {
		return "", fmt.Errorf("create directory: %w", err)
	}

	usedPhases := map[string]struct{}{}
	usedGKs := map[schema.GroupKind]struct{}{}
	var objCount int
	for _, input := range inputs {
		objects, err := k.getInput(ctx, input)
		if err != nil {
			return "", fmt.Errorf("get input: %w", err)
		}

		for _, obj := range objects {
			phase, gk, err := k.processObject(pkgName, obj)
			if err != nil {
				return "", fmt.Errorf("processing object: %w", err)
			}
			usedPhases[phase] = struct{}{}
			usedGKs[gk] = struct{}{}
		}
		objCount += len(objects)
	}

	// Write Manifest
	var phases []manifestsv1alpha1.PackageManifestPhase
	for _, phase := range orderedPhases {
		if _, ok := usedPhases[string(phase)]; ok {
			phases = append(phases, manifestsv1alpha1.PackageManifestPhase{Name: string(phase)})
		}
	}
	//nolint:prealloc
	var (
		probes           []corev1alpha1.ObjectSetProbe
		gksWithoutProbes = map[schema.GroupKind]struct{}{}
	)
	for gk := range usedGKs {
		probe, ok := getProbe(gk)
		if !ok {
			gksWithoutProbes[gk] = struct{}{}
			continue
		}
		probes = append(probes, probe)
	}
	manifest := &manifestsv1alpha1.PackageManifest{
		TypeMeta: metav1.TypeMeta{
			Kind:       "PackageManifest",
			APIVersion: "manifests.package-operator.run/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: pkgName,
		},
		Spec: manifestsv1alpha1.PackageManifestSpec{
			Phases: phases,
			Scopes: []manifestsv1alpha1.PackageManifestScope{
				manifestsv1alpha1.PackageManifestScopeCluster,
				manifestsv1alpha1.PackageManifestScopeNamespaced,
			},
			AvailabilityProbes: probes,
		},
	}
	b, err := yaml.Marshal(manifest)
	if err != nil {
		return "", fmt.Errorf("marshalling PackageManifest YAML: %w", err)
	}
	if err := os.WriteFile(filepath.Join(pkgName, "manifest.yaml"), b, os.ModePerm); err != nil {
		return "", fmt.Errorf("writing PackageManifest: %w", err)
	}

	msg = fmt.Sprintf("Kickstarted the %q package with %d objects.", pkgName, objCount)
	report, ok := reportGKsWithoutProbes(gksWithoutProbes)
	if ok {
		msg += "\n" + report
	}

	return msg, nil
}

func (k *Kickstarter) processObject(
	pkgName string, obj unstructured.Unstructured,
) (phase string, gk schema.GroupKind, err error) {
	annotations := obj.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	gk = obj.GroupVersionKind().GroupKind()
	phase = guessPresetPhase(gk)
	annotations[manifestsv1alpha1.PackagePhaseAnnotation] = phase
	obj.SetAnnotations(annotations)

	path := filepath.Join(pkgName, phase,
		fmt.Sprintf("%s.%s.yaml", obj.GetName(), strings.ToLower(obj.GetKind())))
	b, err := yaml.Marshal(obj.Object)
	if err != nil {
		return phase, gk, fmt.Errorf("marshalling YAML: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), os.ModePerm); err != nil {
		return phase, gk, fmt.Errorf("creating directory: %w", err)
	}
	return phase, gk, os.WriteFile(path, b, os.ModePerm)
}

func (k *Kickstarter) getInput(ctx context.Context, input string) (
	[]unstructured.Unstructured, error,
) {
	var reader io.Reader
	switch {
	case input == "-":
		// from stdin
		reader = k.stdin

	case strings.Index(input, "http://") == 0 ||
		strings.Index(input, "https://") == 0:
		// from HTTP(S)

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, input, nil)
		if err != nil {
			return nil, fmt.Errorf("building HTTP request: %w", err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("HTTP get: %w", err)
		}
		defer func() {
			if err := resp.Body.Close(); err != nil {
				panic(err)
			}
		}()
		reader = resp.Body

	default:
		// Files or Folders
		matches, err := expandIfFilePattern(input)
		if err != nil {
			return nil, fmt.Errorf("expand pattern: %w", err)
		}

		var objects []unstructured.Unstructured
		for _, match := range matches {
			i, err := os.Stat(match)
			if err != nil {
				return nil, fmt.Errorf("accessing: %w", err)
			}
			var matchObjs []unstructured.Unstructured
			if i.IsDir() {
				matchObjs, err = kubemanifests.LoadKubernetesObjectsFromFolder(match)
			} else {
				matchObjs, err = kubemanifests.LoadKubernetesObjectsFromFile(match)
			}
			if err != nil {
				return nil, fmt.Errorf("loading kubernetes objects: %w", err)
			}
			objects = append(objects, matchObjs...)
		}
		return objects, nil
	}

	if reader == nil {
		return nil, nil
	}
	content, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("reading: %w", err)
	}
	return kubemanifests.LoadKubernetesObjectsFromBytes(content)
}

// expandIfFilePattern returns all the filenames that match the input pattern
// or the filename if it is a specific filename and not a pattern.
// If the input is a pattern and it yields no result it will result in an error.
func expandIfFilePattern(pattern string) ([]string, error) {
	if _, err := os.Stat(pattern); os.IsNotExist(err) {
		matches, err := filepath.Glob(pattern)
		if err == nil && len(matches) == 0 {
			return nil, fmt.Errorf("%s: %w", pattern, os.ErrNotExist)
		}
		if errors.Is(err, filepath.ErrBadPattern) {
			return nil, fmt.Errorf("pattern %q is not valid: %w", pattern, err)
		}
		return matches, err
	}
	return []string{pattern}, nil
}

func init() {
	for phase, gks := range phaseGKMap {
		for _, gk := range gks {
			gkPhaseMap[gk] = phase
		}
	}
}
