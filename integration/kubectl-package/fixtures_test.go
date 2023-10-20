package kubectlpackage

import (
	"os"
	"path/filepath"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	manv1alpha1 "package-operator.run/apis/manifests/v1alpha1"

	. "github.com/onsi/gomega"
)

func sourcePathFixture(name string) string {
	var (
		packagesDir     = filepath.Join("testdata", "packages")
		invalidPackages = filepath.Join(packagesDir, "invalid")
		validPackages   = filepath.Join(packagesDir, "valid")
	)

	return map[string]string{
		"valid_without_config":                              filepath.Join(validPackages, "without_config"),
		"valid_with_config":                                 filepath.Join(validPackages, "with_config"),
		"valid_with_config_no_tests_no_required_properties": filepath.Join(validPackages, "with_config_no_tests_no_required_properties"),
		"invalid_bad_manifest":                              filepath.Join(invalidPackages, "bad_manifest"),
		"invalid_invalid_resource_label":                    filepath.Join(invalidPackages, "invalid_resource_label"),
		"invalid_missing_lock_file":                         filepath.Join(invalidPackages, "missing_lock_file"),
		"invalid_missing_phase_annotation":                  filepath.Join(invalidPackages, "missing_phase_annotation"),
		"invalid_missing_resource_gvk":                      filepath.Join(invalidPackages, "missing_resource_gvk"),
	}[name]
}

func generatePackage(dir string, opts ...generatePackageOption) {
	var cfg generatePackageConfig

	cfg.Option(opts...)

	ExpectWithOffset(1, os.MkdirAll(dir, 0o755)).To(Succeed())

	man := manv1alpha1.PackageManifest{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "manifests.package-operator.run/v1alpha1",
			Kind:       "PackageManifest",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-package",
		},
		Spec: manv1alpha1.PackageManifestSpec{
			Phases: []manv1alpha1.PackageManifestPhase{
				{
					Name: "test",
				},
			},
			Scopes: []manv1alpha1.PackageManifestScope{
				manv1alpha1.PackageManifestScopeCluster,
			},
			AvailabilityProbes: []corev1alpha1.ObjectSetProbe{
				{
					Probes: []corev1alpha1.Probe{
						{
							Condition: &corev1alpha1.ProbeConditionSpec{
								Type:   "Available",
								Status: "True",
							},
							FieldsEqual: &corev1alpha1.ProbeFieldsEqualSpec{
								FieldA: ".status.updatedReplicas",
								FieldB: ".status.repicas",
							},
						},
					},
					Selector: corev1alpha1.ProbeSelector{
						Kind: &corev1alpha1.PackageProbeKindSpec{
							Group: "apps",
							Kind:  "Deployment",
						},
					},
				},
			},
			Images: cfg.Images,
		},
	}

	manData, err := yaml.Marshal(man)
	ExpectWithOffset(1, err).ToNot(HaveOccurred())

	ExpectWithOffset(1, os.WriteFile(filepath.Join(dir, "manifest.yaml"), manData, 0o644)).To(Succeed())

	if cfg.LockData != nil {
		lockData, err := yaml.Marshal(cfg.LockData)
		ExpectWithOffset(1, err).ToNot(HaveOccurred())

		ExpectWithOffset(1, os.WriteFile(filepath.Join(dir, "manifest.lock.yaml"), lockData, 0o644)).To(Succeed())
	}
}

type generatePackageConfig struct {
	Images   []manv1alpha1.PackageManifestImage
	LockData *manv1alpha1.PackageManifestLock
}

func (c *generatePackageConfig) Option(opts ...generatePackageOption) {
	for _, opt := range opts {
		opt.ConfigureGeneratePackage(c)
	}
}

type generatePackageOption interface {
	ConfigureGeneratePackage(*generatePackageConfig)
}

type withImages []manv1alpha1.PackageManifestImage

func (w withImages) ConfigureGeneratePackage(c *generatePackageConfig) {
	c.Images = append(c.Images, w...)
}

type withLockData struct {
	LockData *manv1alpha1.PackageManifestLock
}

func (w withLockData) ConfigureGeneratePackage(c *generatePackageConfig) {
	c.LockData = w.LockData
}
