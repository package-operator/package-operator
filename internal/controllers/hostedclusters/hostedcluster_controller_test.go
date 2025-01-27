package hostedclusters

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	hypershiftv1beta1 "package-operator.run/internal/controllers/hostedclusters/hypershift/v1beta1"
	"package-operator.run/internal/testutil"
)

var testScheme = runtime.NewScheme()

func init() {
	if err := corev1alpha1.AddToScheme(testScheme); err != nil {
		panic(err)
	}
	if err := hypershiftv1beta1.AddToScheme(testScheme); err != nil {
		panic(err)
	}
}

func TestHostedClusterController_noop(t *testing.T) {
	t.Parallel()

	mockClient := testutil.NewClient()

	image := "image321"
	controller := NewHostedClusterController(
		mockClient, ctrl.Log.WithName("hc controller test"), testScheme, image, nil, nil,
	)
	hcName := "testing123"
	now := metav1.Now()
	hc := &hypershiftv1beta1.HostedCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:              hcName,
			DeletionTimestamp: &now,
		},
	}

	mockClient.
		On("Get", mock.Anything, mock.Anything, mock.AnythingOfType("*v1beta1.HostedCluster"), mock.Anything).
		Run(func(args mock.Arguments) {
			obj := args.Get(2).(*hypershiftv1beta1.HostedCluster)
			*obj = *hc
		}).
		Return(nil)

	ctx := context.Background()
	res, err := controller.Reconcile(ctx, ctrl.Request{
		NamespacedName: client.ObjectKeyFromObject(hc),
	})
	require.NoError(t, err)
	assert.True(t, res.IsZero())
}

func TestHostedClusterController_DesiredPackage(t *testing.T) {
	t.Parallel()

	mockClient := testutil.NewClient()

	image := "image321"
	controller := NewHostedClusterController(mockClient, ctrl.Log.WithName("hc controller test"), testScheme, image,
		&corev1.Affinity{}, []corev1.Toleration{{}})
	hcName := "testing123"
	hc := &hypershiftv1beta1.HostedCluster{
		ObjectMeta: metav1.ObjectMeta{Name: hcName, Namespace: "default"},
	}

	pkg, err := controller.desiredRemotePhasePackage(hc)
	require.NoError(t, err)
	assert.Equal(t, "remote-phase", pkg.Name)
	assert.Equal(t, image, pkg.Spec.Image)
	if assert.NotNil(t, pkg.Spec.Config) {
		assert.JSONEq(t, `{"affinity":{},"tolerations":[{}]}`, string(pkg.Spec.Config.Raw))
	}
}

var readyHostedCluster = &hypershiftv1beta1.HostedCluster{
	Status: hypershiftv1beta1.HostedClusterStatus{
		Conditions: []metav1.Condition{
			{Type: hypershiftv1beta1.HostedClusterAvailable, Status: metav1.ConditionTrue},
		},
	},
}

func TestHostedClusterController_Reconcile_GetHostedClusterError(t *testing.T) {
	t.Parallel()

	fooErr := errors.NewBadRequest("foo")

	clientMock := testutil.NewClient()
	c := NewHostedClusterController(
		clientMock, ctrl.Log.WithName("hc controller test"), testScheme, "desired-image:test", nil, nil,
	)

	clientMock.
		On("Get", mock.Anything, mock.Anything, mock.AnythingOfType("*v1beta1.HostedCluster"), mock.Anything).
		Return(fooErr)

	res, err := c.Reconcile(context.Background(), ctrl.Request{})
	require.EqualError(t, err, fooErr.Error())
	assert.Empty(t, res)

	clientMock.AssertExpectations(t)
}

func TestHostedClusterController_Reconcile_DontHandleDeleted(t *testing.T) {
	t.Parallel()

	clientMock := testutil.NewClient()
	c := NewHostedClusterController(
		clientMock, ctrl.Log.WithName("hc controller test"), testScheme, "desired-image:test", nil, nil,
	)

	clientMock.
		On("Get", mock.Anything, mock.Anything, mock.AnythingOfType("*v1beta1.HostedCluster"), mock.Anything).
		Run(func(args mock.Arguments) {
			hc := args.Get(2).(*hypershiftv1beta1.HostedCluster)
			hc.DeletionTimestamp = &metav1.Time{Time: time.Now()}
		}).
		Return(nil)

	res, err := c.Reconcile(context.Background(), ctrl.Request{})
	require.NoError(t, err)
	assert.Empty(t, res)

	clientMock.AssertExpectations(t)
}

func TestHostedClusterController_Reconcile_waitsForClusterReady(t *testing.T) {
	t.Parallel()

	clientMock := testutil.NewClient()
	c := NewHostedClusterController(
		clientMock, ctrl.Log.WithName("hc controller test"), testScheme, "desired-image:test", nil, nil,
	)

	clientMock.
		On("Get", mock.Anything, mock.Anything, mock.AnythingOfType("*v1beta1.HostedCluster"), mock.Anything).
		Return(nil)

	clientMock.
		On("Create", mock.Anything, mock.Anything, mock.Anything).
		Return(nil)

	res, err := c.Reconcile(context.Background(), ctrl.Request{})
	require.NoError(t, err)
	assert.Empty(t, res)

	clientMock.AssertNotCalled(t, "Create", mock.Anything, mock.AnythingOfType("*v1alpha1.Package"), mock.Anything)
}

