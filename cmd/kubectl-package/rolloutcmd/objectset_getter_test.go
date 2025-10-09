package rolloutcmd

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
	internalcmd "package-operator.run/internal/cmd"
)

func TestObjectSetGetter_GetObjectSets(t *testing.T) {
	t.Parallel()

	type expected struct {
		ObjectSetList  internalcmd.ObjectSetList
		ErrorAssertion require.ErrorAssertionFunc
	}

	for name, tc := range map[string]struct {
		ActualObjects []client.Object
		Type          string
		Name          string
		Namespace     string
		Expected      expected
	}{
		"invalid resource type": {
			Type: "invalid",
			Expected: expected{
				ErrorAssertion: require.Error,
			},
		},
		"no clusterpackage objects": {
			Type: "clusterpackage",
			Name: "test",
			Expected: expected{
				ErrorAssertion: require.Error,
			},
		},
		"no package objects": {
			Type: "package",
			Name: "test",
			Expected: expected{
				ErrorAssertion: require.Error,
			},
		},
		"no clusterobjectdeployment objects": {
			Type: "clusterobjectdeployment",
			Name: "test",
			Expected: expected{
				ErrorAssertion: require.Error,
			},
		},
		"no objectdeployment objects": {
			Type: "objectdeployment",
			Name: "test",
			Expected: expected{
				ErrorAssertion: require.Error,
			},
		},
		"clusterpackage with objectsets": {
			Type: "clusterpackage",
			Name: "test",
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
			Expected: expected{
				ErrorAssertion: require.NoError,
				ObjectSetList: []internalcmd.ObjectSet{
					internalcmd.NewObjectSet(
						&corev1alpha1.ClusterObjectSet{
							ObjectMeta: metav1.ObjectMeta{
								Name: "test",
							},
							Spec: corev1alpha1.ClusterObjectSetSpec{
								Revision: 1,
							},
						},
					),
				},
			},
		},
		"package with objectsets": {
			Type:      "package",
			Name:      "test",
			Namespace: "test",
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
			Expected: expected{
				ErrorAssertion: require.NoError,
				ObjectSetList: []internalcmd.ObjectSet{
					internalcmd.NewObjectSet(
						&corev1alpha1.ObjectSet{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "test",
								Namespace: "test",
							},
							Spec: corev1alpha1.ObjectSetSpec{
								Revision: 1,
							},
						},
					),
				},
			},
		},
		"clusterobjectdeployment with objectsets": {
			Type: "clusterobjectdeployment",
			Name: "test",
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
			Expected: expected{
				ErrorAssertion: require.NoError,
				ObjectSetList: []internalcmd.ObjectSet{
					internalcmd.NewObjectSet(
						&corev1alpha1.ClusterObjectSet{
							ObjectMeta: metav1.ObjectMeta{
								Name: "test",
							},
							Spec: corev1alpha1.ClusterObjectSetSpec{
								Revision: 1,
							},
						},
					),
				},
			},
		},
		"objectdeployment with objectsets": {
			Type:      "objectdeployment",
			Name:      "test",
			Namespace: "test",
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
			Expected: expected{
				ErrorAssertion: require.NoError,
				ObjectSetList: []internalcmd.ObjectSet{
					internalcmd.NewObjectSet(
						&corev1alpha1.ObjectSet{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "test",
								Namespace: "test",
							},
							Spec: corev1alpha1.ObjectSetSpec{
								Revision: 1,
							},
						},
					),
				},
			},
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

			getter := newObjectSetGetter(internalcmd.NewClient(c))

			list, err := getter.GetObjectSets(context.Background(), tc.Type, tc.Name, tc.Namespace)
			tc.Expected.ErrorAssertion(t, err)

			requireEqualObjectSetLists(t, tc.Expected.ObjectSetList, list)
		})
	}
}

func requireEqualObjectSetLists(t *testing.T, a, b internalcmd.ObjectSetList) {
	t.Helper()

	type item struct {
		Name      string
		Namespace string
		Revision  int64
	}

	listA := make([]item, 0, len(a))
	for _, os := range a {
		listA = append(listA, item{
			Name:      os.Name(),
			Namespace: os.Namespace(),
			Revision:  os.Revision(),
		})
	}

	listB := make([]item, 0, len(b))
	for _, os := range b {
		listB = append(listB, item{
			Name:      os.Name(),
			Namespace: os.Namespace(),
			Revision:  os.Revision(),
		})
	}

	require.ElementsMatch(t, listA, listB)
}
