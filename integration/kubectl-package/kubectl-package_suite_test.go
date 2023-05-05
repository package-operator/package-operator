/* #nosec */

package kubectlpackage

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

func TestKubectlPackage(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "kubectl-package Suite")
}

var _pluginPath string

const outputPath = "output"

var _ = BeforeSuite(func() {
	var err error

	DeferCleanup(gexec.CleanupBuildArtifacts)

	_pluginPath, err = buildPluginBinary()
	Expect(err).ToNot(
		HaveOccurred(),
		"Unable to build plug-in binary.",
	)

	Expect(os.Mkdir(outputPath, 0o755)).Error().ToNot(HaveOccurred())

	DeferCleanup(func() error {
		return os.RemoveAll(outputPath)
	})
})

var errSetup = errors.New("test setup failed")

func buildPluginBinary() (string, error) {
	root, err := projectRoot()
	if err != nil {
		return "", fmt.Errorf("determining project root: %w", err)
	}

	ldflags := strings.Join([]string{
		"-w", "-s",
		"--extldflags", "'-zrelro -znow -O1'",
		"-X", fmt.Sprintf("'%s/internal/version.version=%s'", module, version),
	}, " ")
	args := []string{
		"--ldflags", ldflags,
	}

	return gexec.Build(filepath.Join(root, "cmd", "kubectl-package"), args...)
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
