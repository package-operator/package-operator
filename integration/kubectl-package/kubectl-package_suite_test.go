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
	"regexp"
	"runtime"
	"strings"
	"testing"

	"golang.org/x/mod/semver"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/registry"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	manv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
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
	_tempDir        string
)

const (
	outputPath                 = "output"
	coverProfilingMinGoVersion = "1.20.0"
)

var _ = BeforeSuite(func() {
	var err error

	_tempDir = GinkgoT().TempDir()

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

	generateAllPackages(_tempDir, _registryDomain)
})

var (
	errSetup               = errors.New("test setup failed")
	errRegexpMatchNotFound = errors.New("no match found for regexp")
)

func getGoVersion() (string, error) {
	goVersion := runtime.Version()
	r := regexp.MustCompile(`\d(?:\.\d+){2}`)
	parsedVersion := r.FindString(goVersion)
	if parsedVersion == "" {
		return parsedVersion, errRegexpMatchNotFound
	}
	return parsedVersion, nil
}

func buildPluginBinary() (string, error) {
	ldflags := strings.Join([]string{
		"-w", "-s",
		"--extldflags", "'-zrelro -znow -O1'",
		"-X", fmt.Sprintf("'%s/internal/version.version=%s'", module, version),
	}, " ")

	goVersion, err := getGoVersion()
	if err != nil {
		return "", fmt.Errorf("getting go version: %w", err)
	}
	args := []string{}
	if semver.Compare("v"+goVersion, "v"+coverProfilingMinGoVersion) >= 0 {
		args = append(args, "--cover")
	}
	args = append(args, "--ldflags", ldflags)

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

func generateAllPackages(rootDir, registry string) {
	generatePackage(
		filepath.Join(rootDir, "valid_package"),
		withImages([]manv1alpha1.PackageManifestImage{
			{
				Name:  path.Join(registry, "valid-package-fixture"),
				Image: path.Join(registry, "valid-package-fixture"),
			},
		}),
		withLockData{LockData: &manv1alpha1.PackageManifestLock{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "manifests.package-operator.run/v1alpha1",
				Kind:       "PackageManifestLock",
			},
			Spec: manv1alpha1.PackageManifestLockSpec{
				Images: []manv1alpha1.PackageManifestLockImage{
					{
						Name:   path.Join(registry, "valid-package-fixture"),
						Image:  path.Join(registry, "valid-package-fixture"),
						Digest: "1234",
					},
				},
			},
		}},
	)
	generatePackage(
		filepath.Join(rootDir, "valid_package_invalid_lockfile_unresolvable_images"),
		withImages([]manv1alpha1.PackageManifestImage{
			{
				Name:  path.Join(registry, "dne"),
				Image: path.Join(registry, "dne"),
			},
		}),
		withLockData{LockData: &manv1alpha1.PackageManifestLock{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "manifests.package-operator.run/v1alpha1",
				Kind:       "PackageManifestLock",
			},
			Spec: manv1alpha1.PackageManifestLockSpec{
				Images: []manv1alpha1.PackageManifestLockImage{
					{
						Name:   path.Join(registry, "dne"),
						Image:  path.Join(registry, "dne"),
						Digest: "1234",
					},
				},
			},
		}},
	)
	generatePackage(
		filepath.Join(rootDir, "valid_package_valid_lockfile"),
		withImages([]manv1alpha1.PackageManifestImage{
			{
				Name:  path.Join(registry, "valid-package-fixture"),
				Image: path.Join(registry, "valid-package-fixture"),
			},
		}),
		withLockData{LockData: &manv1alpha1.PackageManifestLock{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "manifests.package-operator.run/v1alpha1",
				Kind:       "PackageManifestLock",
			},
			Spec: manv1alpha1.PackageManifestLockSpec{
				Images: []manv1alpha1.PackageManifestLockImage{
					{
						Name:   path.Join(registry, "valid-package-fixture"),
						Image:  path.Join(registry, "valid-package-fixture"),
						Digest: "sha256:bbb83bd537f5b3179b5d56b9a9086fb9abaa79e18d4d70b7f6572b77500286e4",
					},
				},
			},
		}},
	)
}
