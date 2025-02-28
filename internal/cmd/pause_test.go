package cmd

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"pkg.package-operator.run/cardboard/kubeutils/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/internal/testutil"
)

var testScheme = runtime.NewScheme()

func init() {
	if err := corev1alpha1.AddToScheme(testScheme); err != nil {
		panic(err)
	}
}

func TestPackageSetPaused_NotFound(t *testing.T) {
	t.Parallel()

	for _, tc := range getScopedTestCases() {
		t.Run(tc.scope, func(t *testing.T) {
			t.Parallel()

			clientMock := testutil.NewClient()
			c := Client{client: clientMock}
			w := &waiterMock{}

			objectKey := client.ObjectKey{Name: "test-pkg", Namespace: tc.namespace}
			notFoundErr := apierrors.NewNotFound(schema.GroupResource{
				Group:    "package-operator.run",
				Resource: tc.resource,
			}, objectKey.Name)

			clientMock.On("Scheme").Return(testScheme)
			clientMock.
				On("Get", mock.Anything, objectKey, mock.AnythingOfType("*v1alpha1."+tc.resource), mock.Anything).
				Return(notFoundErr)

			require.ErrorIs(t,
				c.PackageSetPaused(
					context.Background(), w, strings.ToLower(tc.resource),
					objectKey.Name, objectKey.Namespace, true, "banana",
				),
				notFoundErr,
			)

			clientMock.AssertExpectations(t)
		})
	}
}

var errUpdate = errors.New("test error: oops")

func TestPackageSetPaused_UpdateError(t *testing.T) {
	t.Parallel()

	for _, tc := range getScopedTestCases() {
		t.Run(tc.scope, func(t *testing.T) {
			t.Parallel()

			clientMock := testutil.NewClient()
			c := Client{client: clientMock}
			w := &waiterMock{}

			pkg := newPackage("test-pkg", tc.namespace)

			clientMock.On("Scheme").Return(testScheme)
			clientMock.
				On("Get", mock.Anything, client.ObjectKeyFromObject(pkg),
					mock.AnythingOfType("*v1alpha1."+tc.resource), mock.Anything).
				Return(nil)

			clientMock.
				On("Update", mock.Anything, mock.AnythingOfType("*v1alpha1."+tc.resource), mock.Anything).
				Return(errUpdate)

			err := c.PackageSetPaused(
				context.Background(), w, strings.ToLower(tc.resource),
				pkg.GetName(), pkg.GetNamespace(), true, "banana",
			)
			require.ErrorIs(t, err, errPausingPackage)
			require.ErrorIs(t, err, errUpdate)

			err = c.PackageSetPaused(
				context.Background(), w, strings.ToLower(tc.resource),
				pkg.GetName(), pkg.GetNamespace(), false, "banana",
			)
			require.ErrorIs(t, err, errUnpausingPackage)
			require.ErrorIs(t, err, errUpdate)

			clientMock.AssertExpectations(t)
		})
	}
}

func TestPackageSetPaused_Success_Namespaced(t *testing.T) {
	t.Parallel()

	clientMock := testutil.NewClient()
	c := Client{client: clientMock}

	pkg := newPackage("test-pkg", "test-pkg-ns")

	clientMock.On("Scheme").Return(testScheme)
	clientMock.
		On("Get", mock.Anything, client.ObjectKeyFromObject(pkg),
			mock.AnythingOfType("*v1alpha1.Package"), mock.Anything).
		Run(func(args mock.Arguments) {
			pkgObj := args.Get(2).(*corev1alpha1.Package)
			*pkgObj = *pkg.(*corev1alpha1.Package)
		}).
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
				context.Background(), w, "package", pkg.GetName(), pkg.GetNamespace(), tc.pause, testMsg))

			assert.Equal(t, pkg.GetName(), updatedPkg.Name)
			assert.Equal(t, pkg.GetNamespace(), updatedPkg.Namespace)
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

func TestPackageSetPaused_Success_Cluster(t *testing.T) {
	t.Parallel()

	clientMock := testutil.NewClient()
	c := Client{client: clientMock}

	pkg := newPackage("test-pkg", "")

	clientMock.On("Scheme").Return(testScheme)
	clientMock.
		On("Get", mock.Anything, client.ObjectKeyFromObject(pkg),
			mock.AnythingOfType("*v1alpha1.ClusterPackage"), mock.Anything).
		Run(func(args mock.Arguments) {
			pkgObj := args.Get(2).(*corev1alpha1.ClusterPackage)
			*pkgObj = *pkg.(*corev1alpha1.ClusterPackage)
		}).
		Return(nil)

	var updatedPkg *corev1alpha1.ClusterPackage
	clientMock.
		On("Update", mock.Anything, mock.AnythingOfType("*v1alpha1.ClusterPackage"), mock.Anything).
		Run(func(args mock.Arguments) {
			updatedPkg = args.Get(1).(*corev1alpha1.ClusterPackage)
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
				"WaitForCondition", mock.Anything, mock.AnythingOfType("*v1alpha1.ClusterPackage"),
				tc.conditionType, metav1.ConditionTrue, mock.Anything,
			).Return(nil)

			testMsg := "test message"
			require.NoError(t, c.PackageSetPaused(
				context.Background(), w, "clusterpackage", pkg.GetName(), pkg.GetNamespace(), tc.pause, testMsg))

			assert.Equal(t, pkg.GetName(), updatedPkg.Name)
			assert.Equal(t, pkg.GetNamespace(), updatedPkg.Namespace)
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

func newPackage(name, namespace string) client.Object {
	m := metav1.ObjectMeta{
		Name:      name,
		Namespace: namespace,
	}

	if namespace == "" {
		return &corev1alpha1.ClusterPackage{
			ObjectMeta: m,
		}
	}
	return &corev1alpha1.Package{
		ObjectMeta: m,
	}
}

type testCase struct {
	scope     string
	resource  string
	namespace string
}

func getScopedTestCases() []testCase {
	return []testCase{
		{
			scope:     "namespaced",
			resource:  "Package",
			namespace: "test-pkg-ns",
		},
		{
			scope:    "cluster",
			resource: "ClusterPackage",
		},
	}
}
