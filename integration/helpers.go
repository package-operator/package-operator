// Package integration contains the Addon Operator integration tests.
package integration

import (
	"bytes"
	"context"
	goerrors "errors"
	"fmt"
	"io"
	"io/fs"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"testing"
	"time"

	configv1 "github.com/openshift/api/config/v1"
	operatorsv1 "github.com/operator-framework/api/pkg/operators/v1"
	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/kubectl/pkg/proxy"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/yaml"

	aoapis "github.com/openshift/addon-operator/apis"
	"github.com/openshift/addon-operator/internal/ocm"
	"github.com/openshift/addon-operator/internal/testutil"
)

const (
	relativeConfigDeployPath           = "../config/deploy"
	relativeWebhookConfigDeployPath    = "../config/deploy/webhook"
	relativeOCMAPIMockConfigDeployPath = "../config/deploy/api-mock"
	OCMAPIEndpoint                     = "http://api-mock.api-mock.svc.cluster.local"
)

type fileInfosByName []fs.FileInfo

type fileInfoMap struct {
	absPath  string
	fileInfo []os.FileInfo
}

func (x fileInfosByName) Len() int { return len(x) }

func (x fileInfosByName) Less(i, j int) bool {
	iName := path.Base(x[i].Name())
	jName := path.Base(x[j].Name())
	return iName < jName
}
func (x fileInfosByName) Swap(i, j int) { x[i], x[j] = x[j], x[i] }

var (
	// Client pointing to the e2e test cluster.
	Client client.Client
	Config *rest.Config
	Scheme = runtime.NewScheme()

	OCMClient *ocm.Client

	// Typed K8s Clients
	CoreV1Client corev1client.CoreV1Interface

	// Path to the deployment configuration directory.
	PathConfigDeploy string

	// Path to the webhook deployment configuration directory.
	PathWebhookConfigDeploy string

	// Path to the OCM API
	PathOCMAPIMockDeploy string
)

