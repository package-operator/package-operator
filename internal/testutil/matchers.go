package testutil

import (
	"context"

	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"github.com/stretchr/testify/mock"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	addonsv1alpha1 "github.com/openshift/addon-operator/apis/addons/v1alpha1"
)

// custom testify/mock matchers
var (
	IsAddonsv1alpha1AddonPtr            = mock.IsType(&addonsv1alpha1.Addon{})
	IsContext                           = mock.IsType(context.TODO())
	IsCoreV1NamespacePtr                = mock.IsType(&corev1.Namespace{})
	IsCoreV1NamespaceListPtr            = mock.IsType(&corev1.NamespaceList{})
	IsObjectKey                         = mock.IsType(client.ObjectKey{})
	IsOperatorsV1Alpha1CatalogSourcePtr = mock.IsType(&operatorsv1alpha1.CatalogSource{})
)
