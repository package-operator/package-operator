//go:build mage

package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-logr/logr"
	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
	"github.com/mt-sre/devkube/dev"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

	defer func() {
		os.Setenv("KUBECONFIG", locations.devEnvironment.Cluster.Kubeconfig())

		args := []string{"export", "logs", locations.IntegrationTestLogs(), "--name", clusterName}
		if err := locations.DevEnvNoInit().RunKindCommand(ctx, os.Stdout, os.Stderr, args...); err != nil {
			logger.Error(err, "exporting logs")
		}
	}()

	cluster := locations.DevEnv().Cluster

	must(cluster.CreateAndWaitFromFiles(
		ctx, []string{filepath.Join("config", "self-bootstrap-job.yaml")}))

	ctx = logr.NewContext(ctx, logger)
	// Bootstrap job is cleaning itself up after completion, so we can't wait for Condition Completed=True.
	// See self-bootstrap-job .spec.ttlSecondsAfterFinished: 0
	must(cluster.Waiter.WaitToBeGone(ctx, &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "package-operator-bootstrap",
			Namespace: "package-operator-system",
		},
	}, func(obj client.Object) (done bool, err error) { return }))

	must(cluster.Waiter.WaitForCondition(ctx, &corev1alpha1.ClusterPackage{
		ObjectMeta: metav1.ObjectMeta{
			Name: "package-operator",
		},
	}, corev1alpha1.PackageAvailable, metav1.ConditionTrue))

	d.deployTargetKubeConfig(ctx, cluster)
}

// deploy the Package Operator Manager from local files.
func (d Dev) deployPackageOperatorManager(ctx context.Context, cluster *dev.Cluster) {
	packageOperatorDeployment := templatePackageOperatorManager(cluster.Scheme)

	ctx = logr.NewContext(ctx, logger)

	// Deploy
	if err := cluster.CreateAndWaitFromFolders(ctx, []string{filepath.Join("config", "static-deployment")}); err != nil {
		panic(fmt.Errorf("deploy package-operator-manager dependencies: %w", err))
	}
	_ = cluster.CtrlClient.Delete(ctx, packageOperatorDeployment)
	if err := cluster.CreateAndWaitForReadiness(ctx, packageOperatorDeployment); err != nil {
		panic(fmt.Errorf("deploy package-operator-manager: %w", err))
	}
}

// Package Operator Webhook server from local files.
func (d Dev) deployPackageOperatorWebhook(ctx context.Context, cluster *dev.Cluster) {
	objs, err := dev.LoadKubernetesObjectsFromFile(filepath.Join("config", "deploy", "webhook", "deployment.yaml.tpl"))
	if err != nil {
		panic(fmt.Errorf("loading package-operator-webhook deployment.yaml.tpl: %w", err))
	}

	// Replace image
	packageOperatorWebhookDeployment := &appsv1.Deployment{}
	if err := cluster.Scheme.Convert(
		&objs[0], packageOperatorWebhookDeployment, nil); err != nil {
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

	// Deploy
	if err := cluster.CreateAndWaitFromFiles(ctx, []string{
		filepath.Join("config", "deploy", "webhook", "00-tls-secret.yaml"),
		filepath.Join("config", "deploy", "webhook", "service.yaml.tpl"),
		filepath.Join("config", "deploy", "webhook", "objectsetvalidatingwebhookconfig.yaml"),
		filepath.Join("config", "deploy", "webhook", "objectsetphasevalidatingwebhookconfig.yaml"),
		filepath.Join("config", "deploy", "webhook", "clusterobjectsetvalidatingwebhookconfig.yaml"),
		filepath.Join("config", "deploy", "webhook", "clusterobjectsetphasevalidatingwebhookconfig.yaml"),
	}); err != nil {
		panic(fmt.Errorf("deploy package-operator-webhook dependencies: %w", err))
	}
	_ = cluster.CtrlClient.Delete(ctx, packageOperatorWebhookDeployment)
	if err := cluster.CreateAndWaitForReadiness(ctx, packageOperatorWebhookDeployment); err != nil {
		panic(fmt.Errorf("deploy package-operator-webhook: %w", err))
	}
}

func (d Dev) deployTargetKubeConfig(ctx context.Context, cluster *dev.Cluster) {
	ctx = logr.NewContext(ctx, logger)

	var err error
	// Get Kubeconfig, will be edited for the target service account
	targetKubeconfigPath := os.Getenv("TARGET_KUBECONFIG_PATH")
	var kubeconfigBytes []byte
	if len(targetKubeconfigPath) == 0 {
		kubeconfigBuf := new(bytes.Buffer)
		args := []string{"get", "kubeconfig", "--name", clusterName, "--internal"}
		err = locations.DevEnv().RunKindCommand(ctx, kubeconfigBuf, os.Stderr, args...)
		if err != nil {
			panic(fmt.Errorf("exporting internal kubeconfig: %w", err))
		}
		kubeconfigBytes = kubeconfigBuf.Bytes()
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
	_ = cluster.CtrlClient.Delete(ctx, secret)
	if err := cluster.CtrlClient.Create(ctx, secret); err != nil {
		panic(fmt.Errorf("deploy kubeconfig secret: %w", err))
	}
}

// Remote phase manager from local files.
func (d Dev) deployRemotePhaseManager(ctx context.Context, cluster *dev.Cluster) {
	objs, err := dev.LoadKubernetesObjectsFromFile(filepath.Join("config", "remote-phase-static-deployment", "deployment.yaml.tpl"))
	if err != nil {
		panic(fmt.Errorf("loading package-operator-webhook deployment.yaml.tpl: %w", err))
	}

	// Insert new image in remote-phase-manager deployment manifest
	remotePhaseManagerDeployment := &appsv1.Deployment{}
	if err := cluster.Scheme.Convert(&objs[0], remotePhaseManagerDeployment, nil); err != nil {
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
	err = cluster.CreateAndWaitFromFolders(ctx, []string{filepath.Join("config", "remote-phase-static-deployment")})
	if err != nil {
		panic(fmt.Errorf("deploy remote-phase-manager dependencies: %w", err))
	}

	// Deploy the remote phase manager deployment
	_ = cluster.CtrlClient.Delete(ctx, remotePhaseManagerDeployment)
	if err := cluster.CreateAndWaitForReadiness(ctx, remotePhaseManagerDeployment); err != nil {
		panic(fmt.Errorf("deploy remote-phase-manager: %w", err))
	}
}

// Setup local dev environment with the package operator installed and run the integration test suite.
func (d Dev) Integration(ctx context.Context) {
	mg.SerialDeps(Dev.Deploy)

	os.Setenv("KUBECONFIG", locations.DevEnv().Cluster.Kubeconfig())

	mg.SerialCtxDeps(ctx, mg.F(Test.Integration, "package-operator"))
}

func (d Dev) loadImage(image string) error {
	mg.Deps(mg.F(Build.Image, image))

	return sh.Run(
		"crane", "push",
		locations.ImageCache(image)+".tar",
		locations.LocalImageURL(image),
	)
}

func (d Dev) init() {
	mg.Deps(Dependency.Kind, Dependency.Crane, Dependency.Helm)
}

func templatePackageOperatorManager(scheme *k8sruntime.Scheme) (deploy *appsv1.Deployment) {
	objs, err := dev.LoadKubernetesObjectsFromFile(filepath.Join("config", "static-deployment", "deployment.yaml.tpl"))
	if err != nil {
		panic(fmt.Errorf("loading package-operator-manager deployment.yaml.tpl: %w", err))
	}

	return patchPackageOperatorManager(scheme, &objs[0])
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
