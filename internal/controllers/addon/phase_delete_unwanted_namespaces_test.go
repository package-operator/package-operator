package addon

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	k8sApiErrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/openshift/addon-operator/internal/controllers"
	"github.com/openshift/addon-operator/internal/testutil"
)

func TestEnsureDeletionOfUnwantedNamespaces_NoNamespacesInSpec_NoNamespacesInCluster(t *testing.T) {
	c := testutil.NewClient()

	c.On("List", testutil.IsContext, testutil.IsCoreV1NamespaceListPtr, mock.Anything).
		Return(nil)

	r := &AddonReconciler{
		Client: c,
		Log:    testutil.NewLogger(t),
		Scheme: testutil.NewTestSchemeWithAddonsv1alpha1(),
	}

	ctx := context.Background()
	err := r.ensureDeletionOfUnwantedNamespaces(ctx, testutil.NewTestAddonWithoutNamespace())
	require.NoError(t, err)
	c.AssertExpectations(t)
}

func TestEnsureDeletionOfUnwantedNamespaces_NoNamespacesInSpec_NamespaceInCluster(t *testing.T) {
	c := testutil.NewClient()

	addon := testutil.NewTestAddonWithoutNamespace()
	existingNamespace := testutil.NewTestNamespace()

	c.On("List", testutil.IsContext, testutil.IsCoreV1NamespaceListPtr, mock.Anything).
		Run(func(args mock.Arguments) {
			arg := args.Get(1).(*corev1.NamespaceList)
			namespaceList := corev1.NamespaceList{
				Items: []corev1.Namespace{
					*existingNamespace,
				},
			}
			namespaceList.DeepCopyInto(arg)
		}).
		Return(nil)
	c.On("Delete", testutil.IsContext, testutil.IsCoreV1NamespacePtr, mock.Anything).
		Return(nil)

	r := &AddonReconciler{
		Client: c,
		Log:    testutil.NewLogger(t),
		Scheme: testutil.NewTestSchemeWithAddonsv1alpha1(),
	}

	ctx := context.Background()
	err := r.ensureDeletionOfUnwantedNamespaces(ctx, addon)
	require.NoError(t, err)
	c.AssertExpectations(t)
	c.AssertCalled(t, "List", testutil.IsContext,
		testutil.IsCoreV1NamespaceListPtr,
		// verify that the list call did use the correct labelSelector
		mock.MatchedBy(func(listOptions []client.ListOption) bool {
			testListOptions := &client.ListOptions{}
			listOptions[0].ApplyToList(testListOptions)
			testLabelSelectorString := testListOptions.LabelSelector.String()
			return len(testLabelSelectorString) > 0 &&
				testLabelSelectorString == controllers.CommonLabelsAsLabelSelector(addon).String()
		}))
	c.AssertCalled(t, "Delete", testutil.IsContext,
		mock.MatchedBy(func(val *corev1.Namespace) bool {
			return val.Name == existingNamespace.Name
		}),
		mock.Anything)
}

func TestEnsureDeletionOfUnwantedNamespaces_NamespacesInSpec_matching_NamespacesInCluster(t *testing.T) {
	c := testutil.NewClient()

	addon := testutil.NewTestAddonWithSingleNamespace()
	existingNamespace := testutil.NewTestNamespace()

	c.On("List", testutil.IsContext, testutil.IsCoreV1NamespaceListPtr, mock.Anything).
		Run(func(args mock.Arguments) {
			arg := args.Get(1).(*corev1.NamespaceList)
			namespaceList := corev1.NamespaceList{
				Items: []corev1.Namespace{
					*existingNamespace,
				},
			}
			namespaceList.DeepCopyInto(arg)
		}).
		Return(nil)

	r := &AddonReconciler{
		Client: c,
		Log:    testutil.NewLogger(t),
		Scheme: testutil.NewTestSchemeWithAddonsv1alpha1(),
	}

	ctx := context.Background()
	err := r.ensureDeletionOfUnwantedNamespaces(ctx, addon)
	require.NoError(t, err)
	c.AssertExpectations(t)
	c.AssertCalled(t, "List", testutil.IsContext,
		testutil.IsCoreV1NamespaceListPtr,
		// verify that the list call did use the correct labelSelector
		mock.MatchedBy(func(listOptions []client.ListOption) bool {
			testListOptions := &client.ListOptions{}
			listOptions[0].ApplyToList(testListOptions)
			testLabelSelectorString := testListOptions.LabelSelector.String()
			return len(testLabelSelectorString) > 0 &&
				testLabelSelectorString == controllers.CommonLabelsAsLabelSelector(addon).String()
		}))
}

func TestEnsureDeletionOfUnwantedNamespaces_NoNamespacesInSpec_WithClientError(t *testing.T) {
	timeoutErr := k8sApiErrors.NewTimeoutError("for testing", 1)

	c := testutil.NewClient()
	c.On("List", testutil.IsContext, testutil.IsCoreV1NamespaceListPtr, mock.Anything).
		Return(timeoutErr)

	r := &AddonReconciler{
		Client: c,
		Log:    testutil.NewLogger(t),
		Scheme: testutil.NewTestSchemeWithAddonsv1alpha1(),
	}

	ctx := context.Background()
	err := r.ensureDeletionOfUnwantedNamespaces(ctx, testutil.NewTestAddonWithoutNamespace())
	require.EqualError(t, errors.Unwrap(err), timeoutErr.Error())
	c.AssertExpectations(t)
}
