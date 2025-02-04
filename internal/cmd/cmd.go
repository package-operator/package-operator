package cmd

import (
	"errors"
	"time"

	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"pkg.package-operator.run/cardboard/kubeutils/wait"

	"package-operator.run/apis"
	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
)

var ErrInvalidArgs = errors.New("arguments invalid")

func NewScheme() (*runtime.Scheme, error) {
	scheme := runtime.NewScheme()

	if err := apis.AddToScheme(scheme); err != nil {
		return nil, err
	}
	if err := manifestsv1alpha1.AddToScheme(scheme); err != nil {
		return nil, err
	}
	if err := apiextensionsv1.AddToScheme(scheme); err != nil {
		return nil, err
	}
	if err := apiextensions.AddToScheme(scheme); err != nil {
		return nil, err
	}

	return scheme, nil
}

type ClientFactory interface {
	Client() (*Client, error)
}

func NewDefaultClientFactory(kcliFactory KubeClientFactory) *DefaultClientFactory {
	return &DefaultClientFactory{
		kcliFactory: kcliFactory,
	}
}

type DefaultClientFactory struct {
	kcliFactory KubeClientFactory
}

func (f *DefaultClientFactory) Client() (*Client, error) {
	cli, err := f.kcliFactory.GetKubeClient()
	if err != nil {
		return nil, err
	}

	return NewClient(cli), nil
}

type KubeClientFactory interface {
	GetKubeClient() (client.Client, error)
}

func NewDefaultKubeClientFactory(scheme *runtime.Scheme, cfgFactory RestConfigFactory) *DefaultKubeClientFactory {
	return &DefaultKubeClientFactory{
		cfgFactory: cfgFactory,
		scheme:     scheme,
	}
}

type DefaultKubeClientFactory struct {
	cfgFactory RestConfigFactory
	scheme     *runtime.Scheme
}

func (f *DefaultKubeClientFactory) GetKubeClient() (client.Client, error) {
	cfg, err := f.cfgFactory.GetConfig()
	if err != nil {
		return nil, err
	}

	return client.New(cfg, client.Options{
		Scheme: f.scheme,
	})
}

type RestConfigFactory interface {
	GetConfig() (*rest.Config, error)
}

func NewDefaultRestConfigFactory() *DefaultRestConfigFactory {
	return &DefaultRestConfigFactory{}
}

type DefaultRestConfigFactory struct{}

func (f *DefaultRestConfigFactory) GetConfig() (*rest.Config, error) {
	return ctrl.GetConfig()
}

func NewWaiter(client *Client, scheme *runtime.Scheme) *wait.Waiter {
	return wait.NewWaiter(
		client.client, scheme,
		wait.WithTimeout(20*time.Second),
		wait.WithInterval(time.Second),
	)
}
