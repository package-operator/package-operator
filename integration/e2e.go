// Package integration contains the Addon Operator integration tests.
package integration

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"testing"

	operatorsv1 "github.com/operator-framework/api/pkg/operators/v1"
	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	aoapis "github.com/openshift/addon-operator/apis"
)

const (
	relativeConfigDeployPath = "../config/deploy"
)

var (
	// Client pointing to the e2e test cluster.
	Client client.Client
	Config *rest.Config
	Scheme = runtime.NewScheme()

	// Typed K8s Clients
	CoreV1Client corev1client.CoreV1Interface

	// Path to the deployment configuration directory.
	PathConfigDeploy string
)

func init() {
	// Client/Scheme setup.
	AddToSchemes := runtime.SchemeBuilder{
		clientgoscheme.AddToScheme,
		aoapis.AddToScheme,
		apiextensionsv1.AddToScheme,
		operatorsv1.AddToScheme,
		operatorsv1alpha1.AddToScheme,
	}
	if err := AddToSchemes.AddToScheme(Scheme); err != nil {
		panic(fmt.Errorf("could not load schemes: %w", err))
	}

	Config = ctrl.GetConfigOrDie()

	var err error
	Client, err = client.New(Config, client.Options{
		Scheme: Scheme,
	})
	if err != nil {
		panic(err)
	}

	// Typed Kubernetes Clients
	CoreV1Client = corev1client.NewForConfigOrDie(Config)

	// Paths
	PathConfigDeploy, err = filepath.Abs(relativeConfigDeployPath)
	if err != nil {
		panic(err)
	}
}

// Load all k8s objects from .yaml files in config/deploy.
// File/Object order is preserved.
func LoadObjectsFromDeploymentFiles(t *testing.T) []unstructured.Unstructured {
	configDeploy, err := os.Open(PathConfigDeploy)
	require.NoError(t, err)

	files, err := configDeploy.Readdir(-1)
	require.NoError(t, err)

	var objects []unstructured.Unstructured
	for _, f := range files {
		if f.IsDir() {
			continue
		}
		if path.Ext(f.Name()) != ".yaml" {
			continue
		}

		fileYaml, err := ioutil.ReadFile(path.Join(
			PathConfigDeploy, f.Name()))
		require.NoError(t, err)

		// Trim empty starting and ending objects
		fileYaml = bytes.Trim(fileYaml, "---\n")

		// Split for every included yaml document.
		for _, yamlDocument := range bytes.Split(fileYaml, []byte("---\n")) {
			obj := unstructured.Unstructured{}
			require.NoError(t, yaml.Unmarshal(yamlDocument, &obj))

			objects = append(objects, obj)
		}
	}

	return objects
}

// Prints the phase of a pod together with the logs of every container.
func PrintPodStatusAndLogs(namespace string) error {
	ctx := context.Background()

	pods := &corev1.PodList{}
	if err := Client.List(ctx, pods, client.InNamespace(namespace)); err != nil {
		return err
	}

	for _, pod := range pods.Items {
		if err := reportPodStatus(ctx, &pod); err != nil {
			return err
		}
	}
	return nil
}

func reportPodStatus(ctx context.Context, pod *corev1.Pod) error {
	fmt.Println("-----------------------------------------------------------")
	fmt.Printf("Pod %s: %s\n", client.ObjectKeyFromObject(pod), pod.Status.Phase)
	fmt.Println("-----------------------------------------------------------")

	for _, container := range pod.Spec.Containers {
		fmt.Printf("Container logs for: %s\n", container.Name)

		req := CoreV1Client.Pods(pod.Namespace).GetLogs(pod.Name, &corev1.PodLogOptions{
			Container: container.Name,
		})
		logs, err := req.Stream(ctx)
		if err != nil {
			return err
		}
		defer logs.Close()
		if _, err := io.Copy(os.Stdout, logs); err != nil {
			return err
		}
		fmt.Println("-----------------------------------------------------------")
	}
	return nil
}
