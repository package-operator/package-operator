package packages

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/package-operator/internal/testutil"
)

func init() {
	if err := corev1alpha1.AddToScheme(testScheme); err != nil {
		panic(err)
	}
}

func Test_objectDeploymentStatusReconciler(t *testing.T) {
	pkgName, pkgNamespace := "foo", "foo-ns"
	objDepName, objDepNamespace := pkgName, pkgNamespace

	t.Run("ignore non-existing objectDeployment", func(t *testing.T) {
		testClient := testutil.NewClient()
		r := &objectDeploymentStatusReconciler{
			scheme:              testScheme,
			client:              testClient,
			newObjectDeployment: newGenericObjectDeployment,
		}
		testClient.
			On("Get", mock.Anything, client.ObjectKey{Name: objDepName, Namespace: objDepNamespace}, mock.Anything, mock.Anything).
			Return(errWithStatusError{errStatusReason: metav1.StatusReasonNotFound})

		pkg := &GenericPackage{
			corev1alpha1.Package{
				ObjectMeta: metav1.ObjectMeta{
					Name:      pkgName,
					Namespace: pkgNamespace,
				},
			},
		}

		ctx := context.Background()
		res, err := r.Reconcile(ctx, pkg)
		require.NoError(t, err)
		require.Equal(t, res, ctrl.Result{})

		testClient.AssertCalled(t, "Get", mock.Anything, client.ObjectKey{Name: objDepName, Namespace: objDepNamespace}, mock.Anything, mock.Anything)
	})

	testCases := []struct {
		name                     string
		conditionType            string
		objDepGeneration         int64
		objDepObservedGeneration int64
	}{
		{
			name:                     "objectDeployment with available condition pointing to current generation",
			conditionType:            corev1alpha1.ObjectDeploymentAvailable,
			objDepGeneration:         2,
			objDepObservedGeneration: 2,
		},
		{
			name:                     "objectDeployment with available condition pointing to old generation",
			conditionType:            corev1alpha1.ObjectDeploymentAvailable,
			objDepGeneration:         2,
			objDepObservedGeneration: 1,
		},
		{
			name:                     "objectDeployment with progressing condition pointing to current generation",
			conditionType:            corev1alpha1.ObjectDeploymentProgressing,
			objDepGeneration:         2,
			objDepObservedGeneration: 2,
		},
		{
			name:                     "objectDeployment with progressing condition pointing to old generation",
			conditionType:            corev1alpha1.ObjectDeploymentProgressing,
			objDepGeneration:         2,
			objDepObservedGeneration: 1,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			testClient := testutil.NewClient()
			r := &objectDeploymentStatusReconciler{
				scheme:              testScheme,
				client:              testClient,
				newObjectDeployment: newGenericObjectDeployment,
			}
			pkg := &GenericPackage{
				corev1alpha1.Package{
					ObjectMeta: metav1.ObjectMeta{
						Name:      pkgName,
						Namespace: pkgNamespace,
					},
				},
			}
			testClient.
				On("Get", mock.Anything, client.ObjectKey{Name: objDepName, Namespace: objDepNamespace}, mock.Anything, mock.Anything).
				Run(func(args mock.Arguments) {
					objDep := corev1alpha1.ObjectDeployment{
						ObjectMeta: metav1.ObjectMeta{
							Name:       objDepName,
							Namespace:  objDepNamespace,
							Generation: testCase.objDepGeneration,
						},
					}
					meta.SetStatusCondition(&objDep.Status.Conditions, metav1.Condition{
						Type:               testCase.conditionType,
						Status:             metav1.ConditionTrue,
						ObservedGeneration: testCase.objDepObservedGeneration,
					})
					destination := args.Get(2).(*corev1alpha1.ObjectDeployment)
					objDep.DeepCopyInto(destination)
				}).
				Return(nil)

			ctx := context.Background()
			res, err := r.Reconcile(ctx, pkg)
			require.NoError(t, err)
			require.Equal(t, res, ctrl.Result{})

			testClient.AssertCalled(t, "Get", mock.Anything, client.ObjectKey{Name: objDepName, Namespace: objDepNamespace}, mock.Anything, mock.Anything)
			condFound := meta.FindStatusCondition(*pkg.GetConditions(), testCase.conditionType)
			if testCase.objDepGeneration != testCase.objDepObservedGeneration {
				require.Nil(t, condFound)
				return
			}
			require.NotNil(t, condFound)

			expectedCondition := metav1.Condition{
				Type:               testCase.conditionType,
				Status:             metav1.ConditionTrue,
				ObservedGeneration: pkg.ClientObject().GetGeneration(),
			}
			foundCondition := metav1.Condition{
				Type:               condFound.Type,
				Status:             condFound.Status,
				ObservedGeneration: condFound.ObservedGeneration,
			}
			require.Equal(t, expectedCondition, foundCondition)
		})
	}
}
