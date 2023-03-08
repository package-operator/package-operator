package treecmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/disiqueira/gotree"
	"github.com/go-logr/logr"
	"github.com/go-logr/logr/funcr"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"

	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
	"package-operator.run/package-operator/cmd/cmdutil"
	"package-operator.run/package-operator/internal/packages/packagecontent"
	"package-operator.run/package-operator/internal/packages/packageimport"
	"package-operator.run/package-operator/internal/packages/packageloader"
)

const (
	cmdUse            = "tree source_path"
	cmdShort          = "outputs a logical tree view of the package contents"
	cmdLong           = "outputs a logical tree view of the package by printing root->phases->objects"
	clusterScopeUse   = "render package in cluster scope"
	configTestcaseUse = "name of the testcase which config is for templating"
	configPathUse     = "file containing config which is used for templating."
)

type Tree struct {
	SourcePath     string
	ClusterScope   bool
	ConfigPath     string
	ConfigTestcase string
}

func (t *Tree) Complete(args []string) error {
	switch {
	case len(args) != 1:
		return fmt.Errorf("%w: got %v positional args. Need one argument containing the source path", cmdutil.ErrInvalidArgs, len(args))
	case args[0] == "":
		return fmt.Errorf("%w: source path empty", cmdutil.ErrInvalidArgs)
	}

	if len(t.ConfigPath) != 0 && len(t.ConfigTestcase) != 0 {
		return fmt.Errorf("%w: only one of config-path and config-testcase may be set", cmdutil.ErrInvalidArgs)
	}

	t.SourcePath = args[0]
	return nil
}

func (t *Tree) Run(ctx context.Context, out io.Writer) error {
	verboseLog := logr.FromContextOrDiscard(ctx).V(1)
	verboseLog.Info("loading source from disk", "path", t.SourcePath)

	files, err := packageimport.Folder(ctx, t.SourcePath)
	if err != nil {
		return fmt.Errorf("loading package contents from folder: %w", err)
	}

	pkg, err := packagecontent.PackageFromFiles(ctx, cmdutil.Scheme, files)
	if err != nil {
		return fmt.Errorf("parsing package contents: %w", err)
	}

	var config map[string]interface{}
	switch {
	case len(t.ConfigPath) != 0:
		data, err := os.ReadFile(t.ConfigPath)
		if err != nil {
			return fmt.Errorf("read config from file: %w", err)
		}
		if err := yaml.Unmarshal(data, &config); err != nil {
			return fmt.Errorf("unmarshal config from file %s: %w", t.ConfigPath, err)
		}
	case len(t.ConfigTestcase) != 0:
		for _, test := range pkg.PackageManifest.Test.Template {
			if test.Name != t.ConfigTestcase {
				continue
			}
			if err := json.Unmarshal(test.Context.Config.Raw, &config); err != nil {
				return fmt.Errorf("unmarshal config from test template %s: %w", t.ConfigTestcase, err)
			}
		}

		if config == nil {
			return fmt.Errorf("%w: test template with name %s not found", cmdutil.ErrInvalidArgs, t.ConfigTestcase)
		}
	default:
	}

	templateContext := packageloader.PackageFileTemplateContext{
		Package: manifestsv1alpha1.TemplateContextPackage{
			TemplateContextObjectMeta: manifestsv1alpha1.TemplateContextObjectMeta{
				Name:      "name",
				Namespace: "namespace",
			},
		},
		Config: config,
	}
	pkgPrefix := "Package"
	scope := manifestsv1alpha1.PackageManifestScopeNamespaced
	if t.ClusterScope {
		scope = manifestsv1alpha1.PackageManifestScopeCluster
		templateContext.Package.Namespace = ""
		pkgPrefix = "ClusterPackage"
	}

	tt, err := packageloader.NewTemplateTransformer(templateContext)
	if err != nil {
		return err
	}

	l := packageloader.New(cmdutil.Scheme, packageloader.WithDefaults,
		packageloader.WithValidators(packageloader.PackageScopeValidator(scope)),
		packageloader.WithFilesTransformers(tt),
	)

	packageContent, err := l.FromFiles(ctx, files)
	if err != nil {
		return fmt.Errorf("parsing package contents: %w", err)
	}

	spec := packagecontent.TemplateSpecFromPackage(packageContent)

	pkgTree := gotree.New(
		fmt.Sprintf("%s\n%s %s",
			packageContent.PackageManifest.Name,
			pkgPrefix, client.ObjectKey{
				Name:      templateContext.Package.Name,
				Namespace: templateContext.Package.Namespace,
			}))
	for _, phase := range spec.Phases {
		treePhase := pkgTree.Add(fmt.Sprintf("Phase %s", phase.Name))

		for _, obj := range phase.Objects {
			treePhase.Add(
				fmt.Sprintf("%s %s",
					obj.Object.GroupVersionKind(),
					client.ObjectKeyFromObject(&obj.Object)))
		}
	}
	fmt.Fprint(out, pkgTree.Print())

	return nil
}

func (t *Tree) CobraCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   cmdUse,
		Short: cmdShort,
		Long:  cmdLong,
	}
	f := cmd.Flags()
	f.BoolVar(&t.ClusterScope, "cluster", false, clusterScopeUse)
	f.StringVar(&t.ConfigPath, "config-path", "", configPathUse)
	f.StringVar(&t.ConfigTestcase, "config-testcase", "", configTestcaseUse)

	cmd.RunE = func(cmd *cobra.Command, args []string) (err error) {
		if err := t.Complete(args); err != nil {
			return err
		}
		logOut := cmd.ErrOrStderr()
		log := funcr.New(func(p, a string) { fmt.Fprintln(logOut, p, a) }, funcr.Options{})
		return t.Run(logr.NewContext(cmd.Context(), log), cmd.OutOrStdout())
	}

	return cmd
}
