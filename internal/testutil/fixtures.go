package testutil

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
)

func NewTestSchemeWithCoreV1() *runtime.Scheme {
	testScheme := runtime.NewScheme()
	_ = corev1.AddToScheme(testScheme)
	return testScheme
}

func NewTestSchemeWithCoreV1AppsV1() *runtime.Scheme {
	testScheme := runtime.NewScheme()
	_ = corev1.AddToScheme(testScheme)
	_ = appsv1.AddToScheme(testScheme)
	return testScheme
}

func NewConfigMap() *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cm",
			Namespace: "cmtestns",
			UID:       types.UID("asdfjkl"),
		},
	}
}

func NewSecret() *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "secret1",
			Namespace: "testns",
			UID:       types.UID("qweruiop"),
		},
	}
}
