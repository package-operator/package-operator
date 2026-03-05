package kickstart

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/go-containerregistry/pkg/crane"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"pkg.package-operator.run/cardboard/kubeutils/kubemanifests"

	"package-operator.run/internal/packages"
)

var errPackageFolderExists = errors.New("package folder already exists")

type Kickstarter struct {
	stdin  io.Reader
	client *http.Client
}

func NewKickstarter(stdin io.Reader) *Kickstarter {
	t := &http.Transport{
		Dial: (&net.Dialer{
			Timeout: 5 * time.Second,
		}).Dial,
		TLSHandshakeTimeout: 5 * time.Second,
	}
	client := &http.Client{
		Timeout:   time.Second * 30,
		Transport: t,
	}

	return &Kickstarter{
		stdin:  stdin,
		client: client,
	}
}

// Runs kickstart processing on given inputs and returns a user message on success.
func (k *Kickstarter) Kickstart(
	ctx context.Context,
	pkgName string,
	inputs []string,
	olmBundle string,
	paramOpts []string,
) (string, error) {
	folderName := pkgName
	// Preflight check: Check if pkgName folder already exists.
	if _, err := os.Stat(folderName); err != nil {
		// If the error is "not exist" then we're fine.
		if !os.IsNotExist(err) {
			return "", fmt.Errorf("preflight check: %w", err)
		}
	} else {
		// Stat was successful and thus something already exists at `pkgName`.
		return "", fmt.Errorf("%w: %s", errPackageFolderExists, pkgName)
	}

	var objects []unstructured.Unstructured
	// Imports from Inputs.
	for _, input := range inputs {
		newObjects, err := k.getInput(ctx, input)
		if err != nil {
			return "", fmt.Errorf("get input: %w", err)
		}

		objects = append(objects, newObjects...)
	}

	// Import from OLM bundle.
	if len(olmBundle) > 0 {
		img, err := crane.Pull(olmBundle)
		if err != nil {
			return "", err
		}

		objs, reg, err := packages.ImportOLMBundleImage(ctx, img)
		if err != nil {
			return "", fmt.Errorf("import olm bundle: %w", err)
		}
		objects = append(objects, objs...)
		// Take package name from OLM Bundle.
		pkgName = reg.PackageName
	}

	rawPkg, res, err := packages.Kickstart(ctx, pkgName, objects, paramOpts)
	if err != nil {
		return "", err
	}

	// Write files.
	if err := os.Mkdir(folderName, os.ModePerm); err != nil {
		return "", fmt.Errorf("create directory: %w", err)
	}
	for path, data := range rawPkg.Files {
		path = filepath.Join(folderName, path)
		if err := os.MkdirAll(filepath.Dir(path), os.ModePerm); err != nil {
			return "", fmt.Errorf("creating directory: %w", err)
		}
		if err := os.WriteFile(path, data, os.ModePerm); err != nil {
			return "", fmt.Errorf("writing file: %w", err)
		}
	}

	msg := fmt.Sprintf("Kickstarted the %q package with %d objects.", pkgName, res.ObjectCount)
	report, ok := reportGKsWithoutProbes(res.GroupKindsWithoutProbes)
	if ok {
		msg += "\n" + report
	}
	return msg, nil
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
		resp, err := k.client.Do(req) //nolint:gosec // G704: input validated as http(s) URL
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
	// Pattern directly matched a file or folder and was not expanded.
	return []string{pattern}, nil
}

func reportGKsWithoutProbes(gksWithoutProbes []schema.GroupKind) (r string, ok bool) {
	var report strings.Builder
	report.WriteString("[WARN] Some kinds don't have availability probes defined:\n")
	for _, gk := range gksWithoutProbes {
		fmt.Fprintf(&report, "- %s\n", gk.String())
		ok = true
	}
	return report.String(), ok
}
