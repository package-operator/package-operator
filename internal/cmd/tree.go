package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/disiqueira/gotree"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"k8s.io/apimachinery/pkg/util/validation/field"

	"package-operator.run/apis/core/v1alpha1"
	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
	"package-operator.run/internal/packages/packageadmission"
	"package-operator.run/internal/packages/packagecontent"
	"package-operator.run/internal/packages/packageimport"
	"package-operator.run/internal/packages/packageloader"
	"package-operator.run/internal/utils"
)

func NewTree(scheme *runtime.Scheme, opts ...TreeOption) *Tree {
	var cfg TreeConfig

	cfg.Option(opts...)
	cfg.Default()

	return &Tree{
		cfg:    cfg,
		scheme: scheme,
	}
}

type Tree struct {
	cfg    TreeConfig
	scheme *runtime.Scheme
}

type TreeConfig struct {
	Log logr.Logger
}

func (c *TreeConfig) Option(opts ...TreeOption) {
	for _, opt := range opts {
		opt.ConfigureTree(c)
	}
}

func (c *TreeConfig) Default() {
	if c.Log.GetSink() == nil {
		c.Log = logr.Discard()
	}
}

type TreeOption interface {
	ConfigureTree(*TreeConfig)
}

func (t *Tree) RenderPackage(ctx context.Context, srcPath string, opts ...RenderPackageOption) (string, error) {
	var cfg RenderPackageConfig

	cfg.Option(opts...)

	t.cfg.Log.Info("loading source from disk", "path", srcPath)

	files, err := packageimport.Folder(ctx, srcPath)
	if err != nil {
		return "", fmt.Errorf("loading package contents from folder: %w", err)
	}

	pkg, err := packagecontent.PackageFromFiles(ctx, t.scheme, files, cfg.Component)
	if err != nil {
		return "", fmt.Errorf("parsing package contents: %w", err)
	}

	tmplCtx := t.getTemplateContext(pkg, cfg)
	tmplCfg, err := t.getConfig(pkg, cfg)
	if err != nil {
		return "", fmt.Errorf("getting config: %w", err)
	}

	validationErrors, err := packageadmission.AdmitPackageConfiguration(
		ctx, t.scheme, tmplCfg, pkg.PackageManifest, field.NewPath("spec", "config"))
	if err != nil {
		return "", fmt.Errorf("validate Package configuration: %w", err)
	}
	if len(validationErrors) > 0 {
		return "", validationErrors.ToAggregate()
	}

	tmplCtx.Config = tmplCfg
	tmplCtx.Images = utils.GenerateStaticImages(pkg.PackageManifest)

	pkgPrefix := "Package"
	scope := manifestsv1alpha1.PackageManifestScopeNamespaced
	if cfg.ClusterScope || len(tmplCtx.Package.Namespace) == 0 {
		scope = manifestsv1alpha1.PackageManifestScopeCluster
		tmplCtx.Package.Namespace = ""
		pkgPrefix = "ClusterPackage"
	}

	tt, err := packageloader.NewTemplateTransformer(tmplCtx)
	if err != nil {
		return "", err
	}

	l := packageloader.New(t.scheme, packageloader.WithDefaults,
		packageloader.WithValidators(packageloader.PackageScopeValidator(scope)),
		packageloader.WithFilesTransformers(tt),
	)

	packageContent, err := l.FromFiles(ctx, files)
	if err != nil {
		return "", fmt.Errorf("parsing package contents: %w", err)
	}

	pkgTree := newTreeFromSpec(
		fmt.Sprintf("%s\n%s %s",
			packageContent.PackageManifest.Name,
			pkgPrefix, client.ObjectKey{
				Name:      tmplCtx.Package.Name,
				Namespace: tmplCtx.Package.Namespace,
			},
		),
		packagecontent.TemplateSpecFromPackage(packageContent),
	)

	return pkgTree.Print(), nil
}

func (t *Tree) getTemplateContext(pkg *packagecontent.Package, cfg RenderPackageConfig) packageloader.PackageFileTemplateContext {
	templateContext := packageloader.PackageFileTemplateContext{
		Package: manifestsv1alpha1.TemplateContextPackage{
			TemplateContextObjectMeta: manifestsv1alpha1.TemplateContextObjectMeta{
				Name:      "name",
				Namespace: "namespace",
			},
		},
	}

	switch {
	case cfg.ConfigTestcase != "":
		for _, test := range pkg.PackageManifest.Test.Template {
			if test.Name != cfg.ConfigTestcase {
				continue
			}

			templateContext = packageloader.PackageFileTemplateContext{
				Package: test.Context.Package,
			}
		}
	case len(pkg.PackageManifest.Test.Template) > 0:
		test := pkg.PackageManifest.Test.Template[0]

		templateContext = packageloader.PackageFileTemplateContext{
			Package: test.Context.Package,
		}
	}

	return templateContext
}

func (t *Tree) getConfig(pkg *packagecontent.Package, cfg RenderPackageConfig) (map[string]any, error) {
	var config map[string]any

	switch {
	case cfg.ConfigPath != "":
		data, err := os.ReadFile(cfg.ConfigPath)
		if err != nil {
			return nil, fmt.Errorf("read config from file: %w", err)
		}
		if err := yaml.Unmarshal(data, &config); err != nil {
			return nil, fmt.Errorf("unmarshal config from file %s: %w", cfg.ConfigPath, err)
		}
	case cfg.ConfigTestcase != "":
		for _, test := range pkg.PackageManifest.Test.Template {
			if test.Name != cfg.ConfigTestcase {
				continue
			}

			if test.Context.Config == nil {
				return config, nil
			}
			if err := json.Unmarshal(test.Context.Config.Raw, &config); err != nil {
				return nil, fmt.Errorf("unmarshal config from test template %s: %w", cfg.ConfigTestcase, err)
			}
		}

		if config == nil {
			return nil, fmt.Errorf("%w: test template with name %s not found", ErrInvalidArgs, cfg.ConfigTestcase)
		}
	case len(pkg.PackageManifest.Test.Template) > 0:
		testCtxCfg := pkg.PackageManifest.Test.Template[0].Context.Config
		if testCtxCfg == nil {
			return config, nil
		}

		if err := json.Unmarshal(testCtxCfg.Raw, &config); err != nil {
			return nil, fmt.Errorf("unmarshal config from first test template: %w", err)
		}
	}

	return config, nil
}

func newTreeFromSpec(header string, spec v1alpha1.ObjectSetTemplateSpec) gotree.Tree {
	tree := gotree.New(header)

	for _, phase := range spec.Phases {
		treePhase := tree.Add(fmt.Sprintf("Phase %s", phase.Name))

		for _, obj := range phase.Objects {
			treePhase.Add(
				fmt.Sprintf("%s %s",
					obj.Object.GroupVersionKind(),
					client.ObjectKeyFromObject(&obj.Object)))
		}

		for _, obj := range phase.ExternalObjects {
			treePhase.Add(
				fmt.Sprintf("%s %s (EXTERNAL)",
					obj.Object.GroupVersionKind(),
					client.ObjectKeyFromObject(&obj.Object)))
		}
	}

	return tree
}

type RenderPackageConfig struct {
	ClusterScope   bool
	ConfigPath     string
	ConfigTestcase string
	Component      string
}

func (c *RenderPackageConfig) Option(opts ...RenderPackageOption) {
	for _, opt := range opts {
		opt.ConfigureRenderPackage(c)
	}
}

type RenderPackageOption interface {
	ConfigureRenderPackage(*RenderPackageConfig)
}
