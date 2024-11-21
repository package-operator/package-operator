package controllers

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

var testScheme = runtime.NewScheme()

func init() {
	if err := corev1alpha1.AddToScheme(testScheme); err != nil {
		panic(err)
	}
	if err := corev1.AddToScheme(testScheme); err != nil {
		panic(err)
	}
}
