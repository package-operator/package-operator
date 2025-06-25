//go:build integration

package packageoperator

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"pkg.package-operator.run/cardboard/kubeutils/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/yaml"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

var defaultNamespace = "default"

func TestObjectTemplate_creationDeletion_packages(t *testing.T) {
	cm1Key := "database"
	cm1Destination := "database"
	cm1Value := "big-database"
	cm1PatchedValue := "another-big-database"
	cm1Name := "config-map-1"
	cm1, cm1Source := createCMAndObjectTemplateSource(cm1Key, cm1Destination, cm1Value, cm1Name)

	cm2Key := "testStubImage"
	cm2Destination := "testStubImage"
	cm2Value := TestStubImage
	cm2Name := "config-map-2"
	cm2, cm2Source := createCMAndObjectTemplateSource(cm2Key, cm2Destination, cm2Value, cm2Name)

	// get kubernetes cluster version from apiserver
	versionInfo, err := DiscoveryClient.ServerVersion()
	require.NoError(t, err)

	kubernetesKey := "kubernetes"
	kubernetesPath := ".environment.kubernetes.version"
	kubernetesValue := versionInfo.String()

	template := fmt.Sprintf(`apiVersion: package-operator.run/v1alpha1
kind: Package
metadata:
  name: test-stub
  namespace: default
spec:
  image: %s
  config:
    %s: {{ .config.%s }}
    %s: {{ .config.%s }}
    %s: {{ %s }}`, SuccessTestPackageImage,
		cm1Destination, cm1Destination,
		cm2Destination, cm2Destination,
		kubernetesKey, kubernetesPath)

	objectTemplateName := "object-template"
	objectTemplate := corev1alpha1.ObjectTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      objectTemplateName,
			Namespace: defaultNamespace,
		},
		Spec: corev1alpha1.ObjectTemplateSpec{
			Template: template,
			Sources: []corev1alpha1.ObjectTemplateSource{
				cm1Source,
				cm2Source,
			},
		},
	}

	clusterTemplate := fmt.Sprintf(`apiVersion: package-operator.run/v1alpha1
kind: ClusterPackage
metadata:
  name: cluster-test-stub
spec:
  image: %s
  config:
    %s: {{ .config.%s }}
    %s: {{ .config.%s }}
    %s: {{ %s }}`, SuccessTestPackageImage,
		cm1Destination, cm1Destination,
		cm2Destination, cm2Destination,
		kubernetesKey, kubernetesPath,
	)

	clusterObjectTemplateName := "cluster-object-template"
	clusterObjectTemplate := corev1alpha1.ClusterObjectTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterObjectTemplateName,
		},
		Spec: corev1alpha1.ObjectTemplateSpec{
			Template: clusterTemplate,
			Sources: []corev1alpha1.ObjectTemplateSource{
				cm1Source,
				cm2Source,
			},
		},
	}

	deploymentTemplate := fmt.Sprintf(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx-deployment
  namespace: default
  labels:
    app: nginx
spec:
  replicas: 1
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
      - name: nginx
        env:
          - name: %s
            value: {{ .config.%s }}
        image: nginx:1.14.2
        ports:
        - containerPort: 80`, cm1Destination, cm1Destination)
	deploymentObjectTemplateName := "deployment-object-template"
	deploymentObjectTemplate := corev1alpha1.ObjectTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deploymentObjectTemplateName,
			Namespace: defaultNamespace,
		},
		Spec: corev1alpha1.ObjectTemplateSpec{
			Template: deploymentTemplate,
			Sources: []corev1alpha1.ObjectTemplateSource{
				cm1Source,
			},
		},
	}

	ctx := logr.NewContext(context.Background(), testr.New(t))
	err = Client.Create(ctx, &cm1)
	require.NoError(t, err)
	defer cleanupOnSuccess(ctx, t, &cm1)

	err = Client.Create(ctx, &cm2)
	require.NoError(t, err)
	defer cleanupOnSuccess(ctx, t, &cm2)
	err = Client.Create(ctx, &objectTemplate)
	require.NoError(t, err)
	defer cleanupOnSuccess(ctx, t, &objectTemplate)

	// Test Package
	pkg := &corev1alpha1.Package{}
	pkg.Name = "test-stub"
	pkg.Namespace = defaultNamespace

	require.NoError(t,
		Waiter.WaitForObject(
			ctx, pkg, "to be created",
			func(client.Object) (bool, error) {
				return true, nil
			}, wait.WithTimeout(20*time.Second),
		),
	)

	require.NoError(t, Client.Get(ctx, client.ObjectKeyFromObject(pkg), pkg))
	packageConfig := map[string]any{}

	require.NoError(t, yaml.Unmarshal(pkg.Spec.Config.Raw, &packageConfig))
	assert.Equal(t, cm1Value, packageConfig[cm1Destination])
	assert.Equal(t, cm2Value, packageConfig[cm2Destination])
	assert.Equal(t, kubernetesValue, packageConfig[kubernetesKey])

	// Patch config map

	patch := fmt.Sprintf(`{"data":{"%s":"%s"}}`, cm1Key, cm1PatchedValue)
	err = Client.Patch(ctx, &cm1, client.RawPatch(types.MergePatchType, []byte(patch)))
	require.NoError(t, err)

	require.NoError(t,
		Waiter.WaitForObject(
			ctx, pkg, "to get to second generation",
			func(obj client.Object) (done bool, err error) {
				waitPkg := obj.(*corev1alpha1.Package)
				return waitPkg.GetGeneration() == 2, nil
			},
		),
	)

	// check that config value was updated
	require.NoError(t, Client.Get(ctx, client.ObjectKeyFromObject(pkg), pkg))
	packageConfig2 := map[string]any{}

	require.NoError(t, yaml.Unmarshal(pkg.Spec.Config.Raw, &packageConfig2))
	assert.Equal(t, cm1PatchedValue, packageConfig2[cm1Destination])

	// Test ClusterPackage
	err = Client.Create(ctx, &clusterObjectTemplate)
	defer cleanupOnSuccess(ctx, t, &clusterObjectTemplate)
	require.NoError(t, err)
	clusterPkg := &corev1alpha1.ClusterPackage{}
	clusterPkg.Name = "cluster-test-stub"

	require.NoError(t,
		Waiter.WaitForObject(
			ctx, clusterPkg, "to be created",
			func(client.Object) (bool, error) { return true, nil },
			wait.WithTimeout(20*time.Second),
		),
	)

	require.NoError(t, Client.Get(ctx, client.ObjectKeyFromObject(clusterPkg), clusterPkg))
	clusterPackageConfig := map[string]any{}
	require.NoError(t, yaml.Unmarshal(clusterPkg.Spec.Config.Raw, &clusterPackageConfig))
	assert.Equal(t, cm1PatchedValue, clusterPackageConfig[cm1Destination])
	assert.Equal(t, cm2Value, clusterPackageConfig[cm2Destination])
	assert.Equal(t, kubernetesValue, packageConfig[kubernetesKey])

	// Test Deployment
	err = Client.Create(ctx, &deploymentObjectTemplate)
	defer cleanupOnSuccess(ctx, t, &deploymentObjectTemplate)
	require.NoError(t, err)
	deployment := &appsv1.Deployment{}
	deployment.Name = "nginx-deployment"
	deployment.Namespace = defaultNamespace

	require.NoError(t,
		Waiter.WaitForObject(ctx, deployment, "to be created", func(client.Object) (bool, error) {
			return true, nil
		}, wait.WithTimeout(20*time.Second)))

	require.NoError(t, Client.Get(ctx, client.ObjectKeyFromObject(deployment), deployment))
	envVar := deployment.Spec.Template.Spec.Containers[0].Env[0]
	assert.Equal(t, cm1PatchedValue, envVar.Value)
}

func createCMAndObjectTemplateSource(
	cmKey, cmDestination, cmValue, cmName string,
) (corev1.ConfigMap, corev1alpha1.ObjectTemplateSource) {
	cm := corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      cmName,
			Namespace: defaultNamespace,
		},
		Data: map[string]string{
			cmKey: cmValue,
		},
	}
	cmSource := corev1alpha1.ObjectTemplateSource{
		APIVersion: "v1",
		Kind:       "ConfigMap",
		Name:       cmName,
		Namespace:  defaultNamespace,
		Items: []corev1alpha1.ObjectTemplateSourceItem{
			{Key: ".data." + cmKey, Destination: "." + cmDestination},
		},
	}
	return cm, cmSource
}

const pw = "password"

func TestObjectTemplate_secretBase64Encoded(t *testing.T) {
	ctx := logr.NewContext(context.Background(), testr.New(t))
	secretName := "object-template-secret"
	secretKey := pw
	secretDestination := pw
	secretValue := "super-secret-password"
	secret := corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: defaultNamespace,
		},
		Type: corev1.SecretTypeOpaque,
		StringData: map[string]string{
			secretKey: secretValue,
		},
	}
	secretGVK, err := apiutil.GVKForObject(&secret, Scheme)
	require.NoError(t, err)
	secret.SetGroupVersionKind(secretGVK)
	secretSource := corev1alpha1.ObjectTemplateSource{
		APIVersion: "v1",
		Kind:       "Secret",
		Name:       secretName,
		Items: []corev1alpha1.ObjectTemplateSourceItem{
			{Key: ".data." + secretKey, Destination: "." + secretDestination},
		},
	}
	packageName := "test-stub-secret-test"
	template := fmt.Sprintf(`apiVersion: package-operator.run/v1alpha1
kind: Package
metadata:
  name: %s
  namespace: default
spec:
  image: %s
  config:
    testStubImage: %s
    %s: {{ b64dec .config.%s }}`,
		packageName, SuccessTestPackageImage, TestStubImage, secretDestination, secretDestination,
	)

	objectTemplateName := "object-template-password"
	objectTemplate := corev1alpha1.ObjectTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      objectTemplateName,
			Namespace: defaultNamespace,
		},
		Spec: corev1alpha1.ObjectTemplateSpec{
			Template: template,
			Sources: []corev1alpha1.ObjectTemplateSource{
				secretSource,
			},
		},
	}
	objectTemplateGVK, err := apiutil.GVKForObject(&objectTemplate, Scheme)
	require.NoError(t, err)
	objectTemplate.SetGroupVersionKind(objectTemplateGVK)

	require.NoError(t, Client.Create(ctx, &secret))
	defer cleanupOnSuccess(ctx, t, &secret)

	require.NoError(t, Client.Create(ctx, &objectTemplate))
	defer cleanupOnSuccess(ctx, t, &objectTemplate)

	pkg := &corev1alpha1.Package{}
	pkg.Name = packageName
	pkg.Namespace = defaultNamespace

	require.NoError(t,
		Waiter.WaitForObject(
			ctx, pkg, "to be created",
			func(client.Object) (bool, error) { return true, nil },
			wait.WithTimeout(20*time.Second),
		),
	)

	require.NoError(t, Client.Get(ctx, client.ObjectKeyFromObject(pkg), pkg))
	packageConfig := map[string]any{}

	require.NoError(t, yaml.Unmarshal(pkg.Spec.Config.Raw, &packageConfig))
	assert.Equal(t, secretValue, packageConfig[secretDestination])
}

func TestObjectTemplate_waitsForSource(t *testing.T) {
	ctx := logr.NewContext(context.Background(), testr.New(t))
	secretName := "secret"
	secretKey := pw
	secretDestination := pw
	secretValue := "super-secret-password"
	secret := corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: defaultNamespace,
		},
		Type: corev1.SecretTypeOpaque,
		StringData: map[string]string{
			secretKey: secretValue,
		},
	}
	secretGVK, err := apiutil.GVKForObject(&secret, Scheme)
	require.NoError(t, err)
	secret.SetGroupVersionKind(secretGVK)
	secretSource := corev1alpha1.ObjectTemplateSource{
		APIVersion: "v1",
		Kind:       "Secret",
		Name:       secretName,
		Optional:   true,
		Items: []corev1alpha1.ObjectTemplateSourceItem{
			{Key: ".data." + secretKey, Destination: "." + secretDestination},
		},
	}
	cmName := "object-template-cm-1"
	template := fmt.Sprintf(`apiVersion: v1
kind: ConfigMap
metadata:
  name: %s
data:
  test: {{ index .config "%s" | default "" | b64dec }}
`, cmName, secretDestination)

	objectTemplateName := "object-template-wait"
	objectTemplate := corev1alpha1.ObjectTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      objectTemplateName,
			Namespace: defaultNamespace,
		},
		Spec: corev1alpha1.ObjectTemplateSpec{
			Template: template,
			Sources: []corev1alpha1.ObjectTemplateSource{
				secretSource,
			},
		},
	}
	objectTemplateGVK, err := apiutil.GVKForObject(&objectTemplate, Scheme)
	require.NoError(t, err)
	objectTemplate.SetGroupVersionKind(objectTemplateGVK)

	require.NoError(t, Client.Create(ctx, &objectTemplate))
	defer cleanupOnSuccess(ctx, t, &objectTemplate)

	cm := &corev1.ConfigMap{}
	cm.Name = cmName
	cm.Namespace = defaultNamespace

	require.NoError(t,
		Waiter.WaitForObject(
			ctx, cm, "to be created",
			func(client.Object) (bool, error) { return true, nil },
			wait.WithTimeout(20*time.Second),
		),
	)

	require.NoError(t, Client.Get(ctx, client.ObjectKeyFromObject(cm), cm))
	assert.Empty(t, cm.Data["test"])

	require.NoError(t, Client.Create(ctx, &secret))
	defer cleanupOnSuccess(ctx, t, &secret)

	require.NoError(t,
		Waiter.WaitForObject(
			ctx, cm, "to be updated",
			func(obj client.Object) (done bool, err error) {
				upatedCM := obj.(*corev1.ConfigMap)
				return upatedCM.Data["test"] == secretValue, nil
			},
			wait.WithTimeout(6*time.Second),
		),
	)
}
