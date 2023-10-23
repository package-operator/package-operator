//go:build mage

package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-logr/logr"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/magefile/mage/mg"
	"github.com/mt-sre/devkube/devcluster"
	"github.com/mt-sre/devkube/devhelm"
	"github.com/mt-sre/devkube/devkind"
	"github.com/mt-sre/devkube/devos"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	kindv1alpha4 "sigs.k8s.io/kind/pkg/apis/config/v1alpha4"
)

type Dev mg.Namespace

// Load images into the development environment.
func (d Dev) Load() {
	pushToDevRegistry = true
	os.Setenv("PKO_REPOSITORY_HOST", "localhost:5001")

	// setup is a pre-requisite and needs to run before we can load images.
	mg.SerialDeps(Dev.Setup)
	images := []string{
		"package-operator-manager", "package-operator-webhook",
		"remote-phase-manager", "test-stub", "test-stub-package",
		remotePhasePackageName, pkoPackageName,
	}
	deps := make([]any, len(images))
	for i := range images {
		deps[i] = mg.F(Build.PushImage, images[i])
	}
	mg.Deps(deps...)

	mg.SerialDeps(Generate.SelfBootstrapJob)

	// Print all Loaded images, so we can reference them manually.
	fmt.Println("----------------------------")
	fmt.Println("loaded images into kind cluster:")
	for i := range images {
		fmt.Println(locations.ImageURL(images[i], false))
	}
	fmt.Println("----------------------------")
}

// Setup local cluster and deploy the Package Operator.
func (d Dev) Deploy(ctx context.Context) {
	mg.SerialDeps(Dev.Load)

	kind := devkind.Kind{Provider: locations.ContainerRuntime(ctx).KindProvider()}
	cluster, err := kind.GetKindCluster(devClusterName)

	defer func() {
		os.Setenv("KUBECONFIG", filepath.Join(locations.KindCache(), devClusterName))

		if err != nil {
			nodes, err := cluster.ListNodes()
			must(err)
			buf := bytes.Buffer{}
			for _, node := range nodes {
				must(node.SerialLogs(&buf))
			}

			must(os.WriteFile(locations.IntegrationTestLogs(), buf.Bytes(), 0o600))
		}
	}()

	objs, err := devos.UnstructuredFromFiles(nil, filepath.Join("config", "self-bootstrap-job.yaml"))
	must(err)
	must(cluster.CreateAndAwaitReadiness(ctx, devos.ObjectsFromUnstructured(objs)...))

	ctx = logr.NewContext(ctx, logger)
	// Bootstrap job is cleaning itself up after completion, so we can't wait for Condition Completed=True.
	// See self-bootstrap-job .spec.ttlSecondsAfterFinished: 0
	boostrapJob := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: "package-operator-bootstrap", Namespace: "package-operator-system"}}
	must(cluster.Poller.Wait(ctx, cluster.Checker.CheckGone(cluster.Cli, boostrapJob)))

	obj := &corev1alpha1.ClusterPackage{ObjectMeta: metav1.ObjectMeta{Name: "package-operator"}}
	packageAvailableCheck := cluster.Checker.ObjCheckStatusConditionIs(corev1alpha1.PackageAvailable, metav1.ConditionTrue)
	must(cluster.Poller.Wait(ctx, cluster.Checker.CheckObj(cluster.Cli, obj, packageAvailableCheck)))

	d.deployTargetKubeConfig(ctx, cluster.Cluster)
}

// deploy the Package Operator Manager from local files.
func (d Dev) deployPackageOperatorManager(ctx context.Context, cluster devcluster.Cluster) {
	packageOperatorDeployment := templatePackageOperatorManager(cluster.Cli.Scheme())

	ctx = logr.NewContext(ctx, logger)

	objs, err := devos.UnstructuredFromFolder(nil, filepath.Join("config", "static-deployment"))
	must(err)

	must(cluster.CreateAndAwaitReadiness(ctx, devos.ObjectsFromUnstructured(objs)...))

	_ = cluster.Cli.Delete(ctx, packageOperatorDeployment)

	must(cluster.CreateAndAwaitReadiness(ctx, packageOperatorDeployment))
}

