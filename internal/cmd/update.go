package cmd

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strings"

	"github.com/go-logr/logr"
	"github.com/gobuffalo/flect"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/yaml"

	"package-operator.run/apis/manifests/v1alpha1"
	"package-operator.run/internal/packages"
	"package-operator.run/internal/packages/packagecontent"
	"package-operator.run/internal/packages/packageimport"
	"package-operator.run/internal/packages/packageloader"
	"package-operator.run/internal/utils"
)

func NewUpdate(opts ...UpdateOption) *Update {
	var cfg UpdateConfig

	cfg.Option(opts...)
	cfg.Default()

	return &Update{
		cfg: cfg,
	}
}

type Update struct {
	cfg UpdateConfig
}

type UpdateConfig struct {
	Log      logr.Logger
	Clock    Clock
	Loader   PackageLoader
	Resolver DigestResolver
}

func (c *UpdateConfig) Option(opts ...UpdateOption) {
	for _, opt := range opts {
		opt.ConfigureUpdate(c)
	}
}

func (c *UpdateConfig) Default() {
	if c.Log.GetSink() == nil {
		c.Log = logr.Discard()
	}

	if c.Clock == nil {
		c.Clock = &defaultClock{}
	}

	if c.Loader == nil {
		c.Loader = NewDefaultPackageLoader(runtime.NewScheme())
	}

	if c.Resolver == nil {
		c.Resolver = &defaultDigestResolver{}
	}
}

type UpdateOption interface {
	ConfigureUpdate(*UpdateConfig)
}

func (u *Update) GenerateLockData(ctx context.Context, srcPath string, opts ...GenerateLockDataOption) (data []byte, err error) {
	var cfg GenerateLockDataConfig

	cfg.Option(opts...)

	pkg, fileMap, err := u.cfg.Loader.LoadPackage(ctx, srcPath)
	if err != nil {
		return nil, fmt.Errorf("loading package: %w", err)
	}

	lockImages := make([]v1alpha1.PackageManifestLockImage, len(pkg.PackageManifest.Spec.Images))
	for i, img := range pkg.PackageManifest.Spec.Images {
		lockImg, err := u.lockImageFromManifestImage(cfg, img)
		if err != nil {
			return nil, fmt.Errorf("resolving lock image for %q: %w", img.Image, err)
		}

		lockImages[i] = lockImg
	}

	permissions, err := gatherPermissions(ctx, fileMap)
	if err != nil {
		return nil, fmt.Errorf("gather permissions: %w", err)
	}

	manifestLock := &v1alpha1.PackageManifestLock{
		TypeMeta: v1.TypeMeta{
			Kind:       packages.PackageManifestLockGroupKind.Kind,
			APIVersion: v1alpha1.GroupVersion.String(),
		},
		ObjectMeta: v1.ObjectMeta{
			CreationTimestamp: u.cfg.Clock.Now(),
		},
		Spec: v1alpha1.PackageManifestLockSpec{
			Images:             lockImages,
			InstallPermissions: permissions,
		},
	}

	if pkg.PackageManifestLock != nil && lockSpecsAreEqual(manifestLock.Spec, pkg.PackageManifestLock.Spec) {
		return nil, ErrLockDataUnchanged
	}

	manifestLockYaml, err := yaml.Marshal(manifestLock)
	if err != nil {
		return nil, fmt.Errorf("unmarshalling manifest lock file: %w", err)
	}

	return manifestLockYaml, nil
}

var ErrLockDataUnchanged = errors.New("lock data unchanged")

func (u *Update) lockImageFromManifestImage(cfg GenerateLockDataConfig, img v1alpha1.PackageManifestImage) (v1alpha1.PackageManifestLockImage, error) {
	overriddenImage, err := utils.ImageURLWithOverrideFromEnv(img.Image)
	if err != nil {
		return v1alpha1.PackageManifestLockImage{}, fmt.Errorf("resolving image URL: %w", err)
	}

	digest, err := u.cfg.Resolver.ResolveDigest(overriddenImage, WithInsecure(cfg.Insecure))
	if err != nil {
		return v1alpha1.PackageManifestLockImage{}, fmt.Errorf("resolving image digest: %w", err)
	}

	return v1alpha1.PackageManifestLockImage{
		Name:   img.Name,
		Image:  img.Image,
		Digest: digest,
	}, nil
}

func lockSpecsAreEqual(spec v1alpha1.PackageManifestLockSpec, other v1alpha1.PackageManifestLockSpec) bool {
	thisImages := map[string]v1alpha1.PackageManifestLockImage{}
	for _, image := range spec.Images {
		thisImages[image.Name] = image
	}

	otherImages := map[string]v1alpha1.PackageManifestLockImage{}
	for _, image := range other.Images {
		otherImages[image.Name] = image
	}

	if len(thisImages) != len(otherImages) {
		return false
	}

	for name, image := range thisImages {
		otherImage, exists := otherImages[name]
		if !exists || otherImage != image {
			return false
		}
	}

	return reflect.DeepEqual(spec.InstallPermissions, other.InstallPermissions)
}

