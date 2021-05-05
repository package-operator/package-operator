package testutil

import (
	"context"

	"github.com/stretchr/testify/mock"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	addonsv1alpha1 "github.com/openshift/addon-operator/apis/addons/v1alpha1"
)

// custom testify/mock matchers
var (
	IsContext                = mock.IsType(context.TODO())
	IsObjectKey              = mock.IsType(client.ObjectKey{})
	IsCoreV1NamespacePtr     = mock.IsType(&corev1.Namespace{})
	IsCoreV1NamespaceListPtr = mock.IsType(&corev1.NamespaceList{})
	IsAddonsv1alpha1AddonPtr = mock.IsType(&addonsv1alpha1.Addon{})
)