// Package Operator Webhook server from local files.
func (d Dev) deployPackageOperatorWebhook(ctx context.Context, cluster devcluster.Cluster) {
	objs, err := devos.UnstructuredFromFiles(nil, filepath.Join("config", "deploy", "webhook", "deployment.yaml.tpl"))
	must(err)

	// Replace image
	packageOperatorWebhookDeployment := &appsv1.Deployment{}
	if err := cluster.Cli.Scheme().Convert(&objs[0], packageOperatorWebhookDeployment, nil); err != nil {
		panic(fmt.Errorf("converting to Deployment: %w", err))
	}
	packageOperatorWebhookImage := os.Getenv("PACKAGE_OPERATOR_WEBHOOK_IMAGE")
	if len(packageOperatorWebhookImage) == 0 {
		packageOperatorWebhookImage = locations.ImageURL("package-operator-webhook", false)
	}
	for i := range packageOperatorWebhookDeployment.Spec.Template.Spec.Containers {
		container := &packageOperatorWebhookDeployment.Spec.Template.Spec.Containers[i]

		switch container.Name {
		case "webhook":
			container.Image = packageOperatorWebhookImage
		}
	}

	ctx = logr.NewContext(ctx, logger)

	objs, err = devos.UnstructuredFromFiles(nil,
		filepath.Join("config", "deploy", "webhook", "00-tls-secret.yaml"),
		filepath.Join("config", "deploy", "webhook", "service.yaml.tpl"),
		filepath.Join("config", "deploy", "webhook", "objectsetvalidatingwebhookconfig.yaml"),
		filepath.Join("config", "deploy", "webhook", "objectsetphasevalidatingwebhookconfig.yaml"),
		filepath.Join("config", "deploy", "webhook", "clusterobjectsetvalidatingwebhookconfig.yaml"),
		filepath.Join("config", "deploy", "webhook", "clusterobjectsetphasevalidatingwebhookconfig.yaml"),
	)
	must(err)
	must(cluster.CreateAndAwaitReadiness(ctx, devos.ObjectsFromUnstructured(objs)...))

	_ = cluster.Cli.Delete(ctx, packageOperatorWebhookDeployment)
	must(cluster.CreateAndAwaitReadiness(ctx, packageOperatorWebhookDeployment))
}

func (d Dev) deployTargetKubeConfig(ctx context.Context, cluster devcluster.Cluster) {
	ctx = logr.NewContext(ctx, logger)

	var err error
	// Get Kubeconfig, will be edited for the target service account
	targetKubeconfigPath := os.Getenv("TARGET_KUBECONFIG_PATH")
	var kubeconfigBytes []byte
	if len(targetKubeconfigPath) == 0 {
		kind := devkind.Kind{Provider: locations.ContainerRuntime(ctx).KindProvider()}
		cluster, err := kind.GetKindCluster(devClusterName)
		must(err)
		kubeconfig, err := cluster.Kubeconfig(true)
		must(err)
		kubeconfigBytes = []byte(kubeconfig)
		old := []byte("package-operator-dev-control-plane:6443")
		new := []byte("kubernetes.default")
		kubeconfigBytes = bytes.Replace(kubeconfigBytes, old, new, -1) // use in-cluster DNS
	} else {
		kubeconfigBytes, err = os.ReadFile(targetKubeconfigPath)
		if err != nil {
			panic(fmt.Errorf("reading in kubeconfig: %w", err))
		}
	}

	// Create a new secret for the kubeconfig
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "service-network-admin-kubeconfig",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"kubeconfig": kubeconfigBytes,
		},
	}

	// Deploy the secret with the new kubeconfig
	_ = cluster.Cli.Delete(ctx, secret)
	if err := cluster.Cli.Create(ctx, secret); err != nil {
		panic(fmt.Errorf("deploy kubeconfig secret: %w", err))
	}
}

// Remote phase manager from local files.
func (d Dev) deployRemotePhaseManager(ctx context.Context, cluster devcluster.Cluster) {
	objs, err := devos.UnstructuredFromFiles(nil, filepath.Join("config", "remote-phase-static-deployment", "deployment.yaml.tpl"))
	must(err)

	// Insert new image in remote-phase-manager deployment manifest
	remotePhaseManagerDeployment := &appsv1.Deployment{}
	if err := cluster.Cli.Scheme().Convert(&objs[0], remotePhaseManagerDeployment, nil); err != nil {
		panic(fmt.Errorf("converting to Deployment: %w", err))
	}
	packageOperatorWebhookImage := os.Getenv("REMOTE_PHASE_MANAGER_IMAGE")
	if len(packageOperatorWebhookImage) == 0 {
		packageOperatorWebhookImage = locations.ImageURL("remote-phase-manager", false)
	}
	for i := range remotePhaseManagerDeployment.Spec.Template.Spec.Containers {
		container := &remotePhaseManagerDeployment.Spec.Template.Spec.Containers[i]

		switch container.Name {
		case "manager":
			container.Image = packageOperatorWebhookImage
		}
	}

	d.deployTargetKubeConfig(ctx, cluster)

	// Beware: CreateAndWaitFromFolders doesn't update anything
	// Create the service accounts and related dependencies
	objs, err = devos.UnstructuredFromFolder(nil, filepath.Join("config", "remote-phase-static-deployment"))
	must(err)
	must(cluster.CreateAndAwaitReadiness(ctx, devos.ObjectsFromUnstructured(objs)...))

	// Deploy the remote phase manager deployment
	_ = cluster.Cli.Delete(ctx, remotePhaseManagerDeployment)
	must(cluster.CreateAndAwaitReadiness(ctx, remotePhaseManagerDeployment))
}

