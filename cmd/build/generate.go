package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"pkg.package-operator.run/cardboard/kubeutils/kubemanifests"
	"pkg.package-operator.run/cardboard/run"
	"pkg.package-operator.run/cardboard/sh"
	"sigs.k8s.io/yaml"

	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
	"package-operator.run/internal/cmd"

	corev1 "k8s.io/api/core/v1"
)

// Generate is an internal collection of all code-gen functions.
type Generate struct{}

// All runs all code generators.
func (g Generate) All(ctx context.Context) error {
	self := run.Meth(g, g.All)
	if err := mgr.SerialDeps(
		ctx, self,
		run.Meth(g, g.code),
	); err != nil {
		return err
	}

	return mgr.ParallelDeps(
		ctx, self,
		run.Meth(g, g.docs),
		run.Meth(g, g.selfBootstrapJob),
		run.Meth(g, g.selfBootstrapJobLocal),
	)
}

func (Generate) code() error {
	// Generate CRD manifests.
	err := shr.New(sh.WithWorkDir("apis")).Run("controller-gen",
		"crd:crdVersions=v1,generateEmbeddedObjectMeta=true",
		"paths=./core/...",
		"output:crd:artifacts:config=../config/crds",
	)
	if err != nil {
		return fmt.Errorf("generating kubernetes manifests: %w", err)
	}

	// Generate applyconfigurations.
	err = shr.New().Run("controller-gen",
		"applyconfiguration",
		"paths=./apis/core/...",
	)
	if err != nil {
		return fmt.Errorf("generating applyconfiguration code: %w", err)
	}

	// Generate DeepCopy code.
	err = sh.New(sh.WithWorkDir("apis")).Run("controller-gen", "object", "paths=./...")
	if err != nil {
		return fmt.Errorf("generating deep copy methods: %w", err)
	}

	err = shr.Run("controller-gen", "object", "paths=./internal/...")
	if err != nil {
		return fmt.Errorf("generating deep copy methods: %w", err)
	}

	// Generate conversion methods.
	if err := shr.Run(
		"conversion-gen",
		"--extra-peer-dirs=k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1",
		"--output-file=zz_generated.conversion.go", "./internal/apis/manifests",
	); err != nil {
		return (fmt.Errorf("generating conversion methods: %w", err))
	}
	// conversion-gen expects the SchemeBuilder to be called "localSchemeBuilder"
	conversionFilePath := "./internal/apis/manifests/zz_generated.conversion.go"
	conversionFile, err := os.ReadFile(conversionFilePath)
	if err != nil {
		return (fmt.Errorf("reading zz_generated.conversion.go file: %w", err))
	}
	conversionFile = bytes.Replace(conversionFile, []byte(`localSchemeBuilder`), []byte(`SchemeBuilder`), 1)
	if err := os.WriteFile(conversionFilePath, conversionFile, os.ModePerm); err != nil {
		return (fmt.Errorf("writing zz_generated.conversion.go file: %w", err))
	}

	// Copy CRDs over to config/static-deployment.
	crds, err := filepath.Glob(filepath.Join("config", "crds", "*.yaml"))
	if err != nil {
		return fmt.Errorf("finding CRDs: %w", err)
	}

	for _, crd := range crds {
		err := shr.Copy(filepath.Join("config", "static-deployment", "1-"+filepath.Base(crd)), crd)
		if err != nil {
			return fmt.Errorf("copy: %w", err)
		}
	}

	return nil
}

func (Generate) docs() error {
	refPath := filepath.Join("docs", "api-reference.md")
	return shr.Bash(
		"k8s-docgen apis/core/v1alpha1 > "+refPath,
		"echo >> "+refPath,
		"k8s-docgen apis/manifests/v1alpha1 >> "+refPath,
		"echo >> "+refPath,
	)
}

