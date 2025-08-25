package cmd

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
)

func TestClient_GetObjectset(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		ActualObjects []client.Object
		Assertion     require.ErrorAssertionFunc
		PackageName   string
		Namespace     string
	}{
		"package not found": {
			Assertion:   require.Error,
			PackageName: "dne",
		},
		"Archived Object Set with Package present": {
			ActualObjects: []client.Object{
				&corev1alpha1.ObjectSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-objset-archived",
						Namespace: "default",
					},
					Status: corev1alpha1.ObjectSetStatus{
						Conditions: []metav1.Condition{
							{
								Type:   corev1alpha1.ObjectSetArchived,
								Status: metav1.ConditionTrue,
							},
						},
					},
				},
			},
			Assertion:   require.Error,
			PackageName: "test-objset",
			Namespace:   "default",
		},
		"Package found with available objectset ": {
			ActualObjects: []client.Object{
				&corev1alpha1.ObjectSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-objset-archived",
						Namespace: "default",
					},
					Status: corev1alpha1.ObjectSetStatus{
						Conditions: []metav1.Condition{
							{
								Type:   corev1alpha1.ObjectSetInTransition,
								Status: metav1.ConditionTrue,
							},
							{
								Type:   corev1alpha1.ObjectSetAvailable,
								Status: metav1.ConditionTrue,
							},
						},
					},
				},
				&corev1alpha1.ObjectSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-objset-available",
						Namespace: "default",
					},
					Status: corev1alpha1.ObjectSetStatus{
						Conditions: []metav1.Condition{
							{
								Type:   corev1alpha1.ObjectSetInTransition,
								Status: metav1.ConditionTrue,
							},
							{
								Type:   corev1alpha1.ObjectSetAvailable,
								Status: metav1.ConditionTrue,
							},
						},
					},
				},
			},
			Assertion:   require.NoError,
			PackageName: "test-objset",
			Namespace:   "default",
		},
		"Package found with Not ready objectset ": {
			ActualObjects: []client.Object{
				&corev1alpha1.ObjectSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-objset-archived",
						Namespace: "default",
					},
					Status: corev1alpha1.ObjectSetStatus{
						Conditions: []metav1.Condition{
							{
								Type:   corev1alpha1.ObjectSetInTransition,
								Status: metav1.ConditionTrue,
							},
							{
								Type:   corev1alpha1.ObjectDeploymentProgressing,
								Status: metav1.ConditionTrue,
							},
						},
					},
				},
				&corev1alpha1.ObjectSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-objset-available",
						Namespace: "default",
					},
					Status: corev1alpha1.ObjectSetStatus{
						Conditions: []metav1.Condition{
							{
								Type:   corev1alpha1.ObjectSetInTransition,
								Status: metav1.ConditionTrue,
							},
							{
								Type:   corev1alpha1.ObjectDeploymentProgressing,
								Status: metav1.ConditionTrue,
							},
						},
					},
				},
			},
			Assertion:   require.Error,
			PackageName: "test-objset",
			Namespace:   "default",
		}, "Package found with available objectset in different namespace": {
			ActualObjects: []client.Object{
				&corev1alpha1.ObjectSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-objset-archived",
						Namespace: "default",
					},
					Status: corev1alpha1.ObjectSetStatus{
						Conditions: []metav1.Condition{
							{
								Type:   corev1alpha1.ObjectSetInTransition,
								Status: metav1.ConditionTrue,
							},
							{
								Type:   corev1alpha1.ObjectDeploymentProgressing,
								Status: metav1.ConditionTrue,
							},
						},
					},
				},
				&corev1alpha1.ObjectSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-objset-available",
						Namespace: "pkomax",
					},
					Status: corev1alpha1.ObjectSetStatus{
						Conditions: []metav1.Condition{
							{
								Type:   corev1alpha1.ObjectSetInTransition,
								Status: metav1.ConditionTrue,
							},
							{
								Type:   corev1alpha1.ObjectSetAvailable,
								Status: metav1.ConditionTrue,
							},
						},
					},
				},
			},
			Assertion:   require.Error,
			PackageName: "test-objset",
			Namespace:   "default",
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			scheme, err := NewScheme()
			require.NoError(t, err)

			fakeClient := fake.
				NewClientBuilder().
				WithScheme(scheme).
				WithObjects(tc.ActualObjects...).
				Build()

			c := NewClient(fakeClient)

			// Test fetching object set
			res, err := c.GetObjectset(context.Background(), tc.PackageName, tc.Namespace)
			t.Log(res, err)
			tc.Assertion(t, err)
		})
	}
}

