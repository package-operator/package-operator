package rolloutcmd

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
	internalcmd "package-operator.run/internal/cmd"
)

func TestRollbackCmd(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		Args          []string
		ActualObjects []client.Object
		Output        string
		ShouldFail    bool
	}{
		"no args": {
			ShouldFail: true,
		},
		"too many args": {
			Args:       []string{"1", "2", "3"},
			ShouldFail: true,
		},
		"invalid args": {
			Args:       []string{"invalid"},
			ShouldFail: true,
		},
		"clusterpackage rollback with available status": {
			Args: []string{"clusterpackage/test", "--revision", "1"},
			ActualObjects: []client.Object{
				&corev1alpha1.ClusterPackage{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
				},
				&corev1alpha1.ClusterObjectSet{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
						Labels: map[string]string{
							manifestsv1alpha1.PackageInstanceLabel: "test",
						},
					},
					Status: corev1alpha1.ClusterObjectSetStatus{
						Phase:    corev1alpha1.ObjectSetStatusPhaseAvailable,
						Revision: 1,
					},
				},
			},
			ShouldFail: true,
			Output:     "the clusterobjectset specified via --revision is currently active and cannot be rolled back",
		},
		"clusterpackage rollback with archived status": {
			Args: []string{"clusterpackage/test", "--revision", "1"},
			ActualObjects: []client.Object{
				&corev1alpha1.ClusterPackage{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
				},
				&corev1alpha1.ClusterObjectDeployment{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
						Labels: map[string]string{
							manifestsv1alpha1.PackageInstanceLabel: "test",
						},
					},
				},
				&corev1alpha1.ClusterObjectSet{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-1",
						Labels: map[string]string{
							manifestsv1alpha1.PackageInstanceLabel: "test",
						},
					},
					Status: corev1alpha1.ClusterObjectSetStatus{
						Phase:    corev1alpha1.ObjectSetStatusPhaseArchived,
						Revision: 1,
					},
				},
				&corev1alpha1.ClusterObjectSet{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-2",
						Labels: map[string]string{
							manifestsv1alpha1.PackageInstanceLabel: "test",
						},
					},
					Status: corev1alpha1.ClusterObjectSetStatus{
						Phase:    corev1alpha1.ObjectSetStatusPhaseAvailable,
						Revision: 2,
					},
				},
			},
			ShouldFail: false,
			Output:     "rolling back ClusterObjectDeployment : test with objectset test-1 Successfully rolled back\n",
		},
		"clusterobjectdeployment rollback with archived status": {
			Args: []string{"ClusterObjectDeployment/test", "--revision", "1"},
			ActualObjects: []client.Object{
				&corev1alpha1.ClusterPackage{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
				},
				&corev1alpha1.ClusterObjectDeployment{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
						Labels: map[string]string{
							manifestsv1alpha1.PackageInstanceLabel: "test",
						},
					},
				},
				&corev1alpha1.ClusterObjectSet{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-1",
						Labels: map[string]string{
							manifestsv1alpha1.PackageInstanceLabel: "test",
						},
					},
					Status: corev1alpha1.ClusterObjectSetStatus{
						Phase:    corev1alpha1.ObjectSetStatusPhaseArchived,
						Revision: 1,
					},
				},
				&corev1alpha1.ClusterObjectSet{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-2",
						Labels: map[string]string{
							manifestsv1alpha1.PackageInstanceLabel: "test",
						},
					},
					Status: corev1alpha1.ClusterObjectSetStatus{
						Phase:    corev1alpha1.ObjectSetStatusPhaseAvailable,
						Revision: 2,
					},
				},
			},
			ShouldFail: false,
			Output:     "rolling back ClusterObjectDeployment : test with objectset test-1 Successfully rolled back\n",
		},

		"package rollback with archived status": {
			Args: []string{"package/test", "--namespace", "test", "--revision", "1"},
			ActualObjects: []client.Object{
				&corev1alpha1.Package{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: "test",
					},
				},
				&corev1alpha1.ObjectDeployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: "test",
						Labels: map[string]string{
							manifestsv1alpha1.PackageInstanceLabel: "test",
						},
					},
				},
				&corev1alpha1.ObjectSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-1",
						Namespace: "test",
						Labels: map[string]string{
							manifestsv1alpha1.PackageInstanceLabel: "test",
						},
					},
					Status: corev1alpha1.ObjectSetStatus{
						Phase:    corev1alpha1.ObjectSetStatusPhaseArchived,
						Revision: 1,
					},
				},
				&corev1alpha1.ObjectSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-2",
						Namespace: "test",
						Labels: map[string]string{
							manifestsv1alpha1.PackageInstanceLabel: "test",
						},
					},
					Status: corev1alpha1.ObjectSetStatus{
						Phase:    corev1alpha1.ObjectSetStatusPhaseAvailable,
						Revision: 2,
					},
				},
			},
			ShouldFail: false,
			Output: "rolling back ObjectDeployment : test" +
				" in ns : test with Objectset : test-1 Successfully rolled back\n",
		},

		"package provided but revision is not provided": {
			Args: []string{"package/test", "--namespace", "test"},
			ActualObjects: []client.Object{
				&corev1alpha1.Package{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: "test",
					},
				},
				&corev1alpha1.ObjectDeployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: "test",
						Labels: map[string]string{
							manifestsv1alpha1.PackageInstanceLabel: "test",
						},
					},
				},
				&corev1alpha1.ObjectSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-1",
						Namespace: "test",
						Labels: map[string]string{
							manifestsv1alpha1.PackageInstanceLabel: "test",
						},
					},
					Status: corev1alpha1.ObjectSetStatus{
						Phase:    corev1alpha1.ObjectSetStatusPhaseArchived,
						Revision: 1,
					},
				},
				&corev1alpha1.ObjectSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-2",
						Namespace: "test",
						Labels: map[string]string{
							manifestsv1alpha1.PackageInstanceLabel: "test",
						},
					},
					Status: corev1alpha1.ObjectSetStatus{
						Phase:    corev1alpha1.ObjectSetStatusPhaseAvailable,
						Revision: 2,
					},
				},
			},
			ShouldFail: true,
			Output:     "--revision is required as the rollback requires to provide a revision",
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			scheme, err := internalcmd.NewScheme()
			require.NoError(t, err)

			c := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(tc.ActualObjects...).
				Build()

			cmd := NewRollbackCmd(internalcmd.NewDefaultClientFactory(
				&kubeClientFactoryMock{
					Client: c,
				},
			))
			cmd.SetArgs(tc.Args)

			var (
				out    bytes.Buffer
				errout bytes.Buffer
			)
			cmd.SetOut(&out)
			cmd.SetErr(&errout)

			if tc.ShouldFail {
				require.Error(t, cmd.Execute())

				return
			}
			require.NoError(t, cmd.Execute())
			assert.Equal(t, tc.Output, out.String())
		})
	}
}