func init() {
	// Client/Scheme setup.
	AddToSchemes := runtime.SchemeBuilder{
		clientgoscheme.AddToScheme,
		aoapis.AddToScheme,
		apiextensionsv1.AddToScheme,
		operatorsv1.AddToScheme,
		operatorsv1alpha1.AddToScheme,
		configv1.AddToScheme,
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

	// OCM Client
	cv := &configv1.ClusterVersion{}
	if err := Client.Get(context.Background(), client.ObjectKey{Name: "version"}, cv); err != nil {
		panic(fmt.Errorf("getting clusterversion: %w", err))
	}
	OCMClient = ocm.NewClient(
		ocm.WithEndpoint("http://127.0.0.1:8001/api/v1/namespaces/api-mock/services/api-mock:80/proxy"),
		ocm.WithAccessToken("accessToken"), //TODO: Needs to be supplied from the outside, does not matter for mock.
		ocm.WithClusterID(string(cv.Spec.ClusterID)),
	)

	// Paths
	PathConfigDeploy, err = filepath.Abs(relativeConfigDeployPath)
	if err != nil {
		panic(err)
	}

	PathWebhookConfigDeploy, err = filepath.Abs(relativeWebhookConfigDeployPath)
	if err != nil {
		panic(err)
	}

	PathOCMAPIMockDeploy, err = filepath.Abs(relativeOCMAPIMockConfigDeployPath)
	if err != nil {
		panic(err)
	}
}

func getFileInfoFromPath(paths []string) ([]fileInfoMap, error) {
	fileInfo := []fileInfoMap{}

	for _, path := range paths {
		config, err := os.Open(path)
		if err != nil {
			return fileInfo, err
		}

		files, err := config.Readdir(-1)
		if err != nil {
			return fileInfo, err
		}

		sort.Sort(fileInfosByName(files))

		fileInfo = append(fileInfo, fileInfoMap{
			absPath:  path,
			fileInfo: files,
		})
	}

	return fileInfo, nil
}

// Load all k8s objects from .yaml files in config/deploy.
// File/Object order is preserved.
func LoadObjectsFromDeploymentFiles(t *testing.T) []unstructured.Unstructured {
	paths := []string{PathConfigDeploy, PathOCMAPIMockDeploy}
	if testutil.IsWebhookServerEnabled() {
		paths = append(paths, PathWebhookConfigDeploy)
	}
	fileInfoMap, err := getFileInfoFromPath(paths)
	require.NoError(t, err)

	var objects []unstructured.Unstructured

	for _, m := range fileInfoMap {
		for _, f := range m.fileInfo {
			if f.IsDir() {
				continue
			}
			if path.Ext(f.Name()) != ".yaml" {
				continue
			}

			fileYaml, err := ioutil.ReadFile(path.Join(
				m.absPath, f.Name()))
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
		p := pod
		if err := reportPodStatus(ctx, &p); err != nil {
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

// Default Interval in which to recheck wait conditions.
const defaultWaitPollInterval = time.Second

// WaitToBeGone blocks until the given object is gone from the kubernetes API server.
func WaitToBeGone(t *testing.T, timeout time.Duration, object client.Object) error {
	gvk, err := apiutil.GVKForObject(object, Scheme)
	if err != nil {
		return err
	}

	key := client.ObjectKeyFromObject(object)
	t.Logf("waiting %s for %s %s to be gone...",
		timeout, gvk, key)

	ctx := context.Background()
	return wait.PollImmediate(defaultWaitPollInterval, timeout, func() (done bool, err error) {
		err = Client.Get(ctx, key, object)

		if errors.IsNotFound(err) {
			return true, nil
		}

		if err != nil {
			t.Logf("error waiting for %s %s to be gone: %v",
				object.GetObjectKind().GroupVersionKind().Kind, key, err)
		}
		return false, nil
	})
}

// Wait that something happens with an object.
func WaitForObject(
	t *testing.T, timeout time.Duration,
	object client.Object, reason string,
	checkFn func(obj client.Object) (done bool, err error),
) error {
	gvk, err := apiutil.GVKForObject(object, Scheme)
	if err != nil {
		return err
	}

	key := client.ObjectKeyFromObject(object)
	t.Logf("waiting %s on %s %s %s...",
		timeout, gvk, key, reason)

	ctx := context.Background()
	return wait.PollImmediate(time.Second, timeout, func() (done bool, err error) {
		err = Client.Get(ctx, client.ObjectKeyFromObject(object), object)
		if err != nil {
			//nolint:nilerr // retry on transient errors
			return false, nil
		}

		return checkFn(object)
	})
}

const (
	defaultPort      = 8001
	defaultAPIPrefix = "/"
	defaultAddress   = "127.0.0.1"
)

// Runs a local apiserver proxy on 127.0.0.1:8001 similar to `kubectl proxy`.
func RunAPIServerProxy(closeCh <-chan struct{}) error {
	mux := http.NewServeMux()

	proxyHandler, err := proxy.NewProxyHandler(defaultAPIPrefix, nil, Config, 0)
	if err != nil {
		return fmt.Errorf("creating proxy server: %w", err)
	}
	mux.Handle(defaultAPIPrefix, proxyHandler)

	// Already start a listener, so callers can already connect to the server,
	// even if the server is not up yet.
	l, err := net.Listen("tcp", fmt.Sprintf("%s:%d", defaultAddress, defaultPort))
	if err != nil {
		return fmt.Errorf("listen on %s:%d: %w", defaultAddress, defaultPort, err)
	}

	server := http.Server{
		Handler: mux,
	}

	go func() {
		if err := server.Serve(l); err != nil &&
			!goerrors.Is(err, http.ErrServerClosed) {
			panic(err)
		}
	}()

	go func() {
		<-closeCh
		if err := server.Close(); err != nil {
			panic(err)
		}
	}()
	return nil
}
