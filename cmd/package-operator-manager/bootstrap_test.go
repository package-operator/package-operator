package main

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/package-operator/internal/controllers"
	"package-operator.run/package-operator/internal/testutil"
)

func TestBootstrapper_createPKOPackage(t *testing.T) {
	c := testutil.NewClient()
	ctx := logr.NewContext(context.Background(), testr.New(t))

	b := &bootstrapper{client: c}

	c.
		On("Create", mock.Anything,
			mock.AnythingOfType("*v1alpha1.ClusterPackage"), mock.Anything).
		Return(nil)

	pkg, err := b.createPKOPackage(ctx)
	require.NoError(t, err)
	assert.NotNil(t, pkg)
	c.AssertExpectations(t)
}

func TestBootstrapper_ensureCRDs(t *testing.T) {
	c := testutil.NewClient()
	ctx := logr.NewContext(context.Background(), testr.New(t))

	b := &bootstrapper{client: c}

	crd := unstructured.Unstructured{}
	crd.SetGroupVersionKind(crdGK.WithVersion("v1"))

	c.
		On("Create", mock.Anything, mock.Anything, mock.Anything).
		Once().
		Return(errors.NewAlreadyExists(schema.GroupResource{}, ""))
	c.
		On("Create", mock.Anything, mock.Anything, mock.Anything).
		Return(nil)

	crds := []unstructured.Unstructured{crd, crd}
	err := b.ensureCRDs(ctx, crds)
	require.NoError(t, err)

	for _, crd := range crds {
		assert.Equal(t, map[string]string{
			controllers.DynamicCacheLabel: "True",
		}, crd.GetLabels())
	}
	c.AssertExpectations(t)
}

func Test_crdsFromTemplateSpec(t *testing.T) {
	crd := unstructured.Unstructured{}
	crd.SetGroupVersionKind(crdGK.WithVersion("v1"))

	ts := corev1alpha1.ObjectSetTemplateSpec{
		Phases: []corev1alpha1.ObjectSetTemplatePhase{
			{
				Objects: []corev1alpha1.ObjectSetObject{
					{
						Object: unstructured.Unstructured{},
					},
				},
			},
			{
				Objects: []corev1alpha1.ObjectSetObject{
					{
						Object: crd,
					},
				},
			},
		},
	}
	crds := crdsFromTemplateSpec(ts)
	assert.Len(t, crds, 1)
}