func TestClient_GetClusterObjectset(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		ActualObjects      []client.Object
		Assertion          require.ErrorAssertionFunc
		ClusterPackageName string
	}{
		"cluster package not found": {
			Assertion:          require.Error,
			ClusterPackageName: "dne",
		},
		"Archived cluster Object Set with cluster Package present": {
			ActualObjects: []client.Object{
				&corev1alpha1.ClusterObjectSet{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-objset-archived",
					},
					Status: corev1alpha1.ClusterObjectSetStatus{
						Conditions: []metav1.Condition{
							{
								Type:   corev1alpha1.ObjectSetArchived,
								Status: metav1.ConditionTrue,
							},
						},
					},
				},
			},
			Assertion:          require.Error,
			ClusterPackageName: "test-objset",
		},
		"cluster Package found with available cluster objectset ": {
			ActualObjects: []client.Object{
				&corev1alpha1.ClusterObjectSet{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-objset-archived",
					},
					Status: corev1alpha1.ClusterObjectSetStatus{
						Conditions: []metav1.Condition{
							{
								Type:   corev1alpha1.ObjectSetArchived,
								Status: metav1.ConditionTrue,
							},
						},
					},
				},
				&corev1alpha1.ClusterObjectSet{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-objset-available",
					},
					Status: corev1alpha1.ClusterObjectSetStatus{
						Conditions: []metav1.Condition{
							{
								Type:   corev1alpha1.ObjectSetAvailable,
								Status: metav1.ConditionTrue,
							},
						},
					},
				},
			},
			Assertion:          require.NoError,
			ClusterPackageName: "test-objset",
		},
		"cluster Package found with Not ready cluster objectset ": {
			ActualObjects: []client.Object{
				&corev1alpha1.ClusterObjectSet{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-objset-archived",
					},
					Status: corev1alpha1.ClusterObjectSetStatus{
						Conditions: []metav1.Condition{
							{
								Type:   corev1alpha1.ObjectSetArchived,
								Status: metav1.ConditionTrue,
							},
						},
					},
				},
				&corev1alpha1.ClusterObjectSet{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-objset-available",
					},
					Status: corev1alpha1.ClusterObjectSetStatus{
						Conditions: []metav1.Condition{
							{
								Type:   corev1alpha1.ObjectSetInTransition,
								Status: metav1.ConditionTrue,
							},
						},
					},
				},
			},
			Assertion:          require.Error,
			ClusterPackageName: "test-objset",
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			scheme, err := NewScheme()
			require.NoError(t, err)

			fakeClient := fake.
				NewClientBuilder().
				WithScheme(scheme).
				WithObjects(tc.ActualObjects...).
				Build()

			c := NewClient(fakeClient)

			// Test fetching cluster object set
			res, err := c.GetClusterObjectset(context.Background(), tc.ClusterPackageName)
			t.Log(res, err)
			tc.Assertion(t, err)
		})
	}
}