// Generates a self-bootstrap-job.yaml based on the current VERSION.
// Requires the images to have been build beforehand.
func (g Generate) selfBootstrapJobLocal(context.Context) error {
	latestJobBytes, err := os.ReadFile(filepath.Join("config", "self-bootstrap-job.yaml.tpl"))
	if err != nil {
		return err
	}

	type cfg struct {
		RegistryHostOverrides                       string              `json:"registryHostOverrides"`
		ImagePrefixOverrides                        string              `json:"imagePrefixOverrides"`
		ObjectTemplateOptionalResourceRetryInterval string              `json:"objectTemplateOptionalResourceRetryInterval"`
		ObjectTemplateResourceRetryInterval         string              `json:"objectTemplateResourceRetryInterval"`
		SubcomponentAffinity                        corev1.Affinity     `json:"subcomponentAffinity,omitempty"`
		SubcomponentTolerations                     []corev1.Toleration `json:"subcomponentTolerations,omitempty"`
	}

	registyOverrides := imageRegistryHost() + "=dev-registry.dev-registry.svc.cluster.local:5001"
	imagePrefixOverrides := imageRegistry() + "/src/=" + imageRegistry() + "/mirror/"
	cfgBytes, err := json.Marshal(cfg{
		ObjectTemplateResourceRetryInterval:         "2s",
		ObjectTemplateOptionalResourceRetryInterval: "4s",
		RegistryHostOverrides:                       registyOverrides,
		// Affinity for int test TestHyperShift/SubcomponentTolerationsAffinity
		SubcomponentAffinity: corev1.Affinity{
			NodeAffinity: &corev1.NodeAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
					NodeSelectorTerms: []corev1.NodeSelectorTerm{
						{
							MatchExpressions: []corev1.NodeSelectorRequirement{
								{Key: "hypershift-affinity-test-label", Operator: "Exists"},
							},
						},
					},
				},
			},
		},
		SubcomponentTolerations: []corev1.Toleration{
			{Effect: "NoSchedule", Key: "node-role.kubernetes.io/infra"},
		},
		ImagePrefixOverrides: imagePrefixOverrides,
	})
	if err != nil {
		return err
	}

	replacements := map[string]string{
		`##image-prefix-overrides##`: imagePrefixOverrides,
		`##registry-overrides##`:     registyOverrides,
		`##pko-config##`:             string(cfgBytes),
		`##pko-manager-image##`:      imageURL(imageRegistry(), "package-operator-manager", appVersion),
		`##pko-package-image##`:      imageURL(imageRegistry(), "package-operator-package", appVersion),
	}

	latestJob := string(latestJobBytes)
	for replace, with := range replacements {
		latestJob = strings.ReplaceAll(latestJob, replace, with)
	}

	return os.WriteFile(filepath.Join("config", "self-bootstrap-job-local.yaml"), []byte(latestJob), os.ModePerm)
}

func (g Generate) selfBootstrapJob(context.Context) error {
	latestJobBytes, err := os.ReadFile(filepath.Join("config", "self-bootstrap-job.yaml.tpl"))
	if err != nil {
		return err
	}

	replacements := map[string]string{
		`##image-prefix-overrides##`: "",
		`##registry-overrides##`:     "",
		`##pko-config##`:             "",
		`##pko-manager-image##`:      imageURL(imageRegistry(), "package-operator-manager", appVersion),
		`##pko-package-image##`:      imageURL(imageRegistry(), "package-operator-package", appVersion),
	}

	latestJob := string(latestJobBytes)
	for replace, with := range replacements {
		latestJob = strings.ReplaceAll(latestJob, replace, with)
	}

	return os.WriteFile(filepath.Join("config", "self-bootstrap-job.yaml"), []byte(latestJob), os.ModePerm)
}

func (g Generate) selfBootstrapJobHelm(context.Context) error {
	latestJobBytes, err := os.ReadFile(filepath.Join("config", "self-bootstrap-job.yaml.tpl"))
	if err != nil {
		return err
	}

	replacements := map[string]string{
		`##image-prefix-overrides##`: "",
		`##registry-overrides##`:     "",
		`##pko-config##`:             "{{ .Values.Config | mustToJson }}",
		`##pko-manager-image##`:      `{{ get .Values.Images "package-operator-manager" }}`,
		`##pko-package-image##`:      `{{ get .Values.Images "package-operator-package" }}`,
	}

	latestJob := string(latestJobBytes)
	for replace, with := range replacements {
		latestJob = strings.ReplaceAll(latestJob, replace, with)
	}

	return os.WriteFile(
		filepath.Join(cacheDir, "chart", "templates", "self-bootstrap-job.yaml"), []byte(latestJob), os.ModePerm)
}

