// Package e2e contains the Addon Operator E2E tests.
package e2e

import (
	"bytes"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"testing"

	aoapis "github.com/openshift/addon-operator/apis"
	"github.com/stretchr/testify/require"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

const (
	relativeConfigDeployPath = "../../config/deploy"
)

var (
	// Client pointing to the e2e test cluster.
	Client client.Client
	Scheme = runtime.NewScheme()

	// Path to the deployment configuration directory.
	PathConfigDeploy string
)

func init() {
	// Client/Scheme setup.
	err := clientgoscheme.AddToScheme(Scheme)
	if err != nil {
		panic(err)
	}

	err = aoapis.AddToScheme(Scheme)
	if err != nil {
		panic(err)
	}

	err = apiextensionsv1.AddToScheme(Scheme)
	if err != nil {
		panic(err)
	}

	Client, err = client.New(ctrl.GetConfigOrDie(), client.Options{
		Scheme: Scheme,
	})
	if err != nil {
		panic(err)
	}

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
