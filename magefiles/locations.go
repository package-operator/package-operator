//go:build mage

package main

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/mt-sre/devkube/dev"
	"github.com/mt-sre/devkube/magedeps"
	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	kindv1alpha4 "sigs.k8s.io/kind/pkg/apis/config/v1alpha4"
)

type Locations struct {
	lock             *sync.Mutex
	devEnvironment   *dev.Environment
	containerRuntime string
	cache            string
	bin              string
	imageOrg         string
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

	must(os.MkdirAll(string(l.Deps()), 0o755))
	must(os.MkdirAll(l.unitTestCache(), 0o755))
	must(os.MkdirAll(l.IntegrationTestCache(), 0o755))

	return l
}

func (l Locations) Cache() string                  { return l.cache }
func (l Locations) APISubmodule() string           { return "apis" }
func (l Locations) ClusterDeploymentCache() string { return filepath.Join(l.Cache(), "deploy") }
func (l Locations) unitTestCache() string          { return filepath.Join(l.Cache(), "unit") }
func (l Locations) UnitTestCoverageReport() string {
	return filepath.Join(l.unitTestCache(), "cov.out")
}
func (l Locations) UnitTestExecReport() string   { return filepath.Join(l.unitTestCache(), "exec.json") }
func (l Locations) UnitTestStdOut() string       { return filepath.Join(l.unitTestCache(), "out.txt") }
func (l Locations) IntegrationTestCache() string { return filepath.Join(l.Cache(), "integration") }
func (l Locations) PKOIntegrationTestCoverageReport() string {
	return filepath.Join(l.IntegrationTestCache(), "pko-cov.out")
}
func (l Locations) PKOIntegrationTestExecReport() string {
	return filepath.Join(l.IntegrationTestCache(), "pko-exec.json")
}
func (l Locations) PluginIntegrationTestCoverageReport() string {
	return filepath.Join(l.IntegrationTestCache(), "kubectl-package-cov.out")
}
func (l Locations) PluginIntegrationTestExecReport() string {
	return filepath.Join(l.IntegrationTestCache(), "kubectl-package-exec.json")
}
func (l Locations) IntegrationTestLogs() string { return filepath.Join(l.Cache(), "dev-env-logs") }
func (l Locations) ImageCache(imageName string) string {
	return filepath.Join(l.Cache(), "image", imageName)
}
func (l Locations) DigestFile(imgName string) string {
	return filepath.Join(l.ImageCache(imgName), imgName+".digest")
}
func (l Locations) APIReference() string    { return filepath.Join("docs", "api-reference.md") }
func (l Locations) NativeCliBinary() string { return l.binaryDst(cliCmdName, nativeArch) }
func (l Locations) Deps() magedeps.DependencyDirectory {
	return magedeps.DependencyDirectory(filepath.Join(l.Cache(), "deps"))
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

func (l *Locations) ContainerRuntime() string {
	l.lock.Lock()
	defer l.lock.Unlock()

	if len(l.containerRuntime) == 0 {
		l.containerRuntime = os.Getenv("CONTAINER_RUNTIME")
		if len(l.containerRuntime) == 0 || l.containerRuntime == "auto" {
			cr, err := dev.DetectContainerRuntime()
			if err != nil {
				panic(err)
			}
			l.containerRuntime = string(cr)
			logger.Info("detected container-runtime", "container-runtime", l.containerRuntime)
		}
	}

	return l.containerRuntime
}

func (l *Locations) DevEnv() *dev.Environment {
	containerRuntime := l.ContainerRuntime()
	l.lock.Lock()
	defer l.lock.Unlock()

	var clusterInitializers []dev.ClusterInitializer
	if _, isCI := os.LookupEnv("CI"); !isCI {
		// don't install the monitoring stack in CI to speed up tests.
		clusterInitializers = dev.WithClusterInitializers{
			dev.ClusterHelmInstall{
				RepoName:    "prometheus-community",
				RepoURL:     "https://prometheus-community.github.io/helm-charts",
				PackageName: "kube-prometheus-stack",
				ReleaseName: "prometheus",
				Namespace:   "monitoring",
				SetVars: []string{
					"grafana.enabled=true",
					"kubeStateMetrics.enabled=false",
					"nodeExporter.enabled=false",
				},
			},
			dev.ClusterLoadObjectsFromFiles{
				"config/service-monitor.yaml",
			},
		}
	}

	if l.devEnvironment == nil {
		l.devEnvironment = dev.NewEnvironment(
			clusterName,
			filepath.Join(l.Cache(), "dev-env"),
			dev.WithClusterOptions([]dev.ClusterOption{
				dev.WithWaitOptions([]dev.WaitOption{dev.WithTimeout(2 * time.Minute)}),
				dev.WithSchemeBuilder{corev1alpha1.AddToScheme},
			}),
			dev.WithContainerRuntime(containerRuntime),
			dev.WithClusterInitializers(
				append(clusterInitializers, dev.ClusterLoadObjectsFromFiles{
					"config/local-registry.yaml",
				}),
			),
			dev.WithKindClusterConfig(kindv1alpha4.Cluster{
				ContainerdConfigPatches: []string{
					// Replace `imageRegistry` with our local dev-registry.
					fmt.Sprintf(`[plugins."io.containerd.grpc.v1.cri".registry.mirrors."%s"]
	endpoint = ["http://localhost:31320"]`, imageRegistry),
				},
				Nodes: []kindv1alpha4.Node{
					{
						Role: kindv1alpha4.ControlPlaneRole,
						ExtraPortMappings: []kindv1alpha4.PortMapping{
							// Open port to enable connectivity with local registry.
							{
								ContainerPort: 5001,
								HostPort:      5001,
								ListenAddress: "127.0.0.1",
								Protocol:      "TCP",
							},
						},
					},
				},
			}),
		)
	}

	return l.devEnvironment
}

// DevEnvNoInit returns the dev environment if DevelopmentEnvironment was
// already called, nil if not. This is used in case the env is optional.
func (l *Locations) DevEnvNoInit() *dev.Environment {
	l.lock.Lock()
	defer l.lock.Unlock()

	return l.devEnvironment
}
