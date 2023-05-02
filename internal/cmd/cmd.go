package cmd

import (
	"errors"
	"fmt"

	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/funcr"
	"github.com/spf13/cobra"

	pkoapis "package-operator.run/apis"
	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
)

var ErrInvalidArgs = errors.New("arguments invalid")

func LogFromCmd(cmd *cobra.Command) logr.Logger {
	return funcr.New(func(p, a string) {
		fmt.Fprintln(cmd.ErrOrStderr(), p, a)
	}, funcr.Options{})
}

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
