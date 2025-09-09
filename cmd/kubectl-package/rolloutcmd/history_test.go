package rolloutcmd

import (
	"bytes"
	"strings"
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

func TestHistoryCmd(t *testing.T) { //nolint:maintidx
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
			Args: []string{"clusterpackage", "test"},
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
					Spec: corev1alpha1.ClusterObjectSetSpec{
						Revision: 1,
					},
				},
			},
			ShouldFail: false,
			Output: strings.Join([]string{
				"REVISION  SUCCESSFUL  CHANGE-CAUSE",
				"1         false                   ",
				"",
				"",
			}, "\n"),
		},
		"package": {
			Args: []string{"package", "test", "--namespace", "test"},
			ActualObjects: []client.Object{
				&corev1alpha1.Package{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: "test",
					},
				},
				&corev1alpha1.ObjectSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: "test",
						Labels: map[string]string{
							manifestsv1alpha1.PackageInstanceLabel: "test",
						},
					},
					Spec: corev1alpha1.ObjectSetSpec{
						Revision: 1,
					},
				},
			},
			ShouldFail: false,
			Output: strings.Join([]string{
				"REVISION  SUCCESSFUL  CHANGE-CAUSE",
				"1         false                   ",
				"",
				"",
			}, "\n"),
		},
		"clusterobjectdeployment": {
			Args: []string{"clusterobjectdeployment", "test"},
			ActualObjects: []client.Object{
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
						Name: "test",
						Labels: map[string]string{
							manifestsv1alpha1.PackageInstanceLabel: "test",
						},
					},
					Spec: corev1alpha1.ClusterObjectSetSpec{
						Revision: 1,
					},
				},
			},
			ShouldFail: false,
			Output: strings.Join([]string{
				"REVISION  SUCCESSFUL  CHANGE-CAUSE",
				"1         false                   ",
				"",
				"",
			}, "\n"),
		},
		"objectdeployment": {
			Args: []string{"objectdeployment", "test", "--namespace", "test"},
			ActualObjects: []client.Object{
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
						Name:      "test",
						Namespace: "test",
						Labels: map[string]string{
							manifestsv1alpha1.PackageInstanceLabel: "test",
						},
					},
					Spec: corev1alpha1.ObjectSetSpec{
						Revision: 1,
					},
				},
			},
			ShouldFail: false,
			Output: strings.Join([]string{
				"REVISION  SUCCESSFUL  CHANGE-CAUSE",
				"1         false                   ",
				"",
				"",
			}, "\n"),
		},
		"single arg": {
			Args: []string{"clusterpackage/test"},
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
					Spec: corev1alpha1.ClusterObjectSetSpec{
						Revision: 1,
					},
				},
			},
			ShouldFail: false,
			Output: strings.Join([]string{
				"REVISION  SUCCESSFUL  CHANGE-CAUSE",
				"1         false                   ",
				"",
				"",
			}, "\n"),
		},
		"all revs/json": {
			Args: []string{"clusterpackage/test", "--output", "json"},
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
					Spec: corev1alpha1.ClusterObjectSetSpec{
						Revision: 1,
					},
				},
			},
			ShouldFail: false,
			Output: strings.Join([]string{
				"[",
				"    {",
				`        "metadata": {`,
				`            "name": "test",`,
				`            "resourceVersion": "999",`,
				`            "labels": {`,
				`                "package-operator.run/instance": "test"`,
				"            }",
				"        },",
				`        "spec": {`,
				`            "revision": 1`,
				"        },",
				`        "status": {}`,
				"    }",
				"]",
				"",
			}, "\n"),
		},
		"all revs/yaml": {
			Args: []string{"clusterpackage/test", "--output", "yaml"},
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
					Spec: corev1alpha1.ClusterObjectSetSpec{
						Revision: 1,
					},
				},
			},
			ShouldFail: false,
			Output: strings.Join([]string{
				"- metadata:",
				"    labels:",
				"      package-operator.run/instance: test",
				"    name: test",
				`    resourceVersion: "999"`,
				"  spec:",
				"    revision: 1",
				"  status: {}",
				"",
			}, "\n"),
		},
		"single rev": {
			Args: []string{"clusterpackage/test", "--revision", "2"},
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
					Spec: corev1alpha1.ClusterObjectSetSpec{
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
					Spec: corev1alpha1.ClusterObjectSetSpec{
						Revision: 2,
					},
				},
			},
			ShouldFail: false,
			Output: strings.Join([]string{
				"metadata:",
				"  labels:",
				"    package-operator.run/instance: test",
				"  name: test-2",
				`  resourceVersion: "999"`,
				"spec:",
				"  revision: 2",
				"status: {}",
				"",
			}, "\n"),
		},
		"single rev/json": {
			Args: []string{"clusterpackage/test", "--revision", "2", "--output", "json"},
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
					Spec: corev1alpha1.ClusterObjectSetSpec{
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
					Spec: corev1alpha1.ClusterObjectSetSpec{
						Revision: 2,
					},
				},
			},
			ShouldFail: false,
			Output: strings.Join([]string{
				"{",
				`    "metadata": {`,
				`        "name": "test-2",`,
				`        "resourceVersion": "999",`,
				`        "labels": {`,
				`            "package-operator.run/instance": "test"`,
				"        }",
				"    },",
				`    "spec": {`,
				`        "revision": 2`,
				"    },",
				`    "status": {}`,
				"}",
				"",
			}, "\n"),
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

			cmd := NewHistoryCmd(internalcmd.NewDefaultClientFactory(
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

type kubeClientFactoryMock struct {
	Client client.Client
}

func (m *kubeClientFactoryMock) GetKubeClient() (client.Client, error) {
	return m.Client, nil
}