func (g Generate) helmValuesYaml() error {
	valuesBytes, err := os.ReadFile(filepath.Join("config", "chart", "values.yaml.tpl"))
	if err != nil {
		return err
	}

	replacements := map[string]string{
		`##pko-manager-image##`: imageURL(imageRegistry(), "package-operator-manager", appVersion),
		`##pko-package-image##`: imageURL(imageRegistry(), "package-operator-package", appVersion),
	}

	values := string(valuesBytes)
	for replace, with := range replacements {
		values = strings.ReplaceAll(values, replace, with)
	}

	return os.WriteFile(
		filepath.Join(cacheDir, "chart", "values.yaml"), []byte(values), os.ModePerm)
}

// PackageOperatorPackage: Includes all static-deployment files in the package-operator-package.
func (g Generate) packageOperatorPackageFiles(ctx context.Context) error {
	pkgFolder := filepath.Join("config", "packages", "package-operator")
	images := map[string]string{
		"package-operator-manager": imageURL(imageRegistry(), "package-operator-manager", appVersion),
	}
	err := filepath.WalkDir(filepath.Join("config", "static-deployment"), g.includeInPackageOperatorPackage)
	if err != nil {
		return err
	}
	return g.manifestFileFromTemplate(ctx, pkgFolder, images)
}

// Includes all static-deployment files in the remote-phase component.
func (g Generate) remotePhaseComponentFiles(ctx context.Context) error {
	pkgFolder := filepath.Join("config", "packages", "package-operator", "components", "remote-phase")
	images := map[string]string{
		"remote-phase-manager": imageURL(imageRegistry(), "remote-phase-manager", appVersion),
	}
	return g.manifestFileFromTemplate(ctx, pkgFolder, images)
}

// Includes all static-deployment files in the hosted-cluster component.
func (g Generate) hostedClusterComponentFiles(ctx context.Context) error {
	pkgFolder := filepath.Join("config", "packages", "package-operator", "components", "hosted-cluster")
	images := map[string]string{
		"package-operator-manager": imageURL(imageRegistry(), "package-operator-manager", appVersion),
	}
	err := filepath.WalkDir(filepath.Join("config", "static-deployment"), g.includeInHostedClusterComponent)
	if err != nil {
		return err
	}
	return g.manifestFileFromTemplate(ctx, pkgFolder, images)
}

func (g Generate) manifestFileFromTemplate(ctx context.Context, pkgFolder string, images map[string]string) error {
	manifestFile := filepath.Join(pkgFolder, "manifest.yaml")
	manifestFileContents, err := os.ReadFile(manifestFile + ".tpl")
	if err != nil {
		return err
	}
	manifest := &manifestsv1alpha1.PackageManifest{}
	if err := yaml.Unmarshal(manifestFileContents, manifest); err != nil {
		return err
	}

	manifest.Spec.Images = []manifestsv1alpha1.PackageManifestImage{}
	for k, v := range images {
		manifest.Spec.Images = append(manifest.Spec.Images, manifestsv1alpha1.PackageManifestImage{Name: k, Image: v})
	}

	manifestYaml, err := yaml.Marshal(manifest)
	if err != nil {
		return err
	}

	if err := os.WriteFile(manifestFile, manifestYaml, os.ModePerm); err != nil {
		return err
	}

	err = cmd.Update{}.UpdateLockData(ctx, pkgFolder)
	if err == nil || errors.Is(err, cmd.ErrLockDataUnchanged) {
		return nil
	}

	return err
}

type includeTransform func(obj *unstructured.Unstructured) (
	skip bool, annotations map[string]string, subfolder string, objToMarshal any)

func (g Generate) includeInPackageOperatorPackage(path string, d fs.DirEntry, err error) error {
	if err != nil {
		return err
	}
	if d.IsDir() {
		return nil
	}
	return g.includeInPackage(path, filepath.Join("config", "packages", "package-operator"),
		func(obj *unstructured.Unstructured) (
			skip bool, annotations map[string]string, subfolder string, objToMarshal any,
		) {
			switch obj.GroupVersionKind().GroupKind() {
			case schema.GroupKind{Group: "apiextensions.k8s.io", Kind: "CustomResourceDefinition"}:
				annotations = map[string]string{"package-operator.run/phase": "crds"}
				subfolder = "crds"
				objToMarshal = obj.Object

			case schema.GroupKind{Group: "rbac.authorization.k8s.io", Kind: "ClusterRole"}:
				annotations = map[string]string{"package-operator.run/phase": "rbac"}
				subfolder = "rbac"
				objToMarshal = obj.Object

			default:
				skip = true
			}
			return
		},
	)
}

