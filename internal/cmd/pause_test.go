package cmd

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"pkg.package-operator.run/cardboard/kubeutils/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/internal/testutil"
)

func TestPackageSetPaused_NotFound(t *testing.T) {
	t.Parallel()

	clientMock := testutil.NewClient()
	c := Client{client: clientMock}
	w := &waiterMock{}

	objectKey := client.ObjectKey{Name: "test-pkg", Namespace: "test-pkg-ns"}
	notFoundErr := apierrors.NewNotFound(schema.GroupResource{
		Group:    "package-operator.run",
		Resource: "Package",
	}, objectKey.Name)

	clientMock.
		On("Get", mock.Anything, objectKey, mock.AnythingOfType("*v1alpha1.Package"), mock.Anything).
		Return(notFoundErr)

	require.ErrorIs(t,
		c.PackageSetPaused(context.Background(), w, objectKey.Name, objectKey.Namespace, true, "banana"),
		notFoundErr,
	)

	clientMock.AssertExpectations(t)
}

var errUpdate = errors.New("test error: oops")

func TestPackageSetPaused_UpdateError(t *testing.T) {
	t.Parallel()

	clientMock := testutil.NewClient()
	c := Client{client: clientMock}
	w := &waiterMock{}

	pkg := newPackage("test-pkg", "test-pkg-ns")

	clientMock.
		On("Get", mock.Anything, client.ObjectKeyFromObject(pkg),
			mock.AnythingOfType("*v1alpha1.Package"), mock.Anything).
		Return(nil)

	clientMock.
		On("Update", mock.Anything, mock.AnythingOfType("*v1alpha1.Package"), mock.Anything).
		Return(errUpdate)

	err := c.PackageSetPaused(context.Background(), w, pkg.Name, pkg.Namespace, true, "banana")
	require.ErrorIs(t, err, errPausingPackage)
	require.ErrorIs(t, err, errUpdate)

	err = c.PackageSetPaused(context.Background(), w, pkg.Name, pkg.Namespace, false, "banana")
	require.ErrorIs(t, err, errUnpausingPackage)
	require.ErrorIs(t, err, errUpdate)

	clientMock.AssertExpectations(t)
}

func TestPackageSetPaused_Success(t *testing.T) {
	t.Parallel()

	clientMock := testutil.NewClient()
	c := Client{client: clientMock}

	pkg := newPackage("test-pkg", "test-pkg-ns")

	clientMock.
		On("Get", mock.Anything, client.ObjectKeyFromObject(pkg),
			mock.AnythingOfType("*v1alpha1.Package"), mock.Anything).
		Return(nil)

	var updatedPkg *corev1alpha1.Package
	clientMock.
		On("Update", mock.Anything, mock.AnythingOfType("*v1alpha1.Package"), mock.Anything).
		Run(func(args mock.Arguments) {
			updatedPkg = args.Get(1).(*corev1alpha1.Package)
		}).
		Return(nil)

	for _, tc := range []struct {
		testName      string
		pause         bool
		conditionType string
	}{
		{
			testName:      "pause",
			pause:         true,
			conditionType: corev1alpha1.PackagePaused,
		},
		{
			testName:      "unpause",
			pause:         false,
			conditionType: corev1alpha1.PackageAvailable,
		},
	} {
		t.Run(tc.testName, func(t *testing.T) {
			t.Parallel()

			w := &waiterMock{}
			w.On(
				"WaitForCondition", mock.Anything, mock.AnythingOfType("*v1alpha1.Package"),
				tc.conditionType, metav1.ConditionTrue, mock.Anything,
			).Return(nil)

			testMsg := "test message"
			require.NoError(t, c.PackageSetPaused(
				context.Background(), w, pkg.Name, pkg.Namespace, tc.pause, testMsg))

			assert.Equal(t, pkg.Name, updatedPkg.Name)
			assert.Equal(t, pkg.Namespace, updatedPkg.Namespace)
			assert.Equal(t, tc.pause, updatedPkg.Spec.Paused)

			if tc.pause {
				require.Contains(t, updatedPkg.Annotations, PauseMessageAnnotation)
				assert.Equal(t, testMsg, updatedPkg.Annotations[PauseMessageAnnotation])
			} else {
				assert.NotContains(t, updatedPkg.Annotations, PauseMessageAnnotation)
			}

			clientMock.AssertExpectations(t)
		})
	}
}

type waiterMock struct {
	mock.Mock
}

func (w *waiterMock) WaitForCondition(
	ctx context.Context,
	object client.Object,
	conditionType string,
	conditionStatus metav1.ConditionStatus,
	opts ...wait.Option,
) error {
	args := w.Called(ctx, object, conditionType, conditionStatus, opts)
	return args.Error(0)
}

func newPackage(name, namespace string) *corev1alpha1.Package {
	return &corev1alpha1.Package{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}
