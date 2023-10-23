//go:build mage

package main

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"github.com/mt-sre/devkube/devcr"
)

const devClusterName = "package-operator"

var join = filepath.Join

type Locations struct {
	lock     *sync.Mutex
	cache    string
	bin      string
	imageOrg string
	cr       devcr.ContainerRuntime
}

func newLocations() Locations {
	// Entrypoint ./mage uses .cache/magefile as cache so .cache should exist.
	absCache, err := filepath.Abs(".cache")
	must(err)

	// TODO(jgwosdz) Why is application version set right here inside newLocations?

	// Use version from VERSION env if present, use "git describe" elsewise.
	applicationVersion = strings.TrimSpace(os.Getenv("VERSION"))
	if len(applicationVersion) == 0 {
		gitDescribeCmd := exec.Command("git", "describe", "--tags")
		byteVersion, err := gitDescribeCmd.Output()
		if err != nil {
			panic(fmt.Errorf("git describe: %w", err))
		}

		// Depending on what process was used the last tag my either be a version for
		// the main module (eg `v1.6.6`) or a version for a submodule (eg `apis/v1.6.6`).
		applicationVersion = path.Base(strings.TrimSpace(string(byteVersion)))
	}

	// image org
	imageOrg := os.Getenv("IMAGE_ORG")
	if len(imageOrg) == 0 {
		imageOrg = defaultImageOrg
	}

	// extract image registry 'hostname' from `imageOrg`
	url, err := url.Parse(fmt.Sprintf("http://%s", imageOrg))
	must(err)
	imageRegistry = url.Host

	l := Locations{
		lock: &sync.Mutex{}, cache: absCache, imageOrg: imageOrg,
		bin: filepath.Join(filepath.Dir(absCache), "bin"),
	}

	must(os.MkdirAll(l.unitTestCache(), 0o755))
	must(os.MkdirAll(l.IntTestCache(), 0o755))

	return l
}

func (l Locations) DependencyBin() string          { return join(l.Cache(), "bin") }
func (l Locations) BuildBin() string               { return l.bin }
func (l Locations) Cache() string                  { return l.cache }
func (l Locations) APISubmodule() string           { return "apis" }
func (l Locations) KindCache() string              { return join(l.Cache(), "kind") }
func (l Locations) ClusterDeploymentCache() string { return join(l.Cache(), "deploy") }
func (l Locations) unitTestCache() string          { return join(l.Cache(), "unit") }
func (l Locations) UnitTestCovReport() string      { return join(l.unitTestCache(), "cov.out") }
func (l Locations) UnitTestExecReport() string     { return join(l.unitTestCache(), "exec.json") }
func (l Locations) UnitTestStdOut() string         { return join(l.unitTestCache(), "out.txt") }
func (l Locations) IntTestCache() string           { return join(l.Cache(), "integration") }
func (l Locations) APIReference() string           { return join("docs", "api-reference.md") }
func (l Locations) NativeCliBinary() string        { return l.binaryDst(cliCmdName, nativeArch) }
func (l Locations) IntegrationTestLogs() string    { return join(l.Cache(), "dev-env-logs") }
func (l Locations) DigestFile(img string) string   { return join(l.ImageCache(img), img+".digest") }
func (l Locations) PKOIntTestCovReport() string    { return join(l.IntTestCache(), "pko-cov.out") }
func (l Locations) PKOIntTestExecReport() string   { return join(l.IntTestCache(), "pko-exec.json") }
func (l Locations) ImageCache(img string) string   { return join(l.Cache(), "image", img) }
func (l Locations) PluginIntTestCovReport() string {
	return join(l.IntTestCache(), "kubectl-package-cov.out")
}
func (l Locations) PluginIntTestExecReport() string {
	return join(l.IntTestCache(), "kubectl-package-exec.json")
}
func (l Locations) binaryDst(name string, arch archTarget) string {
	if arch == nativeArch {
		return filepath.Join("bin", name)
	}

	if len(arch.OS) == 0 || len(arch.Arch) == 0 {
		panic("invalid os or arch")
	}

	return filepath.Join("bin", arch.OS+"_"+arch.Arch, name)
}

func (l Locations) ImageURL(name string, useDigest bool) string {
	envvar := strings.ReplaceAll(strings.ToUpper(name), "-", "_") + "_IMAGE"
	if url := os.Getenv(envvar); len(url) != 0 {
		return url
	}
	image := l.imageOrg + "/" + name + ":" + applicationVersion
	if !useDigest {
		return image
	}

	digest, err := os.ReadFile(locations.DigestFile(name))
	if err != nil {
		panic(err)
	}

	return l.imageOrg + "/" + name + "@" + string(digest)
}

func (l Locations) LocalImageURL(name string) string {
	url := l.ImageURL(name, false)
	return strings.Replace(url, imageRegistry, "localhost:5001", 1)
}

func (l *Locations) ContainerRuntime(ctx context.Context) devcr.ContainerRuntime {
	l.lock.Lock()
	defer l.lock.Unlock()

	if l.cr == nil {

		switch os.Getenv("CONTAINER_RUNTIME") {
		case "podman":
			l.cr = devcr.Podman{}
		case "docker":
			l.cr = devcr.Docker{}
		case "":
			cr, err := devcr.Detect(ctx, nil)
			must(err)
			l.cr = cr
		default:
			panic("unknown container runtime")
		}
	}

	return l.cr
}
