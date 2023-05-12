/* #nosec */

package kubectlpackage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/registry"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"

	"package-operator.run/package-operator/internal/packages/packageexport"
	"package-operator.run/package-operator/internal/packages/packageimport"
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

	Expect(loadPackageImages(
		context.Background(),
		packageImageBuildInfo{
			Path: sourcePathFixture("valid_without_config"),
			Ref:  path.Join(_registryDomain, "valid-package-fixture"),
		},
		packageImageBuildInfo{
			Path: sourcePathFixture("invalid_bad_manifest"),
			Ref:  path.Join(_registryDomain, "invalid-package-fixture"),
		},
	)).To(Succeed())
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

func loadPackageImages(ctx context.Context, infos ...packageImageBuildInfo) error {
	for _, info := range infos {
		files, err := packageimport.FS(ctx, os.DirFS(info.Path))
		if err != nil {
			return fmt.Errorf("importing package from directory: %w", err)
		}

		tags := []string{info.Ref}

		if err := packageexport.PushedImage(ctx, tags, files, crane.Insecure); err != nil {
			return fmt.Errorf("pushing package image: %w", err)
		}
	}

	return nil
}

type packageImageBuildInfo struct {
	Path string
	Ref  string
}
