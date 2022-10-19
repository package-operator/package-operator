package main

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/yaml"

	pkoapisv1alpha1 "package-operator.run/apis/core/v1alpha1"
	pkomanifestapisv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
)

// nolint:goerr113
func processPhasesAndProbesFromPackageDir(packageDir string) ([]pkoapisv1alpha1.ObjectSetTemplatePhase, []pkoapisv1alpha1.ObjectSetProbe, error) {
	packageManifestPath := path.Join(packageDir, packageManifestFileName)
	packageManifestFile, err := os.ReadFile(packageManifestPath)
	if err != nil {
		return []pkoapisv1alpha1.ObjectSetTemplatePhase{}, []pkoapisv1alpha1.ObjectSetProbe{}, fmt.Errorf("failed to read the packageManifest file at '%s': %w", packageManifestPath, err)
	}
	log.Debug("contents of the packageManifest file successfully read")

	packageManifest := pkomanifestapisv1alpha1.PackageManifest{}
	if err := yaml.Unmarshal(packageManifestFile, &packageManifest); err != nil {
		return []pkoapisv1alpha1.ObjectSetTemplatePhase{}, []pkoapisv1alpha1.ObjectSetProbe{}, fmt.Errorf("failed to parse the YAML contents of the packageManifest file: %w", err)
	}

	resourcePaths, err := fetchPackageResourcesFromPackageDir(packageDir)
	if err != nil {
		return []pkoapisv1alpha1.ObjectSetTemplatePhase{}, []pkoapisv1alpha1.ObjectSetProbe{}, fmt.Errorf("failed to fetch the paths to all the resource YAML manifests in the packageDir '%s'", packageDir)
	}

	phaseToObjects := map[string][]pkoapisv1alpha1.ObjectSetObject{}
	for _, resourcePath := range resourcePaths {
		if resourcePath == packageManifestPath {
			continue
		}
		file, err := os.ReadFile(resourcePath)
		if err != nil {
			return []pkoapisv1alpha1.ObjectSetTemplatePhase{}, []pkoapisv1alpha1.ObjectSetProbe{}, fmt.Errorf("failed to read the resource file at '%s': %w", resourcePath, err)
		}
		resource := map[string]interface{}{}
		if err := yaml.Unmarshal(file, &resource); err != nil {
			return []pkoapisv1alpha1.ObjectSetTemplatePhase{}, []pkoapisv1alpha1.ObjectSetProbe{}, fmt.Errorf("failed to parse the YAML for the resourcefile at '%s': %w", resourcePath, err)
		}
		phaseObject := &unstructured.Unstructured{}
		phaseObject.SetUnstructuredContent(resource)
		phase, ok := phaseObject.GetAnnotations()[packageOperatorPhaseAnnotation]
		if !ok {
			return []pkoapisv1alpha1.ObjectSetTemplatePhase{}, []pkoapisv1alpha1.ObjectSetProbe{}, fmt.Errorf("'%s' annotation not found in the resource present at '%s'", packageOperatorPhaseAnnotation, resourcePath)
		}

		phaseObjectsFoundTillNow := phaseToObjects[phase]
		phaseToObjects[phase] = append(phaseObjectsFoundTillNow, pkoapisv1alpha1.ObjectSetObject{Object: *phaseObject})
	}

	phases := []pkoapisv1alpha1.ObjectSetTemplatePhase{}
	for _, packageManifestPhase := range packageManifest.Spec.Phases {
		processedObjectSetTemplatePhase := pkoapisv1alpha1.ObjectSetTemplatePhase{}
		processedObjectSetTemplatePhase.Name = packageManifestPhase.Name
		processedObjectSetTemplatePhase.Class = packageManifestPhase.Class
		processedObjectSetTemplatePhase.Objects = phaseToObjects[packageManifestPhase.Name]

		phases = append(phases, processedObjectSetTemplatePhase)
	}

	probes := packageManifest.Spec.AvailabilityProbes

	return phases, probes, nil
}

func fetchPackageResourcesFromPackageDir(packageDir string) ([]string, error) {
	resourcesManifests := []string{}
	err := filepath.Walk(packageDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		pathTokenizedBySlash := strings.Split(path, "/")
		fileName := pathTokenizedBySlash[len(pathTokenizedBySlash)-1]
		fileNameTokenizedByDot := strings.Split(fileName, ".")

		// to ignore file like /path/subpath/yaml
		if len(fileNameTokenizedByDot) < 2 {
			return nil
		}

		extension := fileNameTokenizedByDot[len(fileNameTokenizedByDot)-1]
		if extension != "yaml" && extension != "yml" {
			return nil
		}

		resourcesManifests = append(resourcesManifests, path)
		return nil
	})
	if err != nil {
		return []string{}, fmt.Errorf("failed to walk through the packageDir '%s': %w", packageDir, err)
	}
	return resourcesManifests, nil
}

// Renders and deploys an ObjectDeployment/ClusterObjectDeployment corresponding to the package.
func deployPackage(kubeClient client.Client, scope string, packageName, packageNamespace string, objectDeploymentPhases []pkoapisv1alpha1.ObjectSetTemplatePhase, objectDeploymentProbes []pkoapisv1alpha1.ObjectSetProbe, labels map[string]string) error {
	return deployPackageWithContext(context.TODO(), kubeClient, scope, packageName, packageNamespace, objectDeploymentPhases, objectDeploymentProbes, labels)
}