type GenerateLockDataConfig struct {
	Insecure bool
}

func (c *GenerateLockDataConfig) Option(opts ...GenerateLockDataOption) {
	for _, opt := range opts {
		opt.ConfigureGenerateLockData(c)
	}
}

type GenerateLockDataOption interface {
	ConfigureGenerateLockData(*GenerateLockDataConfig)
}

type PackageLoader interface {
	LoadPackage(ctx context.Context, path string) (*packagecontent.Package, packagecontent.Files, error)
}

func NewDefaultPackageLoader(scheme *runtime.Scheme) *DefaultPackageLoader {
	return &DefaultPackageLoader{
		scheme: scheme,
	}
}

type DefaultPackageLoader struct {
	scheme *runtime.Scheme
}

func (l *DefaultPackageLoader) LoadPackage(ctx context.Context, path string) (
	*packagecontent.Package, packagecontent.Files, error,
) {
	var fileMap packagecontent.Files

	fileMap, err := packageimport.Folder(ctx, path)
	if err != nil {
		return nil, nil, err
	}

	pkg, err := packageloader.New(l.scheme).FromFiles(ctx, fileMap)
	if err != nil {
		return nil, nil, err
	}

	return pkg, fileMap, nil
}

type Clock interface {
	Now() v1.Time
}

type defaultClock struct{}

func (c *defaultClock) Now() v1.Time {
	return v1.Now()
}

func gatherPermissions(ctx context.Context, fileMap packagecontent.Files) ([]rbacv1.PolicyRule, error) {
	gks, err := gatherGroupKinds(ctx, fileMap)
	if err != nil {
		return nil, err
	}

	var rules []rbacv1.PolicyRule
	for group, kinds := range groupKindsByGroup(gks) {
		rules = append(rules, rbacv1.PolicyRule{
			APIGroups: []string{group},
			Resources: kindsToResource(kinds),
			Verbs: []string{
				"get", "list", "watch",
				"update", "patch",
				"create", "delete",
			},
		})
	}

	return rules, nil
}

func kindsToResource(kinds []string) []string {
	var resources []string
	for _, kind := range kinds {
		resources = append(resources, strings.ToLower(flect.Pluralize(kind)))
	}
	return resources
}

func groupKindsByGroup(gks []schema.GroupKind) map[string][]string {
	out := map[string][]string{}
	for _, gk := range gks {
		out[gk.Group] = append(out[gk.Group], gk.Kind)
	}
	return out
}

var (
	kindRegex       = regexp.MustCompile(`kind:(.*)`)
	apiVersionRegex = regexp.MustCompile(`apiVersion:(.*)`)
)

func gatherGroupKinds(ctx context.Context, fileMap packagecontent.Files) ([]schema.GroupKind, error) {
	var gks []schema.GroupKind
	for path, content := range fileMap {
		if packages.IsManifestFile(path) ||
			packages.IsManifestLockFile(path) {
			continue
		}

		if packages.IsYAMLFile(path) {
			for _, yamlDocument := range packages.YAMLDocumentSplitRegex.Split(strings.Trim(string(content), "---\n"), -1) {
				var typeMeta metav1.TypeMeta
				if err := yaml.Unmarshal([]byte(yamlDocument), &typeMeta); err != nil {
					return nil, err
				}
				gks = append(gks, typeMeta.GroupVersionKind().GroupKind())
			}
			continue
		}

		if packages.IsTemplateFile(path) {
			for i, templateDocument := range packages.YAMLDocumentSplitRegex.Split(strings.Trim(string(content), "---\n"), -1) {
				apiVersions := apiVersionRegex.FindAll([]byte(templateDocument), -1)
				if len(apiVersions) == 0 {
					return nil, fmt.Errorf("missing apiVersion in YAML document %s#%d", path, i)
				}
				if len(apiVersions) > 1 {
					return nil, fmt.Errorf("multiple apiVersions in YAML document %s#%d", path, i)
				}
				apiVersion := strings.TrimSpace(strings.TrimPrefix(string(apiVersions[0]), "apiVersion:"))
				if strings.Contains(apiVersion, "{") {
					return nil, fmt.Errorf("template within apiVersion: not allowed in YAML document %s#%d", path, i)
				}
				gv, err := schema.ParseGroupVersion(apiVersion)
				if err != nil {
					return nil, fmt.Errorf("parsing apiVersion: %w in YAML document %s#%d", err, path, i)
				}

				kinds := kindRegex.FindAll([]byte(templateDocument), -1)
				if len(kinds) == 0 {
					return nil, fmt.Errorf("missing kind in YAML document %s#%d", path, i)
				}
				if len(kinds) > 1 {
					return nil, fmt.Errorf("multiple kinds in YAML document %s#%d", path, i)
				}
				kind := strings.TrimSpace(strings.TrimPrefix(string(kinds[0]), "kind:"))
				if strings.Contains(kind, "{") {
					return nil, fmt.Errorf("template within kind: not allowed in YAML document %s#%d", path, i)
				}
				gks = append(gks, gv.WithKind(kind).GroupKind())
			}
			continue
		}
	}
	return gks, nil
}
