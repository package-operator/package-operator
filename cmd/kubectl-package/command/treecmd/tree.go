package treecmd

import (
	"context"
	"fmt"
	"io"

	"github.com/disiqueira/gotree"
	"github.com/go-logr/logr"
	"github.com/go-logr/logr/funcr"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	pkoapis "package-operator.run/apis"
	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
	"package-operator.run/package-operator/cmd/kubectl-package/command/cmdutil"
	"package-operator.run/package-operator/internal/packages/packagebytes"
	"package-operator.run/package-operator/internal/packages/packagestructure"
)

const (
	treeUse             = "tree source_path"
	treeShort           = "outputs a logical tree view of the package contents"
	treeLong            = "outputs a logical tree view of the package by printing root->phases->objects"
	treeClusterScopeUse = "render package in cluster scope"
)

var (
	treeScheme = runtime.NewScheme()
)

func init() {
	if err := pkoapis.AddToScheme(treeScheme); err != nil {
		panic(err)
	}
}

type Tree struct {
	SourcePath   string
	ClusterScope bool
}

func (t *Tree) Complete(args []string) error {
	switch {
	case len(args) != 1:
		return fmt.Errorf("%w: got %v positional args. Need one argument containing the source path", cmdutil.ErrInvalidArgs, len(args))
	case args[0] == "":
		return fmt.Errorf("%w: source path empty", cmdutil.ErrInvalidArgs)
	}

	t.SourcePath = args[0]
	return nil
}

func (t *Tree) Run(ctx context.Context, out io.Writer) error {
	verboseLog := logr.FromContextOrDiscard(ctx).V(1)
	verboseLog.Info("loading source from disk", "path", t.SourcePath)

	templateContext := manifestsv1alpha1.TemplateContext{
		Package: manifestsv1alpha1.TemplateContextPackage{
			TemplateContextObjectMeta: manifestsv1alpha1.TemplateContextObjectMeta{
				Name:      "name",
				Namespace: "namespace",
			},
		},
	}
	pkgPrefix := "Package"
	scope := manifestsv1alpha1.PackageManifestScopeNamespaced
	if t.ClusterScope {
		scope = manifestsv1alpha1.PackageManifestScopeCluster
		templateContext.Package.Namespace = ""
		pkgPrefix = "ClusterPackage"
	}

	l := packagestructure.NewLoader(treeScheme,
		packagestructure.WithManifestValidators(
			packagestructure.PackageScopeValidator(scope),
			packagestructure.DefaultValidators,
		),
		packagestructure.WithByteTransformers(
			&packagebytes.TemplateTransformer{
				TemplateContext: templateContext,
			},
		),
	)

	packageContent, err := l.LoadFromPath(ctx, t.SourcePath)
	if err != nil {
		return fmt.Errorf("loading package contents: %w", err)
	}
	spec := packageContent.ToTemplateSpec()

	pkg := gotree.New(
		fmt.Sprintf("%s\n%s %s",
			packageContent.PackageManifest.Name,
			pkgPrefix, client.ObjectKey{
				Name:      templateContext.Package.Name,
				Namespace: templateContext.Package.Namespace,
			}))
	for _, phase := range spec.Phases {
		treePhase := pkg.Add(fmt.Sprintf("Phase %s", phase.Name))

		for _, obj := range phase.Objects {
			treePhase.Add(
				fmt.Sprintf("%s %s",
					obj.Object.GroupVersionKind(),
					client.ObjectKeyFromObject(&obj.Object)))
		}
	}
	fmt.Fprint(out, pkg.Print())

	return nil
}

func (t *Tree) CobraCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   treeUse,
		Short: treeShort,
		Long:  treeLong,
	}
	f := cmd.Flags()
	f.BoolVar(&t.ClusterScope, "cluster", false, treeClusterScopeUse)

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
