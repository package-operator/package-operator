package objectsets

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/package-operator/internal/testutil"
)

func TestObjectSetRemotePhaseReconciler_Teardown(t *testing.T) {
	tests := []struct {
		name        string
		mockPrepare func(clientMock *testutil.CtrlClient)
		cleanupDone bool
		assertCalls func(t *testing.T, clientMock *testutil.CtrlClient)
	}{
		{
			name: "deletes object",
			mockPrepare: func(clientMock *testutil.CtrlClient) {
				clientMock.
					On("Get", mock.Anything, mock.Anything, mock.Anything).
					Return(nil)
				clientMock.
					On("Delete", mock.Anything, mock.Anything, mock.Anything).
					Return(nil)
			},
			cleanupDone: false,
			assertCalls: func(t *testing.T, clientMock *testutil.CtrlClient) {
				t.Helper()
				clientMock.AssertCalled(
					t, "Delete", mock.Anything, mock.Anything, mock.Anything)
			},
		},
		{
			name: "already gone",
			mockPrepare: func(clientMock *testutil.CtrlClient) {
				clientMock.
					On("Get", mock.Anything, mock.Anything, mock.Anything).
					Return(errors.NewNotFound(schema.GroupResource{}, ""))
				clientMock.
					On("Delete", mock.Anything, mock.Anything, mock.Anything).
					Return(nil)
			},
			cleanupDone: true,
			assertCalls: func(t *testing.T, clientMock *testutil.CtrlClient) {
				t.Helper()
				clientMock.AssertCalled(
					t, "Get", mock.Anything, mock.Anything, mock.Anything)
				clientMock.AssertNotCalled(
					t, "Delete", mock.Anything, mock.Anything, mock.Anything)
			},
		},
		{
			name: "already gone on delete",
			mockPrepare: func(clientMock *testutil.CtrlClient) {
				clientMock.
					On("Get", mock.Anything, mock.Anything, mock.Anything).
					Return(nil)
				clientMock.
					On("Delete", mock.Anything, mock.Anything, mock.Anything).
					Return(errors.NewNotFound(schema.GroupResource{}, ""))
			},
			cleanupDone: true,
			assertCalls: func(t *testing.T, clientMock *testutil.CtrlClient) {
				t.Helper()
				clientMock.AssertCalled(
					t, "Delete", mock.Anything, mock.Anything, mock.Anything)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			clientMock := testutil.NewClient()
			r := &objectSetRemotePhaseReconciler{
				client:            clientMock,
				scheme:            testScheme,
				newObjectSetPhase: newGenericObjectSetPhase,
			}

			genObjectSet := newGenericObjectSet(testScheme)
			objectSet := genObjectSet.ClientObject().(*corev1alpha1.ObjectSet)
			objectSet.Name = "my-stuff"
			objectSet.Namespace = "my-namespace"

			phase := corev1alpha1.ObjectSetTemplatePhase{
				Name: "phase-1",
			}

			test.mockPrepare(clientMock)

			ctx := context.Background()
			cleanupDone, err := r.Teardown(ctx, genObjectSet, phase)
			require.NoError(t, err)
			assert.Equal(t, test.cleanupDone, cleanupDone)

			test.assertCalls(t, clientMock)
		})
	}
}

func TestObjectSetRemotePhaseReconciler_desiredObjectSetPhase(
	t *testing.T) {
	r := &objectSetRemotePhaseReconciler{
		scheme:            testScheme,
		newObjectSetPhase: newGenericObjectSetPhase,
	}

	genObjectSet := newGenericObjectSet(testScheme)
	objectSet := genObjectSet.ClientObject().(*corev1alpha1.ObjectSet)
	objectSet.Name = "my-stuff"
	objectSet.Namespace = "my-namespace"
	objectSet.Spec.AvailabilityProbes = []corev1alpha1.ObjectSetProbe{
		{
			Selector: corev1alpha1.ProbeSelector{
				Kind: &corev1alpha1.PackageProbeKindSpec{},
			},
		},
	}
	objectSet.Status.Revision = 15
	objectSet.Spec.Previous = []corev1alpha1.PreviousRevisionReference{
		{
			Name: "test-1",
		},
	}
	objectSet.Spec.LifecycleState = corev1alpha1.ObjectSetLifecycleStatePaused

	phase := corev1alpha1.ObjectSetTemplatePhase{
		Name: "phase-1",
	}

	genObjectSetPhase, err := r.desiredObjectSetPhase(genObjectSet, phase)
	require.NoError(t, err)
	assert.NotNil(t, genObjectSetPhase)
	objectSetPhase := genObjectSetPhase.
		ClientObject().(*corev1alpha1.ObjectSetPhase)

	assert.Equal(t, phase, objectSetPhase.Spec.ObjectSetTemplatePhase)
	assert.Equal(t, objectSet.Spec.AvailabilityProbes, objectSetPhase.Spec.AvailabilityProbes)
	assert.Equal(t, objectSet.Status.Revision, objectSetPhase.Spec.Revision)
	assert.Equal(t, objectSet.Spec.Previous, objectSetPhase.Spec.Previous)
	assert.True(t, objectSetPhase.Spec.Paused)
	assert.NotEmpty(t, objectSetPhase.GetOwnerReferences())
	assert.Equal(t, "my-stuff-phase-1", objectSetPhase.Name)
	assert.Equal(t, objectSet.Namespace, objectSetPhase.Namespace)
}
