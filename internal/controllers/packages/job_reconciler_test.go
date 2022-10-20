package packages

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/package-operator/internal/testutil"
)

var (
	testScheme = runtime.NewScheme()
)

func init() {
	if err := corev1alpha1.AddToScheme(testScheme); err != nil {
		panic(err)
	}
}

func Test_jobReconciler(t *testing.T) {
	t.Run("create desiredJob with ownerReference when no job found", func(t *testing.T) {
		testClient := testutil.NewClient()
		pkgName, pkgNamespace := "foo", "foo-ns"
		jobName, jobNamespace := fmt.Sprintf("job-%s", pkgName), "package-operator-system"

		pkg := &GenericPackage{
			corev1alpha1.Package{
				ObjectMeta: metav1.ObjectMeta{
					Name:      pkgName,
					Namespace: pkgNamespace,
				},
			},
		}

		testClient.
			On("Get", mock.Anything, client.ObjectKey{Name: jobName, Namespace: jobNamespace}, mock.Anything, mock.Anything).
			Return(errWithStatusError{errStatusReason: metav1.StatusReasonNotFound})

		var createdJob *batchv1.Job
		testClient.
			On("Create", mock.Anything, mock.Anything, mock.Anything).
			Run(func(args mock.Arguments) {
				createdJob = args.Get(1).(*batchv1.Job)
			}).
			Return(nil)

		jobOwnerStrategy := &mockOwnerStrategy{}
		jobOwnerStrategy.On("SetControllerReference", mock.Anything, mock.Anything).Return(nil)

		r := &jobReconciler{
			scheme:           testScheme,
			newPackage:       newGenericPackage,
			client:           testClient,
			jobOwnerStrategy: jobOwnerStrategy,
		}

		desiredJob := pkg.RenderPackageLoaderJob()

		ctx := context.Background()
		res, err := r.Reconcile(ctx, pkg)
		require.NoError(t, err)
		require.Equal(t, res, ctrl.Result{})

		jobOwnerStrategy.AssertCalled(t, "SetControllerReference", pkg.ClientObject(), desiredJob)
		testClient.AssertCalled(t, "Create", mock.Anything, mock.Anything, mock.Anything)
		require.Equal(t, *desiredJob, *createdJob)
	})

	t.Run("job found with succeeded completion", func(t *testing.T) {
		testClient := testutil.NewClient()
		pkgName, pkgNamespace := "foo", "foo-ns"
		jobName, jobNamespace := fmt.Sprintf("job-%s", pkgName), "package-operator-system"

		pkg := &GenericPackage{
			corev1alpha1.Package{
				ObjectMeta: metav1.ObjectMeta{
					Name:      pkgName,
					Namespace: pkgNamespace,
				},
			},
		}

		succeededJobCondition := batchv1.JobCondition{
			Type:   batchv1.JobComplete,
			Status: corev1.ConditionTrue,
		}
		testClient.
			On("Get", mock.Anything, client.ObjectKey{Name: jobName, Namespace: jobNamespace}, mock.Anything, mock.Anything).
			Run(func(args mock.Arguments) {
				foundJobName, foundJobNamespace := args.Get(1).(types.NamespacedName).Name, args.Get(1).(types.NamespacedName).Namespace
				foundJob := &batchv1.Job{
					ObjectMeta: metav1.ObjectMeta{
						Name:      foundJobName,
						Namespace: foundJobNamespace,
					},
				}
				foundJob.Status.Conditions = append(foundJob.Status.Conditions, succeededJobCondition)

				destination := args.Get(2).(*batchv1.Job)
				foundJob.DeepCopyInto(destination)
			}).
			Return(nil)

		r := &jobReconciler{
			scheme:     testScheme,
			newPackage: newGenericPackage,
			client:     testClient,
		}

		ctx := context.Background()
		res, err := r.Reconcile(ctx, pkg)
		require.NoError(t, err)
		require.Equal(t, res, ctrl.Result{})

		condFound := meta.FindStatusCondition(*pkg.GetConditions(), corev1alpha1.PackageUnpacked)
		require.NotNil(t, condFound)

		expectedCondition := metav1.Condition{
			Type:               corev1alpha1.PackageUnpacked,
			Status:             metav1.ConditionTrue,
			Reason:             "PackageLoaderSucceeded",
			Message:            "Job to load the package succeeded",
			ObservedGeneration: pkg.ClientObject().GetGeneration(),
		}
		foundCondition := metav1.Condition{
			Type:               condFound.Type,
			Status:             condFound.Status,
			Reason:             condFound.Reason,
			Message:            condFound.Message,
			ObservedGeneration: condFound.ObservedGeneration,
		}
		require.Equal(t, expectedCondition, foundCondition)
	})

	t.Run("job found with failed completion", func(t *testing.T) {
		testClient := testutil.NewClient()
		pkgName, pkgNamespace := "foo", "foo-ns"
		jobName, jobNamespace := fmt.Sprintf("job-%s", pkgName), "package-operator-system"

		pkg := &GenericPackage{
			corev1alpha1.Package{
				ObjectMeta: metav1.ObjectMeta{
					Name:      pkgName,
					Namespace: pkgNamespace,
				},
			},
		}

		failedJobCondition := batchv1.JobCondition{
			Type:    batchv1.JobFailed,
			Status:  corev1.ConditionTrue,
			Message: "whoops something messed up!",
		}

		var foundJob *batchv1.Job
		testClient.
			On("Get", mock.Anything, client.ObjectKey{Name: jobName, Namespace: jobNamespace}, mock.Anything, mock.Anything).
			Run(func(args mock.Arguments) {
				foundJobName, foundJobNamespace := args.Get(1).(types.NamespacedName).Name, args.Get(1).(types.NamespacedName).Namespace
				foundJob = &batchv1.Job{
					ObjectMeta: metav1.ObjectMeta{
						Name:      foundJobName,
						Namespace: foundJobNamespace,
					},
				}
				foundJob.Status.Conditions = append(foundJob.Status.Conditions, failedJobCondition)

				destination := args.Get(2).(*batchv1.Job)
				foundJob.DeepCopyInto(destination)
			}).
			Return(nil)

		testClient.
			On("Delete", mock.Anything, mock.Anything, mock.Anything).
			Return(nil)

		r := &jobReconciler{
			scheme:     testScheme,
			newPackage: newGenericPackage,
			client:     testClient,
		}

		ctx := context.Background()
		res, err := r.Reconcile(ctx, pkg)
		require.NoError(t, err)
		require.Equal(t, res, ctrl.Result{})

		condFound := meta.FindStatusCondition(*pkg.GetConditions(), corev1alpha1.PackageUnpacked)
		require.NotNil(t, condFound)

		expectedCondition := metav1.Condition{
			Type:               corev1alpha1.PackageUnpacked,
			Status:             metav1.ConditionFalse,
			Reason:             "PackageLoaderFailed",
			Message:            fmt.Sprintf("Job to load the package failed: %s", failedJobCondition.Message),
			ObservedGeneration: pkg.ClientObject().GetGeneration(),
		}
		foundCondition := metav1.Condition{
			Type:               condFound.Type,
			Status:             condFound.Status,
			Reason:             condFound.Reason,
			Message:            condFound.Message,
			ObservedGeneration: condFound.ObservedGeneration,
		}
		require.Equal(t, expectedCondition, foundCondition)

		testClient.AssertCalled(t, "Delete", mock.Anything, foundJob, mock.Anything)
	})

	t.Run("job found to under progress", func(t *testing.T) {
		testClient := testutil.NewClient()
		pkgName, pkgNamespace := "foo", "foo-ns"
		jobName, jobNamespace := fmt.Sprintf("job-%s", pkgName), "package-operator-system"

		pkg := &GenericPackage{
			corev1alpha1.Package{
				ObjectMeta: metav1.ObjectMeta{
					Name:      pkgName,
					Namespace: pkgNamespace,
				},
			},
		}

		underProgressJobCondition := batchv1.JobCondition{
			Type:   batchv1.JobComplete,
			Status: corev1.ConditionFalse,
		}

		testClient.
			On("Get", mock.Anything, client.ObjectKey{Name: jobName, Namespace: jobNamespace}, mock.Anything, mock.Anything).
			Run(func(args mock.Arguments) {
				foundJobName, foundJobNamespace := args.Get(1).(types.NamespacedName).Name, args.Get(1).(types.NamespacedName).Namespace
				foundJob := &batchv1.Job{
					ObjectMeta: metav1.ObjectMeta{
						Name:      foundJobName,
						Namespace: foundJobNamespace,
					},
				}
				foundJob.Status.Conditions = append(foundJob.Status.Conditions, underProgressJobCondition)

				destination := args.Get(2).(*batchv1.Job)
				foundJob.DeepCopyInto(destination)
			}).
			Return(nil)

		r := &jobReconciler{
			scheme:     testScheme,
			newPackage: newGenericPackage,
			client:     testClient,
		}

		ctx := context.Background()
		res, err := r.Reconcile(ctx, pkg)
		require.NoError(t, err)
		require.Equal(t, res, ctrl.Result{})

		condFound := meta.FindStatusCondition(*pkg.GetConditions(), corev1alpha1.PackageUnpacked)
		require.NotNil(t, condFound)

		expectedCondition := metav1.Condition{
			Type:               corev1alpha1.PackageUnpacked,
			Status:             metav1.ConditionFalse,
			Reason:             "PackageLoaderInProgress",
			Message:            "Job to load the package is in progress",
			ObservedGeneration: pkg.ClientObject().GetGeneration(),
		}
		foundCondition := metav1.Condition{
			Type:               condFound.Type,
			Status:             condFound.Status,
			Reason:             condFound.Reason,
			Message:            condFound.Message,
			ObservedGeneration: condFound.ObservedGeneration,
		}
		require.Equal(t, expectedCondition, foundCondition)
	})
}
