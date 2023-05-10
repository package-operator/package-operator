/* #nosec */

package kubectlpackage

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/registry"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

func TestKubectlPackage(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "kubectl-package Suite")
}

var (
	_root           string
	_pluginPath     string
	_registryDomain string
	_registryClient *http.Client
)

const outputPath = "output"

var _ = BeforeSuite(func() {
	var err error

	_root, err = projectRoot()
	Expect(err).ToNot(
		HaveOccurred(),
		"Looking up project root.",
	)

	_pluginPath, err = buildPluginBinary()
	Expect(err).ToNot(
		HaveOccurred(),
		"Unable to build plug-in binary.",
	)

	DeferCleanup(gexec.CleanupBuildArtifacts)

	Expect(os.Mkdir(outputPath, 0o755)).Error().ToNot(HaveOccurred())

	DeferCleanup(func() error {
		return os.RemoveAll(outputPath)
	})

	srv, err := registry.TLS("example.com")
	Expect(err).ToNot(HaveOccurred())

	DeferCleanup(srv.Close)

	_registryDomain = strings.TrimPrefix(srv.URL, "https://")
	_registryClient = srv.Client()

	imagesDir := filepath.Join("testdata", "images")

	ents, err := fs.ReadDir(os.DirFS(imagesDir), ".")
	Expect(err).ToNot(HaveOccurred())

	for _, e := range ents {
		if filepath.Ext(e.Name()) != ".tar" {
			continue
		}

		pushImageFromDisk(
			filepath.Join(imagesDir, e.Name()),
			fmt.Sprintf("%s/%s-fixture", _registryDomain, strings.TrimSuffix(e.Name(), ".tar")))
	}
})

var errSetup = errors.New("test setup failed")

func buildPluginBinary() (string, error) {
	ldflags := strings.Join([]string{
		"-w", "-s",
		"--extldflags", "'-zrelro -znow -O1'",
		"-X", fmt.Sprintf("'%s/internal/version.version=%s'", module, version),
	}, " ")
	args := []string{
		"-cover",
		"--ldflags", ldflags,
	}

	return gexec.Build(filepath.Join(_root, "cmd", "kubectl-package"), args...)
}

const (
	module  = "package-operator.run/package-operator"
	version = "v0.0.0"
)

func projectRoot() (string, error) {
	var buf bytes.Buffer

	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Stdout = &buf
	cmd.Stderr = io.Discard

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("determining top level directory from git: %w", errSetup)
	}

	return strings.TrimSpace(buf.String()), nil
}