func (g Generate) includeInHostedClusterComponent(path string, d fs.DirEntry, err error) error {
	if err != nil {
		return err
	}
	if d.IsDir() {
		return nil
	}
	return g.includeInPackage(
		path,
		filepath.Join("config", "packages", "package-operator", "components", "hosted-cluster"),
		func(obj *unstructured.Unstructured) (
			skip bool, annotations map[string]string, subfolder string, objToMarshal any,
		) {
			switch obj.GroupVersionKind().GroupKind() {
			case schema.GroupKind{Group: "apiextensions.k8s.io", Kind: "CustomResourceDefinition"}:
				annotations = map[string]string{"package-operator.run/phase": "crds"}
				subfolder = "crds"
				objToMarshal = obj.Object

			default:
				skip = true
			}
			return
		},
	)
}

func (g Generate) includeInPackage(path string, outFilePath string, transform includeTransform) error {
	fileContent, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	objs, err := kubemanifests.LoadKubernetesObjectsFromBytes(fileContent)
	if err != nil {
		return err
	}
	for _, obj := range objs {
		if len(obj.Object) == 0 {
			continue
		}

		annotations := obj.GetAnnotations()
		if annotations == nil {
			annotations = map[string]string{}
		}

		skip, ann, subfolder, objToMarshal := transform(&obj)
		if skip {
			continue
		}

		for k, v := range ann {
			annotations[k] = v
		}

		obj.SetAnnotations(annotations)

		yamlBytes, err := yaml.Marshal(objToMarshal)
		if err != nil {
			return err
		}

		if len(subfolder) > 0 {
			outFilePath = filepath.Join(outFilePath, subfolder)
		}

		if err := os.MkdirAll(outFilePath, os.ModePerm); err != nil {
			return fmt.Errorf("creating output directory: %w", err)
		}
		outFileName := fmt.Sprintf("%s.%s.yaml", obj.GetName(), obj.GroupVersionKind().GroupKind().Kind)
		outFilePath = filepath.Join(outFilePath, outFileName)

		outFile, err := os.Create(outFilePath)
		if err != nil {
			panic(fmt.Errorf("creating output file: %w", err))
		}

		_, wErr := outFile.Write(yamlBytes)
		cErr := outFile.Close()

		switch {
		case wErr != nil:
			return wErr
		case cErr != nil:
			return cErr
		}
	}

	return nil
}

func (g Generate) generateChartYaml(outFilePath, chartVersion, appVersion string) error {
	chartBytes, err := os.ReadFile(filepath.Join("config", "chart", "Chart.yaml.tpl"))
	if err != nil {
		return err
	}

	replacements := map[string]string{
		`##chart-version##`: chartVersion,
		`##app-version##`:   appVersion,
	}

	chart := string(chartBytes)
	for replace, with := range replacements {
		chart = strings.ReplaceAll(chart, replace, with)
	}

	return os.WriteFile(outFilePath, []byte(chart), os.ModePerm)
}

func (g Generate) templateManifestFiles(ctx context.Context, templatedPackage string) error {
	templateBytes, err := os.ReadFile(filepath.Join("config", "packages", templatedPackage, "manifest.yaml.tpl"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// Not a templated package so no need to template
			return nil
		}
		return err
	}

	replacements := map[string]string{
		`##image-registry##`: imageRegistry(),
		`##tag##`:            appVersion,
	}

	template := string(templateBytes)
	for replace, with := range replacements {
		template = strings.ReplaceAll(template, replace, with)
	}

	err = os.WriteFile(filepath.Join("config", "packages", templatedPackage, "manifest.yaml"),
		[]byte(template), os.ModePerm)
	if err != nil {
		return err
	}

	err = cmd.Update{}.UpdateLockData(ctx, filepath.Join("config", "packages", templatedPackage))
	if err == nil || errors.Is(err, cmd.ErrLockDataUnchanged) {
		return nil
	}

	return err
}