func TestClient_GetPackage(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		ActualObjects []client.Object
		Assertion     require.ErrorAssertionFunc
		PackageName   string
		Options       []GetPackageOption
	}{
		"package not found": {
			Assertion:   require.Error,
			PackageName: "dne",
		},
		"ClusterPackage found": {
			ActualObjects: []client.Object{
				&corev1alpha1.ClusterPackage{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cluster-package",
					},
				},
			},
			Assertion:   require.NoError,
			PackageName: "cluster-package",
		},
		"Package found": {
			ActualObjects: []client.Object{
				&corev1alpha1.Package{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "package",
						Namespace: "package-namespace",
					},
				},
			},
			Assertion: require.NoError,
			Options: []GetPackageOption{
				WithNamespace("package-namespace"),
			},
			PackageName: "package",
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			scheme, err := NewScheme()
			require.NoError(t, err)

			fakeClient := fake.
				NewClientBuilder().
				WithScheme(scheme).
				WithObjects(tc.ActualObjects...).
				Build()

			c := NewClient(fakeClient)
			_, err = c.GetPackage(context.Background(), tc.PackageName, tc.Options...)
			tc.Assertion(t, err)
		})
	}
}

func TestClient_GetObjectDeployment(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		ActualObjects        []client.Object
		Assertion            require.ErrorAssertionFunc
		ObjectDeploymentName string
		Options              []GetObjectDeploymentOption
	}{
		"objectdeployment not found": {
			Assertion:            require.Error,
			ObjectDeploymentName: "dne",
		},
		"ClusterObjectDeployment found": {
			ActualObjects: []client.Object{
				&corev1alpha1.ClusterObjectDeployment{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cluster-object-deployment",
					},
				},
			},
			Assertion:            require.NoError,
			ObjectDeploymentName: "cluster-object-deployment",
		},
		"ObjectDeployment found": {
			ActualObjects: []client.Object{
				&corev1alpha1.ObjectDeployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "object-deployment",
						Namespace: "object-deployment-namespace",
					},
				},
			},
			Assertion: require.NoError,
			Options: []GetObjectDeploymentOption{
				WithNamespace("object-deployment-namespace"),
			},
			ObjectDeploymentName: "object-deployment",
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			scheme, err := NewScheme()
			require.NoError(t, err)

			fakeClient := fake.
				NewClientBuilder().
				WithScheme(scheme).
				WithObjects(tc.ActualObjects...).
				Build()

			c := NewClient(fakeClient)
			_, err = c.GetObjectDeployment(context.Background(), tc.ObjectDeploymentName, tc.Options...)
			tc.Assertion(t, err)
		})
	}
}

func TestPackage(t *testing.T) {
	t.Parallel()

	type expected struct {
		Name            string
		Namespace       string
		CurrentRevision int64
		NumObjectSets   int
	}

	for name, tc := range map[string]struct {
		PackageObj    client.Object
		ActualObjects []client.Object
		Expected      expected
	}{
		"cluster package without objectsets": {
			PackageObj: &corev1alpha1.ClusterPackage{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-package",
				},
				Status: corev1alpha1.PackageStatus{
					Revision: 0,
				},
			},
			Expected: expected{
				Name:            "cluster-package",
				CurrentRevision: 0,
			},
		},
		"cluster package with objectsets": {
			PackageObj: &corev1alpha1.ClusterPackage{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-package",
				},
				Status: corev1alpha1.PackageStatus{
					Revision: 1,
				},
			},
			ActualObjects: []client.Object{
				&corev1alpha1.ClusterObjectSet{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cluster-object-set",
						Labels: map[string]string{
							manifestsv1alpha1.PackageInstanceLabel: "cluster-package",
						},
					},
					Status: corev1alpha1.ClusterObjectSetStatus{
						Revision: 1,
					},
				},
			},
			Expected: expected{
				Name:            "cluster-package",
				CurrentRevision: 1,
				NumObjectSets:   1,
			},
		},
		"package without objectsets": {
			PackageObj: &corev1alpha1.Package{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "package",
					Namespace: "package-namespace",
				},
				Status: corev1alpha1.PackageStatus{
					Revision: 0,
				},
			},
			Expected: expected{
				Name:            "package",
				Namespace:       "package-namespace",
				CurrentRevision: 0,
			},
		},
		"package with objectsets": {
			PackageObj: &corev1alpha1.Package{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "package",
					Namespace: "package-namespace",
				},
				Status: corev1alpha1.PackageStatus{
					Revision: 1,
				},
			},
			ActualObjects: []client.Object{
				&corev1alpha1.ObjectSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "object-set",
						Namespace: "package-namespace",
						Labels: map[string]string{
							manifestsv1alpha1.PackageInstanceLabel: "package",
						},
					},
					Status: corev1alpha1.ObjectSetStatus{
						Revision: 1,
					},
				},
			},
			Expected: expected{
				Name:            "package",
				Namespace:       "package-namespace",
				CurrentRevision: 1,
				NumObjectSets:   1,
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			scheme, err := NewScheme()
			require.NoError(t, err)

			fakeClient := fake.
				NewClientBuilder().
				WithScheme(scheme).
				WithObjects(tc.ActualObjects...).
				Build()

			pkg := Package{
				client: fakeClient,
				obj:    tc.PackageObj,
			}

			assert.Equal(t, tc.Expected.Name, pkg.Name())
			assert.Equal(t, tc.Expected.Namespace, pkg.Namespace())
			assert.Equal(t, tc.Expected.CurrentRevision, pkg.CurrentRevision())

			sets, err := pkg.ObjectSets(context.Background())
			require.NoError(t, err)

			assert.Len(t, sets, tc.Expected.NumObjectSets)
		})
	}
}

