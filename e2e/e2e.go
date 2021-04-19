// Package e2e contains the Addon Operator E2E tests.
package e2e

import (
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	aoapis "github.com/openshift/addon-operator/apis"
)

var (
	// Client pointing to the e2e test cluster.
	c      client.Client
	scheme = runtime.NewScheme()
)

func init() {
	err := clientgoscheme.AddToScheme(scheme)
	if err != nil {
		panic(err)
	}

	err = aoapis.AddToScheme(scheme)
	if err != nil {
		panic(err)
	}

	err = apiextensionsv1.AddToScheme(scheme)
	if err != nil {
		panic(err)
	}

	c, err = client.New(ctrl.GetConfigOrDie(), client.Options{
		Scheme: scheme,
	})
	if err != nil {
		panic(err)
	}
}
