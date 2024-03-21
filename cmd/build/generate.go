package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"pkg.package-operator.run/cardboard/kubeutils/kubemanifests"
	"pkg.package-operator.run/cardboard/run"
	"pkg.package-operator.run/cardboard/sh"
	"sigs.k8s.io/yaml"

	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
	"package-operator.run/internal/cmd"
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

	// installYamlFile has to come after code generation.
	return mgr.ParallelDeps(
		ctx, self,
		run.Meth(g, g.docs),
		run.Meth(g, g.installYamlFile),
		run.Meth(g, g.selfBootstrapJob),
		run.Meth(g, g.selfBootstrapJobLocal),
	)
}

func (Generate) code() error {
	err := shr.New(sh.WithWorkDir("apis")).Run("controller-gen",
		"crd:crdVersions=v1,generateEmbeddedObjectMeta=true",
		"paths=./core/...",
		"output:crd:artifacts:config=../config/crds",
	)
	if err != nil {
		return fmt.Errorf("generating kubernetes manifests: %w", err)
	}

	// deepcopy generator
	err = sh.New(sh.WithWorkDir("apis")).Run("controller-gen", "object", "paths=./...")
	if err != nil {
		return fmt.Errorf("generating deep copy methods: %w", err)
	}

	err = shr.Run("controller-gen", "object", "paths=./internal/...")
	if err != nil {
		return fmt.Errorf("generating deep copy methods: %w", err)
	}

	// conversion generator
	if err := shr.Run(
		"conversion-gen", "--input-dirs", "./internal/apis/manifests",
		"--extra-peer-dirs=k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1",
		"--output-base=./",
		"--output-file-base=zz_generated.conversion",
		"-h", "/dev/null"); err != nil {
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

	// copy CRDs over to config/statis-deployment
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

func (Generate) installYamlFile(ctx context.Context) (err error) {
	self := run.Meth(&Generate{}, (&Generate{}).installYamlFile)
	if err := mgr.SerialDeps(
		ctx, self,
		run.Meth(generate, generate.code), // install.yaml generation needs code generation first
	); err != nil {
		return err
	}

	folderPath := filepath.Join("config", "static-deployment")
	outputPath := "install.yaml"

	var entries []fs.DirEntry

	entries, err = os.ReadDir(folderPath)
	if err != nil {
		return fmt.Errorf("read dir %q: %w", folderPath, err)
	}

	infoByName := map[string]fs.DirEntry{}
	names := []string{}
	for _, i := range entries {
		names = append(names, i.Name())
		infoByName[i.Name()] = i
	}

	sort.Strings(names)

	if _, err = os.Stat(outputPath); err == nil {
		err = os.Remove(outputPath)
		if err != nil {
			return fmt.Errorf("removing old file: %w", err)
		}
	}

	var outputFile *os.File

	outputFile, err = os.OpenFile(outputPath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("failed opening file: %w", err)
	}

	defer func() {
		if cErr := outputFile.Close(); err == nil {
			err = cErr
		}
	}()

	for i, name := range names {
		if infoByName[name].IsDir() {
			continue
		}

		filePath := filepath.Join(folderPath, name)
		fileYaml, err := os.ReadFile(filePath)
		cleanFileYaml := bytes.Trim(fileYaml, "-\n")
		if err != nil {
			return fmt.Errorf("reading %s: %w", filePath, err)
		}

		_, err = outputFile.Write(cleanFileYaml)
		if err != nil {
			return fmt.Errorf("failed appending manifest from file %s to output file: %w", name, err)
		}
		if i != len(names)-1 {
			_, err = outputFile.WriteString("\n---\n")
			if err != nil {
				return fmt.Errorf("failed appending --- %s to output file: %w", name, err)
			}
		} else {
			_, err = outputFile.WriteString("\n")
			if err != nil {
				return fmt.Errorf("failed appending new line %s to output file: %w", name, err)
			}
		}
	}

	return nil
}

// generates a self-bootstrap-job.yaml based on the current VERSION.
// requires the images to have been build beforehand.
func (g Generate) selfBootstrapJobLocal(context.Context) error {
	latestJobBytes, err := os.ReadFile(filepath.Join("config", "self-bootstrap-job.yaml.tpl"))
	if err != nil {
		return err
	}

	registyOverrides := imageRegistryHost() + "=dev-registry.dev-registry.svc.cluster.local:5001"
	pkoConfig := fmt.Sprintf(`{"registryHostOverrides":"%s"}`, registyOverrides)

	replacements := map[string]string{
		`##registry-overrides##`: registyOverrides,
		`##pko-config##`:         pkoConfig,
		`##pko-manager-image##`:  imageURL(imageRegistry(), "package-operator-manager", appVersion),
		`##pko-package-image##`:  imageURL(imageRegistry(), "package-operator-package", appVersion),
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
		`##registry-overrides##`: "",
		`##pko-config##`:         "",
		`##pko-manager-image##`:  imageURL(imageRegistry(), "package-operator-manager", appVersion),
		`##pko-package-image##`:  imageURL(imageRegistry(), "package-operator-package", appVersion),
	}

	latestJob := string(latestJobBytes)
	for replace, with := range replacements {
		latestJob = strings.ReplaceAll(latestJob, replace, with)
	}

	return os.WriteFile(filepath.Join("config", "self-bootstrap-job.yaml"), []byte(latestJob), os.ModePerm)
}

// PackageOperatorPackage: Includes all static-deployment files in the package-operator-package.
// This depends on an up-to-date remote-phase-package image being pushed to quay
// but we can't code this in as a dependency because then every call to this function would
// push images to quay. :(.
func (g Generate) packageOperatorPackageFiles(ctx context.Context) error {
	mngrURL := imageURL(imageRegistry(), "package-operator-manager", appVersion)

	err := filepath.WalkDir(filepath.Join("config", "static-deployment"), g.includeInPackageOperatorPackage)
	if err != nil {
		return err
	}

	pkgFolder := filepath.Join("config", "packages", "package-operator")
	manifestFile := filepath.Join(pkgFolder, "manifest.yaml")
	manifestFileContents, err := os.ReadFile(manifestFile + ".tpl")
	if err != nil {
		return err
	}
	manifest := &manifestsv1alpha1.PackageManifest{}
	if err := yaml.Unmarshal(manifestFileContents, manifest); err != nil {
		return err
	}

	manifest.Spec.Images = []manifestsv1alpha1.PackageManifestImage{
		{Name: "package-operator-manager", Image: mngrURL},
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

// Includes all static-deployment files in the hosted-cluster component of the package-operator package.
func (Generate) hostedClusterComponentFiles(ctx context.Context) error {
	pkgFolder := filepath.Join("config", "packages", "package-operator", "components", "hosted-cluster")
	manifestFile := filepath.Join(pkgFolder, "manifest.yaml")
	manifestFileContents, err := os.ReadFile(manifestFile + ".tpl")
	if err != nil {
		return err
	}
	manifest := &manifestsv1alpha1.PackageManifest{}
	if err := yaml.Unmarshal(manifestFileContents, manifest); err != nil {
		return err
	}

	manifest.Spec.Images = []manifestsv1alpha1.PackageManifestImage{
		{Name: "package-operator-manager", Image: imageURL(imageRegistry(), "package-operator-manager", appVersion)},
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

// Includes all static-deployment files in the remote-phase component of the package-operator package.
func (Generate) remotePhaseComponentFiles(ctx context.Context) error {
	pkgFolder := filepath.Join("config", "packages", "package-operator", "components", "remote-phase")
	manifestFile := filepath.Join(pkgFolder, "manifest.yaml")
	manifestFileContents, err := os.ReadFile(manifestFile + ".tpl")
	if err != nil {
		return err
	}
	manifest := &manifestsv1alpha1.PackageManifest{}
	if err := yaml.Unmarshal(manifestFileContents, manifest); err != nil {
		return err
	}

	manifest.Spec.Images = []manifestsv1alpha1.PackageManifestImage{
		{Name: "remote-phase-manager", Image: imageURL(imageRegistry(), "remote-phase-manager", appVersion)},
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

func (g Generate) includeInPackageOperatorPackage(path string, d fs.DirEntry, err error) error {
	if err != nil {
		return err
	}

	if d.IsDir() {
		return nil
	}

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
		gk := obj.GroupVersionKind().GroupKind()

		var (
			subfolder    string
			objToMarshal any
		)
		switch gk {
		case schema.GroupKind{Group: "apiextensions.k8s.io", Kind: "CustomResourceDefinition"}:
			annotations["package-operator.run/phase"] = "crds"
			subfolder = "crds"
			objToMarshal = obj.Object

		case schema.GroupKind{Group: "rbac.authorization.k8s.io", Kind: "ClusterRole"}:
			annotations["package-operator.run/phase"] = "rbac"
			subfolder = "rbac"
			objToMarshal = obj.Object

		case schema.GroupKind{Group: "apps", Kind: "Deployment"},
			schema.GroupKind{Group: "", Kind: "Namespace"},
			schema.GroupKind{Group: "rbac.authorization.k8s.io", Kind: "Role"},
			schema.GroupKind{Group: "rbac.authorization.k8s.io", Kind: "RoleBinding"},
			schema.GroupKind{Group: "rbac.authorization.k8s.io", Kind: "ClusterRoleBinding"},
			schema.GroupKind{Group: "", Kind: "ServiceAccount"}:
			continue
		}
		obj.SetAnnotations(annotations)

		yamlBytes, err := yaml.Marshal(objToMarshal)
		if err != nil {
			return err
		}

		outFilePath := filepath.Join("config", "packages", "package-operator")
		if len(subfolder) > 0 {
			outFilePath = filepath.Join(outFilePath, subfolder)
		}

		if err := os.MkdirAll(outFilePath, os.ModePerm); err != nil {
			return fmt.Errorf("creating output directory: %w", err)
		}
		outFilePath = filepath.Join(outFilePath, fmt.Sprintf("%s.%s.yaml", obj.GetName(), gk.Kind))

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
