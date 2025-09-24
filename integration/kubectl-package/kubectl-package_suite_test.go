//go:build integration

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
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
	"package-operator.run/internal/packages"
)

func TestKubectlPackage(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "kubectl-package Suite")
}

var (
	_root           string
	_pluginPath     string
	_registryDomain string
	_registryClient *http.Client
	_tempDir        string
)

const outputPath = "output"

var _ = ginkgo.BeforeSuite(func() {
	var err error

	_tempDir = ginkgo.GinkgoT().TempDir()

	_root, err = projectRoot(context.Background())
	gomega.Expect(err).ToNot(
		gomega.HaveOccurred(),
		"Looking up project root.",
	)

	_pluginPath, err = buildPluginBinary()
	gomega.Expect(err).ToNot(
		gomega.HaveOccurred(),
		"Unable to build plug-in binary.",
	)

	ginkgo.DeferCleanup(gexec.CleanupBuildArtifacts)

	gomega.Expect(os.Mkdir(outputPath, 0o755)).Error().ToNot(gomega.HaveOccurred())

	ginkgo.DeferCleanup(func() error {
		return os.RemoveAll(outputPath)
	})

	srv, err := registry.TLS("example.com")
	gomega.Expect(err).ToNot(gomega.HaveOccurred())

	ginkgo.DeferCleanup(srv.Close)

	_registryDomain = strings.TrimPrefix(srv.URL, "https://")
	_registryClient = srv.Client()

	gomega.Expect(loadPackageImages(
		context.Background(),
		packageImageBuildInfo{
			Path: sourcePathFixture("valid_without_config"),
			Ref:  path.Join(_registryDomain, "valid-package-fixture"),
		},
		packageImageBuildInfo{
			Path: sourcePathFixture("invalid_bad_manifest"),
			Ref:  path.Join(_registryDomain, "invalid-package-fixture"),
		},
	)).To(gomega.Succeed())

	generateAllPackages(_tempDir, _registryDomain)
})

var errSetup = errors.New("test setup failed")

func buildPluginBinary() (string, error) {
	ldflags := strings.Join([]string{
		"-w", "-s",
		"--extldflags", "'-zrelro -znow -O1'",
		"-X", fmt.Sprintf("'%s/internal/version.version=%s'", module, version),
	}, " ")

	return gexec.Build(filepath.Join(_root, "cmd", "kubectl-package"), "--cover", "--ldflags", ldflags)
}

const (
	module  = "package-operator.run"
	version = "v0.0.0"
)

func projectRoot(ctx context.Context) (string, error) {
	var buf bytes.Buffer

	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--show-toplevel")
	cmd.Stdout = &buf
	cmd.Stderr = io.Discard

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("determining top level directory from git: %w", errSetup)
	}

	return strings.TrimSpace(buf.String()), nil
}

func loadPackageImages(ctx context.Context, infos ...packageImageBuildInfo) error {
	for _, info := range infos {
		rawPkg, err := packages.FromFolder(ctx, info.Path)
		if err != nil {
			return fmt.Errorf("importing package from directory: %w", err)
		}

		tags := []string{info.Ref}

		if err := packages.ToPushedOCI(ctx, tags, rawPkg, crane.Insecure); err != nil {
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
		withImages([]manifestsv1alpha1.PackageManifestImage{
			{
				Name:  path.Join(registry, "valid-package-fixture"),
				Image: path.Join(registry, "valid-package-fixture"),
			},
		}),
		withLockData{LockData: &manifestsv1alpha1.PackageManifestLock{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "manifests.package-operator.run/v1alpha1",
				Kind:       "PackageManifestLock",
			},
			Spec: manifestsv1alpha1.PackageManifestLockSpec{
				Images: []manifestsv1alpha1.PackageManifestLockImage{
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
		withImages([]manifestsv1alpha1.PackageManifestImage{
			{
				Name:  path.Join(registry, "dne"),
				Image: path.Join(registry, "dne"),
			},
		}),
		withLockData{LockData: &manifestsv1alpha1.PackageManifestLock{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "manifests.package-operator.run/v1alpha1",
				Kind:       "PackageManifestLock",
			},
			Spec: manifestsv1alpha1.PackageManifestLockSpec{
				Images: []manifestsv1alpha1.PackageManifestLockImage{
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
		withImages([]manifestsv1alpha1.PackageManifestImage{
			{
				Name:  path.Join(registry, "valid-package-fixture"),
				Image: path.Join(registry, "valid-package-fixture"),
			},
		}),
		withLockData{LockData: &manifestsv1alpha1.PackageManifestLock{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "manifests.package-operator.run/v1alpha1",
				Kind:       "PackageManifestLock",
			},
			Spec: manifestsv1alpha1.PackageManifestLockSpec{
				Images: []manifestsv1alpha1.PackageManifestLockImage{
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