func TestObjectDeployment(t *testing.T) {
	t.Parallel()

	type expected struct {
		Name            string
		Namespace       string
		CurrentRevision int64
		NumObjectSets   int
	}

	for name, tc := range map[string]struct {
		ObjectDeploymentObj client.Object
		ActualObjects       []client.Object
		Expected            expected
	}{
		"cluster object deployment without objectsets": {
			ObjectDeploymentObj: &corev1alpha1.ClusterObjectDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-object-deployment",
				},
				Status: corev1alpha1.ClusterObjectDeploymentStatus{
					Revision: 0,
				},
			},
			Expected: expected{
				Name:            "cluster-object-deployment",
				CurrentRevision: 0,
			},
		},
		"cluster object deployment with objectsets": {
			ObjectDeploymentObj: &corev1alpha1.ClusterObjectDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-object-deployment",
					Labels: map[string]string{
						manifestsv1alpha1.PackageInstanceLabel: "cluster-package",
					},
				},
				Status: corev1alpha1.ClusterObjectDeploymentStatus{
					Revision: 1,
				},
			},
			ActualObjects: []client.Object{
				&corev1alpha1.ClusterObjectSet{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cluster-object-set",
						Labels: map[string]string{
							manifestsv1alpha1.PackageInstanceLabel: "cluster-package",
						},
					},
					Status: corev1alpha1.ClusterObjectSetStatus{
						Revision: 1,
					},
				},
			},
			Expected: expected{
				Name:            "cluster-object-deployment",
				CurrentRevision: 1,
				NumObjectSets:   1,
			},
		},
		"object deployment without objectsets": {
			ObjectDeploymentObj: &corev1alpha1.ObjectDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "object-deployment",
					Namespace: "object-deployment-namespace",
				},
				Status: corev1alpha1.ObjectDeploymentStatus{
					Revision: 0,
				},
			},
			Expected: expected{
				Name:            "object-deployment",
				Namespace:       "object-deployment-namespace",
				CurrentRevision: 0,
			},
		},
		"object deployment with objectsets": {
			ObjectDeploymentObj: &corev1alpha1.ObjectDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "object-deployment",
					Namespace: "object-deployment-namespace",
					Labels: map[string]string{
						manifestsv1alpha1.PackageInstanceLabel: "package",
					},
				},
				Status: corev1alpha1.ObjectDeploymentStatus{
					Revision: 1,
				},
			},
			ActualObjects: []client.Object{
				&corev1alpha1.ObjectSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "object-set",
						Namespace: "object-deployment-namespace",
						Labels: map[string]string{
							manifestsv1alpha1.PackageInstanceLabel: "package",
						},
					},
					Status: corev1alpha1.ObjectSetStatus{
						Revision: 1,
					},
				},
			},
			Expected: expected{
				Name:            "object-deployment",
				Namespace:       "object-deployment-namespace",
				CurrentRevision: 1,
				NumObjectSets:   1,
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			scheme, err := NewScheme()
			require.NoError(t, err)

			fakeClient := fake.
				NewClientBuilder().
				WithScheme(scheme).
				WithObjects(tc.ActualObjects...).
				Build()

			dep := ObjectDeployment{
				client: fakeClient,
				obj:    tc.ObjectDeploymentObj,
			}

			assert.Equal(t, tc.Expected.Name, dep.Name())
			assert.Equal(t, tc.Expected.Namespace, dep.Namespace())
			assert.Equal(t, tc.Expected.CurrentRevision, dep.CurrentRevision())

			sets, err := dep.ObjectSets(context.Background())
			require.NoError(t, err)

			assert.Len(t, sets, tc.Expected.NumObjectSets)
		})
	}
}

