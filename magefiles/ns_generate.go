//go:build mage
// +build mage

package main

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
	"github.com/mt-sre/devkube/devos"
	"k8s.io/apimachinery/pkg/runtime/schema"
	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
	"sigs.k8s.io/yaml"
)

type Generate mg.Namespace

// Run all code generators.
// installYamlFile has to come after code generation
func (Generate) All() { mg.SerialDeps(Generate.code, Generate.docs, Generate.installYamlFile) }

func (Generate) code() {
	mg.Deps(Dependency.ControllerGen)

	args := []string{"crd:crdVersions=v1,generateEmbeddedObjectMeta=true", "paths=./core/...", "output:crd:artifacts:config=../config/crds"}
	manifestsCmd := exec.Command("controller-gen", args...)
	manifestsCmd.Dir = locations.APISubmodule()
	manifestsCmd.Stdout = os.Stdout
	manifestsCmd.Stderr = os.Stderr
	if err := manifestsCmd.Run(); err != nil {
		panic(fmt.Errorf("generating kubernetes manifests: %w", err))
	}

	// code gen
	apiCodeCmd := exec.Command("controller-gen", "object", "paths=./...")
	apiCodeCmd.Dir = locations.APISubmodule()
	if err := apiCodeCmd.Run(); err != nil {
		panic(fmt.Errorf("generating deep copy methods: %w", err))
	}

	codeCmd := exec.Command("controller-gen", "object", "paths=./internal/...")
	if err := codeCmd.Run(); err != nil {
		panic(fmt.Errorf("generating deep copy methods: %w", err))
	}

	crds, err := filepath.Glob(filepath.Join("config", "crds", "*.yaml"))
	if err != nil {
		panic(fmt.Errorf("finding CRDs: %w", err))
	}

	for _, crd := range crds {
		cmd := []string{"cp", crd, filepath.Join("config", "static-deployment", "1-"+filepath.Base(crd))}
		if err := sh.RunV(cmd[0], cmd[1:]...); err != nil {
			panic(fmt.Errorf("running %q: %w", strings.Join(cmd, " "), err))
		}
	}
}

func (Generate) docs() {
	mg.Deps(Dependency.Docgen)

	refPath := locations.APIReference()
	// Move the hack script in here.
	must(sh.RunV("bash", "-c", fmt.Sprintf("k8s-docgen apis/core/v1alpha1 > %s", refPath)))
	must(sh.RunV("bash", "-c", fmt.Sprintf("echo >> %s", refPath)))
	must(sh.RunV("bash", "-c", fmt.Sprintf("k8s-docgen apis/manifests/v1alpha1 >> %s", refPath)))
	must(sh.RunV("bash", "-c", fmt.Sprintf("echo >> %s", refPath)))
}

func (Generate) installYamlFile() {
	dumpManifestsFromFolder(filepath.Join("config", "static-deployment"), "install.yaml")
}

// Includes all static-deployment files in the package-operator-package.
func (Generate) PackageOperatorPackage() error {
	mg.Deps(
		mg.F(Build.Binary, cliCmdName, nativeArch.OS, nativeArch.Arch),
		mg.F(Build.PushImage, "package-operator-manager"),
		mg.F(Build.PushImage, remotePhasePackageName),
	)

	err := filepath.WalkDir("config/static-deployment", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		includeInPackageOperatorPackage(path, filepath.Join("config", "packages", "package-operator"))
		return nil
	})
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
		{
			Name:  "package-operator-manager",
			Image: locations.ImageURL("package-operator-manager", false),
		},
		{
			Name:  remotePhasePackageName,
			Image: locations.ImageURL(remotePhasePackageName, false),
		},
	}
	manifestYaml, err := yaml.Marshal(manifest)
	if err != nil {
		return err
	}

	if err := os.WriteFile(manifestFile, manifestYaml, os.ModePerm); err != nil {
		return err
	}

	return sh.Run("kubectl-package", "update", pkgFolder)
}

// Includes all static-deployment files in the remote-phase-package.
func (Generate) RemotePhasePackage() error {
	mg.Deps(
		mg.F(Build.Binary, cliCmdName, nativeArch.OS, nativeArch.Arch),
		mg.F(Build.PushImage, "remote-phase-manager"),
	)

	pkgFolder := filepath.Join("config", "packages", "remote-phase")
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
		{
			Name:  "remote-phase-manager",
			Image: locations.ImageURL("remote-phase-manager", false),
		},
	}
	manifestYaml, err := yaml.Marshal(manifest)
	if err != nil {
		return err
	}
	if err := os.WriteFile(manifestFile, manifestYaml, os.ModePerm); err != nil {
		return err
	}

	return sh.Run("kubectl-package", "update", pkgFolder)
}

