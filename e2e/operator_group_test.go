package e2e_test

import (
	"context"
	"testing"
	"time"

	operatorsv1 "github.com/operator-framework/api/pkg/operators/v1"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	addonsv1alpha1 "github.com/openshift/addon-operator/apis/addons/v1alpha1"
	"github.com/openshift/addon-operator/e2e"
)

func TestAddon_OperatorGroup_OwnNamespace(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	addon := &addonsv1alpha1.Addon{
		ObjectMeta: metav1.ObjectMeta{
			Name: "addon-fuccniy3l4",
		},
		Spec: addonsv1alpha1.AddonSpec{
			Install: addonsv1alpha1.AddonInstallSpec{
				Type: addonsv1alpha1.OwnNamespaces,
				OwnNamespace: &addonsv1alpha1.AddonInstallSpecOwnNamespace{
					Namespace: "default",
				},
			},
		},
	}

	err := e2e.Client.Create(ctx, addon)
	require.NoError(t, err)
	t.Cleanup(func() {
		err := e2e.Client.Delete(ctx, addon)
		if client.IgnoreNotFound(err) != nil {
			t.Logf("could not clean up Addon %s: %v", addon.Name, err)
		}
	})

	err = e2e.WaitForObject(
		t, 1*time.Minute, addon, "to be Available",
		func(obj client.Object) (done bool, err error) {
			a := obj.(*addonsv1alpha1.Addon)
			return meta.IsStatusConditionTrue(
				a.Status.Conditions, addonsv1alpha1.Available), nil
		})
	require.NoError(t, err)

	operatorGroup := &operatorsv1.OperatorGroup{}
	require.NoError(t, e2e.Client.Get(ctx, client.ObjectKey{
		Name:      addon.Name,
		Namespace: addon.Spec.Install.OwnNamespace.Namespace,
	}, operatorGroup))
}