func deployPackageWithContext(ctx context.Context, kubeClient client.Client, scope string, packageName, packageNamespace string, objectDeploymentPhases []pkoapisv1alpha1.ObjectSetTemplatePhase, objectDeploymentProbes []pkoapisv1alpha1.ObjectSetProbe, labels map[string]string) error {
	if scope == clusterScope {
		return deployClusterObjectDeploymentWithContext(ctx, kubeClient, packageName, objectDeploymentPhases, objectDeploymentProbes, labels)
	}
	if scope == namespaceScope {
		return deployObjectDeploymentWithContext(ctx, kubeClient, packageName, packageNamespace, objectDeploymentPhases, objectDeploymentProbes, labels)
	}
	return fmt.Errorf("unknown scope '%s' found", scope) //nolint:goerr113
}

func deployObjectDeploymentWithContext(ctx context.Context, kubeClient client.Client, packageName, packageNamespace string, objectDeploymentPhases []pkoapisv1alpha1.ObjectSetTemplatePhase, objectDeploymentProbes []pkoapisv1alpha1.ObjectSetProbe, labels map[string]string) error {
	desiredObjectDeploymentResource := &pkoapisv1alpha1.ObjectDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      packageName,
			Namespace: packageNamespace,
		},
		Spec: pkoapisv1alpha1.ObjectDeploymentSpec{
			Selector: metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: pkoapisv1alpha1.ObjectSetTemplate{
				Metadata: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: pkoapisv1alpha1.ObjectSetTemplateSpec{
					Phases:             objectDeploymentPhases,
					AvailabilityProbes: objectDeploymentProbes,
				},
			},
		},
	}

	ownerPackageResource := &pkoapisv1alpha1.Package{}
	if err := kubeClient.Get(ctx, types.NamespacedName{Name: packageName, Namespace: packageNamespace}, ownerPackageResource); err != nil {
		return fmt.Errorf("failed to get the owner Package: %w", err)
	}

	foundObjectDeploymentResource := &pkoapisv1alpha1.ObjectDeployment{}
	err := kubeClient.Get(ctx, client.ObjectKeyFromObject(desiredObjectDeploymentResource), foundObjectDeploymentResource)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Debugf("ObjectDeployment '%s/%s' not found, creating it", desiredObjectDeploymentResource.Namespace, desiredObjectDeploymentResource.Name)
			if err := controllerutil.SetControllerReference(ownerPackageResource, desiredObjectDeploymentResource, kubeClient.Scheme()); err != nil {
				return fmt.Errorf("failed to set ownerRef of the packageManifest to the ObjectDeployment: %w", err)
			}
			return kubeClient.Create(ctx, desiredObjectDeploymentResource)
		}
		return err
	}
	log.Debugf("ObjectDeployment '%s/%s' already found, updating it with the desired spec", foundObjectDeploymentResource.Namespace, foundObjectDeploymentResource.Name)
	foundObjectDeploymentResource.Spec = desiredObjectDeploymentResource.Spec
	return kubeClient.Update(ctx, foundObjectDeploymentResource)
}

func deployClusterObjectDeploymentWithContext(ctx context.Context, kubeClient client.Client, packageName string, objectDeploymentPhases []pkoapisv1alpha1.ObjectSetTemplatePhase, objectDeploymentProbes []pkoapisv1alpha1.ObjectSetProbe, labels map[string]string) error {
	desiredClusterObjectDeploymentResource := &pkoapisv1alpha1.ClusterObjectDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: packageName,
		},
		Spec: pkoapisv1alpha1.ClusterObjectDeploymentSpec{
			Selector: metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: pkoapisv1alpha1.ObjectSetTemplate{
				Metadata: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: pkoapisv1alpha1.ObjectSetTemplateSpec{
					Phases:             objectDeploymentPhases,
					AvailabilityProbes: objectDeploymentProbes,
				},
			},
		},
	}

	ownerClusterPackageResource := &pkoapisv1alpha1.ClusterPackage{}
	if err := kubeClient.Get(ctx, types.NamespacedName{Name: packageName}, ownerClusterPackageResource); err != nil {
		return fmt.Errorf("failed to get the owner ClusterPackage: %w", err)
	}

	foundClusterObjectDeploymentResource := &pkoapisv1alpha1.ClusterObjectDeployment{}
	err := kubeClient.Get(ctx, client.ObjectKeyFromObject(desiredClusterObjectDeploymentResource), foundClusterObjectDeploymentResource)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Debugf("ClusterObjectDeployment '%s/%s' not found, creating it", desiredClusterObjectDeploymentResource.Namespace, desiredClusterObjectDeploymentResource.Name)
			if err := controllerutil.SetControllerReference(ownerClusterPackageResource, desiredClusterObjectDeploymentResource, kubeClient.Scheme()); err != nil {
				return fmt.Errorf("failed to set ownerRef of the packageManifest to the ClusterObjectDeployment: %w", err)
			}
			return kubeClient.Create(ctx, desiredClusterObjectDeploymentResource)
		}
		return err
	}
	log.Debugf("ClusterObjectDeployment '%s/%s' already found, updating it with the desired spec", foundClusterObjectDeploymentResource.Namespace, foundClusterObjectDeploymentResource.Name)
	foundClusterObjectDeploymentResource.Spec = desiredClusterObjectDeploymentResource.Spec
	return kubeClient.Update(ctx, foundClusterObjectDeploymentResource)
}

func ensureNamespace(kubeClient client.Client, packageNamespace string) error {
	desiredNamespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: packageNamespace}}
	return client.IgnoreAlreadyExists(kubeClient.Create(context.TODO(), desiredNamespace))
}