func TestObjectSet(t *testing.T) {
	t.Parallel()

	type expected struct {
		Name        string
		Namespace   string
		ChangeCause string
		Revision    int64
		Succeeded   bool
	}

	for name, tc := range map[string]struct {
		ObjectSetObj  client.Object
		ActualObjects []client.Object
		Expected      expected
	}{
		"cluster object set/not succeeded/no change cause": {
			ObjectSetObj: &corev1alpha1.ClusterObjectSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-object-set",
				},
			},
			Expected: expected{
				Name: "cluster-object-set",
			},
		},
		"cluster object set/succeeded/change cause": {
			ObjectSetObj: &corev1alpha1.ClusterObjectSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-object-set",
					Annotations: map[string]string{
						"kubernetes.io/change-cause": "magic",
					},
				},
				Status: corev1alpha1.ClusterObjectSetStatus{
					Revision: 1,
					Conditions: []metav1.Condition{
						{
							Type:   corev1alpha1.ObjectSetSucceeded,
							Status: metav1.ConditionTrue,
						},
					},
				},
			},
			Expected: expected{
				Name:        "cluster-object-set",
				ChangeCause: "magic",
				Revision:    1,
				Succeeded:   true,
			},
		},
		"object set": {
			ObjectSetObj: &corev1alpha1.ObjectSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "object-set",
					Namespace: "object-set-namespace",
				},
			},
			Expected: expected{
				Name:      "object-set",
				Namespace: "object-set-namespace",
			},
		},
		"object set/succeeded/change cause": {
			ObjectSetObj: &corev1alpha1.ObjectSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: "object-set",
					Annotations: map[string]string{
						"kubernetes.io/change-cause": "magic",
					},
				},
				Status: corev1alpha1.ObjectSetStatus{
					Revision: 1,
					Conditions: []metav1.Condition{
						{
							Type:   corev1alpha1.ObjectSetSucceeded,
							Status: metav1.ConditionTrue,
						},
					},
				},
			},
			Expected: expected{
				Name:        "object-set",
				ChangeCause: "magic",
				Revision:    1,
				Succeeded:   true,
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			set := ObjectSet{
				obj: tc.ObjectSetObj,
			}

			assert.Equal(t, tc.Expected.Name, set.Name())
			assert.Equal(t, tc.Expected.Namespace, set.Namespace())
			assert.Equal(t, tc.Expected.ChangeCause, set.ChangeCause())
			assert.Equal(t, tc.Expected.Succeeded, set.HasSucceeded())
		})
	}
}

