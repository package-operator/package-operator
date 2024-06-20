package clustertreecmd

import (
	"bytes"
	"testing"

	corev1 "k8s.io/api/core/v1"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
	internalcmd "package-operator.run/internal/cmd"
)

func TestClusterTreeCmd(t *testing.T) {
	t.Parallel()
	const expectedOutput = `ClusterPackage /test
└── Phase phase-1
│   ├── /v1, Kind=ConfigMap /cm-4
└── Phase phase-2
    └── /v1, Kind=Namespace /ns1
`

	const expectedPackageOutput = `Package /test
namespace/test
└── Phase phase-1
    └── /v1, Kind=ConfigMap /cm-4
`
	cm4 := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "cm-4",
			Labels: map[string]string{"test.package-operator.run/test": "True"},
		},
		Data: map[string]string{
			"banana": "bread",
		},
	}
	ns1 := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "ns1",
			Labels: map[string]string{"test.package-operator.run/test": "True"},
		},
	}
	cm4.Kind = "ConfigMap"
	cm4.APIVersion = "v1"
	ns1.Kind = "Namespace"
	ns1.APIVersion = "v1"

	cm4Obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(cm4)
	if err != nil {
		t.Fatal(err)
	}
	ns1Obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(ns1)
	if err != nil {
		t.Fatal(err)
	}

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
						ObjectSetTemplateSpec: corev1alpha1.ObjectSetTemplateSpec{
							Phases: []corev1alpha1.ObjectSetTemplatePhase{{
								Name: "phase-1",
								Objects: []corev1alpha1.ObjectSetObject{
									{
										Object: unstructured.Unstructured{Object: cm4Obj},
									},
								},
							}, {
								Name: "phase-2",
								Objects: []corev1alpha1.ObjectSetObject{
									{
										Object: unstructured.Unstructured{Object: ns1Obj},
									},
								},
							}},
						},
					},
					Status: corev1alpha1.ClusterObjectSetStatus{
						Phase: "Available",
					},
				},
			},
			ShouldFail: false,
			Output:     expectedOutput,
		},
		"package": {
			Args: []string{"package/test", "--namespace", "test"},
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
						ObjectSetTemplateSpec: corev1alpha1.ObjectSetTemplateSpec{
							Phases: []corev1alpha1.ObjectSetTemplatePhase{{
								Name: "phase-1",
								Objects: []corev1alpha1.ObjectSetObject{
									{
										Object: unstructured.Unstructured{Object: cm4Obj},
									},
								},
							}},
						},
					},

					Status: corev1alpha1.ObjectSetStatus{
						Revision: 1,
					},
				},
			},
			ShouldFail: false,
			Output:     expectedPackageOutput,
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

			cmd := NewClusterTreeCmd(internalcmd.NewDefaultClientFactory(
				&kubeClientFactoryMock{
					Client: c,
				},
			))
			cmd.SetArgs(tc.Args)

			stdout := &bytes.Buffer{}
			stderr := &bytes.Buffer{}
			cmd.SetOut(stdout)
			cmd.SetErr(stderr)
			if tc.ShouldFail {
				require.Error(t, cmd.Execute())

				return
			}
			require.NoError(t, cmd.Execute())
			out := stdout.String()
			assert.Equal(t, tc.Output, out)
		})
	}
}

type kubeClientFactoryMock struct {
	Client client.Client
}

func (m *kubeClientFactoryMock) GetKubeClient() (client.Client, error) {
	return m.Client, nil
}
