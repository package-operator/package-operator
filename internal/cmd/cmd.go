package cmd

import (
	"errors"

	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"

	pkoapis "package-operator.run/apis"
	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
)

var ErrInvalidArgs = errors.New("arguments invalid")

func NewScheme() (*runtime.Scheme, error) {
	scheme := runtime.NewScheme()

	if err := pkoapis.AddToScheme(scheme); err != nil {
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