func TestObjectSetList_Sort(t *testing.T) {
	t.Parallel()

	list := ObjectSetList{
		NewObjectSet(&corev1alpha1.ClusterObjectSet{
			Status: corev1alpha1.ClusterObjectSetStatus{
				Revision: 2,
			},
		}),
		NewObjectSet(&corev1alpha1.ClusterObjectSet{
			Status: corev1alpha1.ClusterObjectSetStatus{
				Revision: 3,
			},
		}),
		NewObjectSet(&corev1alpha1.ClusterObjectSet{
			Status: corev1alpha1.ClusterObjectSetStatus{
				Revision: 1,
			},
		}),
	}

	list.Sort()

	revs := make([]int64, 0, len(list))
	for _, os := range list {
		revs = append(revs, os.Revision())
	}

	assert.Equal(t, []int64{1, 2, 3}, revs)
}

func TestObjectSetList_FindRevision(t *testing.T) {
	t.Parallel()

	list := ObjectSetList{
		NewObjectSet(&corev1alpha1.ClusterObjectSet{
			Status: corev1alpha1.ClusterObjectSetStatus{
				Revision: 2,
			},
		}),
		NewObjectSet(&corev1alpha1.ClusterObjectSet{
			Status: corev1alpha1.ClusterObjectSetStatus{
				Revision: 3,
			},
		}),
		NewObjectSet(&corev1alpha1.ClusterObjectSet{
			Status: corev1alpha1.ClusterObjectSetStatus{
				Revision: 1,
			},
		}),
	}

	rev, found := list.FindRevision(1)
	require.True(t, found)

	assert.Equal(t, int64(1), rev.Revision())
}

func TestObjectSetList_RenderYAML(t *testing.T) {
	t.Parallel()

	expected := strings.Join([]string{
		"- metadata:",
		"    creationTimestamp: null",
		"  spec:",
		"    revision: 1",
		"  status:",
		"    revision: 1",
		"",
	}, "\n")

	list := ObjectSetList{
		NewObjectSet(&corev1alpha1.ClusterObjectSet{
			Spec: corev1alpha1.ClusterObjectSetSpec{
				Revision: 1,
			},
			Status: corev1alpha1.ClusterObjectSetStatus{
				Revision: 1,
			},
		}),
	}

	data, err := list.RenderYAML()
	require.NoError(t, err)

	assert.Equal(t, expected, string(data))
}

func TestObjectSetList_RenderJSON(t *testing.T) {
	t.Parallel()

	expected := strings.Join([]string{
		"[",
		"    {",
		`        "metadata": {`,
		`            "creationTimestamp": null`,
		"        },",
		`        "spec": {`,
		`            "revision": 1`,
		"        },",
		`        "status": {`,
		`            "revision": 1`,
		"        }",
		"    }",
		"]",
	}, "\n")

	list := ObjectSetList{
		NewObjectSet(&corev1alpha1.ClusterObjectSet{
			Spec: corev1alpha1.ClusterObjectSetSpec{
				Revision: 1,
			},
			Status: corev1alpha1.ClusterObjectSetStatus{
				Revision: 1,
			},
		}),
	}

	data, err := list.RenderJSON()
	require.NoError(t, err)

	assert.Equal(t, expected, string(data))
}

func TestObjectSetList_RenderTable(t *testing.T) {
	t.Parallel()

	expected := NewDefaultTable(
		WithHeaders{"Revision", "Successful", "Change-Cause"},
	)

	expected.AddRow(
		Field{
			Name:  "Revision",
			Value: int64(1),
		},
		Field{
			Name:  "Successful",
			Value: false,
		},
		Field{
			Name:  "Change-Cause",
			Value: "",
		},
	)

	list := ObjectSetList{
		NewObjectSet(&corev1alpha1.ClusterObjectSet{
			Status: corev1alpha1.ClusterObjectSetStatus{
				Revision: 1,
			},
		}),
	}

	table := list.RenderTable("Revision", "Successful", "Change-Cause")

	assert.Equal(t, expected, table)
}
