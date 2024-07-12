package controllers

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/internal/testutil"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func Test_impersonationConfigForObject_namespaced(t *testing.T) {
	t.Parallel()
	pkg := &corev1alpha1.Package{
		TypeMeta: metav1.TypeMeta{
			Kind: "Package",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "banana",
			Namespace: "fruits",
		},
	}
	odepl := &corev1alpha1.ObjectDeployment{
		TypeMeta: metav1.TypeMeta{
			Kind: "ObjectDeployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "banana",
			Namespace: "fruits",
		},
	}
	require.NoError(t,
		controllerutil.SetControllerReference(pkg, odepl, testScheme))

	oset := &corev1alpha1.ObjectSet{
		TypeMeta: metav1.TypeMeta{
			Kind: "ObjectSet",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "banana-xxx",
			Namespace: "fruits",
		},
	}
	require.NoError(t,
		controllerutil.SetControllerReference(odepl, oset, testScheme))

	c := testutil.NewClient()
	c.On(
		"Get", mock.Anything, mock.Anything,
		mock.AnythingOfType("*v1alpha1.ObjectDeployment"),
		mock.Anything,
	).Run(func(args mock.Arguments) {
		o := args.Get(2).(*corev1alpha1.ObjectDeployment)
		*o = *odepl
	}).Return(nil)
	c.On(
		"Get", mock.Anything, mock.Anything,
		mock.AnythingOfType("*v1alpha1.Package"),
		mock.Anything,
	).Run(func(args mock.Arguments) {
		o := args.Get(2).(*corev1alpha1.Package)
		*o = *pkg
	}).Return(nil)

	ctx := context.Background()
	w := &AutoImpersonatingWriterWrapper{
		scheme: testScheme,
		reader: c,
	}
	i, err := w.impersonationConfigForObject(ctx, oset)
	require.NoError(t, err)
	assert.Equal(t, rest.ImpersonationConfig{
		UserName: "pko:package:fruits:banana",
		Groups: []string{
			"pko:packages:fruits",
			"pko:packages",
		},
	}, i)
}

func Test_impersonationConfigForObject_cluster(t *testing.T) {
	t.Parallel()
	pkg := &corev1alpha1.ClusterPackage{
		TypeMeta: metav1.TypeMeta{
			Kind: "ClusterPackage",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "banana",
		},
	}
	odepl := &corev1alpha1.ClusterObjectDeployment{
		TypeMeta: metav1.TypeMeta{
			Kind: "ClusterObjectDeployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "banana",
		},
	}
	require.NoError(t,
		controllerutil.SetControllerReference(pkg, odepl, testScheme))

	oset := &corev1alpha1.ClusterObjectSet{
		TypeMeta: metav1.TypeMeta{
			Kind: "ClusterObjectSet",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "banana-xxx",
		},
	}
	require.NoError(t,
		controllerutil.SetControllerReference(odepl, oset, testScheme))

	c := testutil.NewClient()
	c.On(
		"Get", mock.Anything, mock.Anything,
		mock.AnythingOfType("*v1alpha1.ClusterObjectDeployment"),
		mock.Anything,
	).Run(func(args mock.Arguments) {
		o := args.Get(2).(*corev1alpha1.ClusterObjectDeployment)
		*o = *odepl
	}).Return(nil)
	c.On(
		"Get", mock.Anything, mock.Anything,
		mock.AnythingOfType("*v1alpha1.ClusterPackage"),
		mock.Anything,
	).Run(func(args mock.Arguments) {
		o := args.Get(2).(*corev1alpha1.ClusterPackage)
		*o = *pkg
	}).Return(nil)

	ctx := context.Background()
	w := &AutoImpersonatingWriterWrapper{
		scheme: testScheme,
		reader: c,
	}
	i, err := w.impersonationConfigForObject(ctx, oset)
	require.NoError(t, err)
	assert.Equal(t, rest.ImpersonationConfig{
		UserName: "pko:clusterpackage:banana",
		Groups: []string{
			"pko:clusterpackages",
		},
	}, i)
}
