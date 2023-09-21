package packages

import (
	"context"
	"testing"

	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	core "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	pkocore "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/internal/testutil"
)

var testScheme = runtime.NewScheme()

func init() {
	if err := pkocore.AddToScheme(testScheme); err != nil {
		panic(err)
	}
	if err := core.AddToScheme(testScheme); err != nil {
		panic(err)
	}
}

// TestGenericPackageController_Bootstrap tests if the controller ignores non package-operator related resources if it is told that
// it runs in bootstrap mode.
func TestGenericPackageController_Bootstrap(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	c := testutil.NewClient()
	log := testr.New(t)
	controller := NewClusterPackageController(c, log, testScheme, nil, nil, nil, true)
	_, err := controller.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: "NOT-package-operator"}})
	require.NoError(t, err)
}

// TestGenericPackageController_NonBootstrap tests if the controller handles a resources if it does not run in bootstrap mode (it should only look at pko resource in bootstrap mode).
func TestGenericPackageController_NonBootstrap(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	c := testutil.NewClient()
	log := testr.New(t)
	controller := NewClusterPackageController(c, log, testScheme, nil, nil, nil, false)
	c.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(k8serrors.NewNotFound(schema.GroupResource{}, ""))
	_, err := controller.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: "NOT-package-operator"}})
	require.NoError(t, err)
}