func TestHostedClusterController_Reconcile_createsPackage(t *testing.T) {
	t.Parallel()

	clientMock := testutil.NewClient()
	c := NewHostedClusterController(
		clientMock, ctrl.Log.WithName("hc controller test"), testScheme, "desired-image:test", nil, nil,
	)

	clientMock.
		On("Get", mock.Anything, mock.Anything, mock.AnythingOfType("*v1beta1.HostedCluster"), mock.Anything).
		Run(func(args mock.Arguments) {
			obj := args.Get(2).(*hypershiftv1beta1.HostedCluster)
			*obj = *readyHostedCluster.DeepCopy()
		}).
		Return(nil)

	clientMock.
		On("Get", mock.Anything, mock.Anything, mock.AnythingOfType("*v1alpha1.Package"), mock.Anything).
		Return(errors.NewNotFound(schema.GroupResource{}, ""))

	clientMock.
		On("Create", mock.Anything, mock.Anything, mock.Anything).
		Return(nil)

	res, err := c.Reconcile(context.Background(), ctrl.Request{})
	require.NoError(t, err)
	assert.Empty(t, res)

	clientMock.AssertCalled(t, "Create", mock.Anything, mock.AnythingOfType("*v1alpha1.Package"), mock.Anything)
}

func packageConfig(
	t *testing.T,
	remotePhaseAffinity *corev1.Affinity,
	remotePhaseTolerations []corev1.Toleration,
) *runtime.RawExtension {
	t.Helper()

	config := map[string]any{}
	if remotePhaseAffinity != nil {
		config["affinity"] = remotePhaseAffinity
	}
	if remotePhaseTolerations != nil {
		config["tolerations"] = remotePhaseTolerations
	}

	configJSON, err := json.Marshal(config)
	require.NoError(t, err)
	return &runtime.RawExtension{
		Raw: configJSON,
	}
}

func TestHostedClusterController_Reconcile_updatesPackageSpec(t *testing.T) {
	t.Parallel()

	type tcase struct {
		old, expectedNew            corev1alpha1.PackageSpec
		packageOperatorPackageImage string
		remotePhaseAffinity         *corev1.Affinity
		remotePhaseTolerations      []corev1.Toleration
	}

	outdatedImage := "image:outdated"
	latestImage := "image:latest"
	expectedComponent := "remote-phase"

	tcases := []tcase{
		{
			old: corev1alpha1.PackageSpec{
				Image: outdatedImage,
			},
			expectedNew: corev1alpha1.PackageSpec{
				Image:     latestImage,
				Component: expectedComponent,
			},
			packageOperatorPackageImage: latestImage,
			remotePhaseAffinity:         nil,
			remotePhaseTolerations:      nil,
		},
		{
			old: corev1alpha1.PackageSpec{
				Image:     outdatedImage,
				Component: "foo",
			},
			expectedNew: corev1alpha1.PackageSpec{
				Image:     latestImage,
				Component: expectedComponent,
			},
			packageOperatorPackageImage: latestImage,
			remotePhaseAffinity:         nil,
			remotePhaseTolerations:      nil,
		},
		{
			old: corev1alpha1.PackageSpec{
				Image: outdatedImage,
			},
			expectedNew: corev1alpha1.PackageSpec{
				Image:     latestImage,
				Component: expectedComponent,
				Config: packageConfig(t, &corev1.Affinity{
					PodAntiAffinity: &corev1.PodAntiAffinity{
						RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
							{
								LabelSelector: &metav1.LabelSelector{
									MatchLabels: map[string]string{
										"foo": "bar",
									},
								},
							},
						},
					},
				}, nil),
			},
			packageOperatorPackageImage: latestImage,
			remotePhaseAffinity: &corev1.Affinity{
				PodAntiAffinity: &corev1.PodAntiAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
						{
							LabelSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"foo": "bar",
								},
							},
						},
					},
				},
			},
			remotePhaseTolerations: nil,
		},
	}

	for i := range tcases {
		tcase := tcases[i]

		clientMock := testutil.NewClient()
		c := NewHostedClusterController(
			clientMock,
			ctrl.Log.WithName("hc controller test"),
			testScheme,
			tcase.packageOperatorPackageImage,
			tcase.remotePhaseAffinity,
			tcase.remotePhaseTolerations,
		)

		clientMock.
			On("Get", mock.Anything, mock.Anything, mock.AnythingOfType("*v1beta1.HostedCluster"), mock.Anything).
			Run(func(args mock.Arguments) {
				obj := args.Get(2).(*hypershiftv1beta1.HostedCluster)
				*obj = *readyHostedCluster.DeepCopy()
			}).
			Return(nil)

		clientMock.
			On("Get", mock.Anything, mock.Anything, mock.AnythingOfType("*v1alpha1.Package"), mock.Anything).
			Run(func(args mock.Arguments) {
				obj := args.Get(2).(*corev1alpha1.Package)
				*obj = corev1alpha1.Package{
					Spec: tcase.old,
				}
			}).
			Return(nil)

		clientMock.
			On("Update", mock.Anything, mock.AnythingOfType("*v1alpha1.Package"), mock.Anything).
			Run(func(args mock.Arguments) {
				obj := args.Get(1).(*corev1alpha1.Package)
				require.Equal(t, tcase.expectedNew, obj.Spec)
			}).
			Return(nil)

		res, err := c.Reconcile(context.Background(), ctrl.Request{})
		require.NoError(t, err)
		assert.Empty(t, res)

		clientMock.AssertExpectations(t)
	}
}
