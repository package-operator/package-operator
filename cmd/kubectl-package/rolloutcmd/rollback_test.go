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

func TestRollbackCmd(t *testing.T) { //nolint:maintidx
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
		"clusterpackage": {
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
			ShouldFail: false,
			Output:     "Can not rollback from an available ClusterObjectSet Type"},
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