// Setup local dev environment with the package operator installed and run the integration test suite.
func (d Dev) Integration(ctx context.Context) {
	mg.SerialDeps(Dev.Deploy)

	os.Setenv("KUBECONFIG", filepath.Join(locations.KindCache(), devClusterName))

	mg.SerialCtxDeps(ctx, mg.F(Test.Integration, "package-operator"))
}

func (d Dev) loadImage(image string) {
	mg.Deps(mg.F(Build.Image, image))
	img, err := crane.Load(locations.ImageCache(image) + ".tar")
	must(err)
	must(crane.Push(img, locations.LocalImageURL(image)))
}

func (d Dev) init() {
	mg.Deps(Dependency.Kind, Dependency.Helm)
}

func templatePackageOperatorManager(scheme *k8sruntime.Scheme) (deploy *appsv1.Deployment) {
	objs, err := devos.UnstructuredFromFiles(nil, filepath.Join("config", "static-deployment", "deployment.yaml.tpl"))
	must(err)

	return patchPackageOperatorManager(scheme, objs[0])
}

func patchPackageOperatorManager(scheme *k8sruntime.Scheme, obj *unstructured.Unstructured) (deploy *appsv1.Deployment) {
	// Replace image
	packageOperatorDeployment := &appsv1.Deployment{}
	if err := scheme.Convert(
		obj, packageOperatorDeployment, nil); err != nil {
		panic(fmt.Errorf("converting to Deployment: %w", err))
	}

	var (
		packageOperatorManagerImage string
		remotePhasePackageImage     string
	)
	if len(os.Getenv("USE_DIGESTS")) > 0 {
		// To use digests the image needs to be pushed to a registry first.
		mg.Deps(
			mg.F(Build.PushImage, "package-operator-manager"),
			mg.F(Build.PushImage, remotePhasePackageName),
		)
		packageOperatorManagerImage = locations.ImageURL("package-operator-manager", true)
		remotePhasePackageImage = locations.ImageURL(remotePhasePackageName, true)
	} else {
		packageOperatorManagerImage = locations.ImageURL("package-operator-manager", false)
		remotePhasePackageImage = locations.ImageURL(remotePhasePackageName, false)
	}

	for i := range packageOperatorDeployment.Spec.Template.Spec.Containers {
		container := &packageOperatorDeployment.Spec.Template.Spec.Containers[i]

		switch container.Name {
		case "manager":
			container.Image = packageOperatorManagerImage

			for j := range container.Env {
				env := &container.Env[j]
				switch env.Name {
				case "PKO_IMAGE":
					env.Value = packageOperatorManagerImage
				case "PKO_REMOTE_PHASE_PACKAGE_IMAGE":
					env.Value = remotePhasePackageImage
				}
			}
		}
	}

	return packageOperatorDeployment
}

// Creates an empty development environment via kind.
func (d Dev) Setup(ctx context.Context) {
	mg.SerialDeps(Dev.init)

	containerRuntime := locations.ContainerRuntime(ctx)
	kind := devkind.Kind{Provider: containerRuntime.KindProvider()}

	cluster, err := kind.CreateOrRecreateKindCluster(
		devClusterName,
		filepath.Join(locations.KindCache(), devClusterName, "kubeconfig"),
		kindv1alpha4.Cluster{
			ContainerdConfigPatches: []string{
				// Replace `imageRegistry` with our local dev-registry.
				fmt.Sprintf(`[plugins."io.containerd.grpc.v1.cri".registry.mirrors."%s"]\nendpoint = ["http://localhost:31320"]`, imageRegistry),
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
		},
	)
	must(err)
	must(corev1alpha1.AddToScheme(cluster.Cli.Scheme()))

	if _, isCI := os.LookupEnv("CI"); !isCI {
		helm := devhelm.RealHelm{}
		// don't install the monitoring stack in CI to speed up tests.
		helm.RepoAdd(ctx, "prometheus-community", "https://prometheus-community.github.io/helm-charts")
		must(helm.Install(ctx,
			"prometheus",
			"prometheus-community/kube-prometheus-stack",
			devhelm.InstallWithNamespace("monitoring"),
			devhelm.InstallWithSet("grafana.enabled=true,kubeStateMetrics.enabled=false,nodeExporter.enabled=false"),
		))

		objs, err := devos.UnstructuredFromFiles(nil, "config/service-monitor.yaml")
		must(err)
		err = cluster.CreateAndAwaitReadiness(ctx, devos.ObjectsFromUnstructured(objs)...)
		must(err)
	}

	objs, err := devos.UnstructuredFromFiles(nil, "config/local-registry.yaml")
	must(err)
	err = cluster.CreateAndAwaitReadiness(ctx, devos.ObjectsFromUnstructured(objs)...)
	must(err)
}

// Tears the whole kind development environment down.
func (d Dev) Teardown(ctx context.Context) {
	mg.SerialDeps(Dev.init)

	containerRuntime := locations.ContainerRuntime(ctx)
	kind := devkind.Kind{Provider: containerRuntime.KindProvider()}
	must(kind.DeleteKindClusterByName(devClusterName, filepath.Join(locations.KindCache(), devClusterName)))
}