// generates a self-bootstrap-job.yaml based on the current VERSION.
// requires the images to have been build beforehand.
func (Generate) SelfBootstrapJob() {
	latestJob, err := os.ReadFile("config/self-bootstrap-job.yaml.tpl")
	if err != nil {
		panic(err)
	}

	var (
		packageOperatorManagerImage string
		packageOperatorPackageImage string
	)
	if len(os.Getenv("USE_DIGESTS")) > 0 {
		mg.Deps(mg.F(Build.PushImage, "package-operator-manager"), mg.F(Build.PushImage, pkoPackageName))
		packageOperatorManagerImage = locations.ImageURL("package-operator-manager", true)
		packageOperatorPackageImage = locations.ImageURL(pkoPackageName, true)
	} else {
		packageOperatorManagerImage = locations.ImageURL("package-operator-manager", false)
		packageOperatorPackageImage = locations.ImageURL(pkoPackageName, false)
	}

	var (
		registyOverrides string
		pkoConfig        string
	)
	if pushToDevRegistry {
		registyOverrides = fmt.Sprintf("%s=dev-registry.dev-registry.svc.cluster.local:5001", imageRegistry)
		pkoConfig = fmt.Sprintf(`{"registryHostOverrides":"%s"}`, registyOverrides)
	}

	latestJob = bytes.ReplaceAll(latestJob, []byte(`##registry-overrides##`), []byte(registyOverrides))
	latestJob = bytes.ReplaceAll(latestJob, []byte(`##pko-config##`), []byte(pkoConfig))

	latestJob = bytes.ReplaceAll(latestJob, []byte(`##pko-manager-image##`), []byte(packageOperatorManagerImage))
	latestJob = bytes.ReplaceAll(latestJob, []byte(`##pko-package-image##`), []byte(packageOperatorPackageImage))

	must(os.WriteFile("config/self-bootstrap-job.yaml", latestJob, os.ModePerm))
}

func includeInPackageOperatorPackage(file string, outDir string) {
	objs, err := devos.UnstructuredFromFiles(devos.RealFS{}, file)
	must(err)
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

		outFilePath := outDir
		if len(subfolder) > 0 {
			outFilePath = filepath.Join(outFilePath, subfolder)
		}

		if err := os.MkdirAll(outFilePath, os.ModePerm); err != nil {
			panic(fmt.Errorf("creating output directory"))
		}
		outFilePath = filepath.Join(outFilePath, fmt.Sprintf("%s.%s.yaml", obj.GetName(), gk.Kind))

		outFile, err := os.Create(outFilePath)
		if err != nil {
			panic(fmt.Errorf("creating output file: %w", err))
		}
		defer outFile.Close()

		yamlBytes, err := yaml.Marshal(objToMarshal)
		if err != nil {
			panic(err)
		}

		if _, err := outFile.Write(yamlBytes); err != nil {
			panic(err)
		}
	}
}

// dumpManifestsFromFolder dumps all kubernets manifests from all files
// in the given folder into the output file. It does not recurse into subfolders.
// It dumps the manifests in lexical order based on file name.
func dumpManifestsFromFolder(folderPath string, outputPath string) {
	entries, err := os.ReadDir(folderPath)
	if err != nil {
		panic(fmt.Errorf("read dir %q: %w", folderPath, err))
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
			panic(fmt.Errorf("removing old file: %s", err))
		}
	}

	outputFile, err := os.OpenFile(outputPath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		panic(fmt.Errorf("failed opening file: %s", err))
	}
	defer outputFile.Close()
	for i, name := range names {
		if infoByName[name].IsDir() {
			continue
		}

		filePath := filepath.Join(folderPath, name)
		fileYaml, err := os.ReadFile(filePath)
		cleanFileYaml := bytes.Trim(fileYaml, "-\n")
		if err != nil {
			panic(fmt.Errorf("reading %s: %w", filePath, err))
		}

		_, err = outputFile.Write(cleanFileYaml)
		if err != nil {
			panic(fmt.Errorf("failed appending manifest from file %s to output file: %s", name, err))
		}
		if i != len(names)-1 {
			_, err = outputFile.WriteString("\n---\n")
			if err != nil {
				panic(fmt.Errorf("failed appending --- %s to output file: %s", name, err))
			}
		} else {
			_, err = outputFile.WriteString("\n")
			if err != nil {
				panic(fmt.Errorf("failed appending new line %s to output file: %s", name, err))
			}
		}
	}
}
