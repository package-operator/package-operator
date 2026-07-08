package controllers

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/internal/adapters"
	"package-operator.run/internal/testutil"
)

func TestClassifierFromObjectSets(t *testing.T) {
	t.Parallel()
	scheme := testutil.NewTestSchemeWithCoreV1Alpha1()

	tests := []struct {
		name        string
		objectSets  []adapters.ObjectSetAccessor
		selfUID     types.UID
		matchUIDs   []types.UID
		noMatchUIDs []types.UID
	}{
		{
			name:        "empty list",
			objectSets:  nil,
			selfUID:     "self-uid",
			noMatchUIDs: []types.UID{"any-uid"},
		},
		{
			name: "matches ObjectSet UIDs",
			objectSets: func() []adapters.ObjectSetAccessor {
				os := adapters.NewObjectSet(scheme)
				os.ClientObject().SetUID("os-uid-1")
				return []adapters.ObjectSetAccessor{os}
			}(),
			selfUID:     "self-uid",
			matchUIDs:   []types.UID{"os-uid-1"},
			noMatchUIDs: []types.UID{"unknown-uid"},
		},
		{
			name: "matches remote phase UIDs",
			objectSets: func() []adapters.ObjectSetAccessor {
				os := adapters.NewObjectSet(scheme)
				os.ClientObject().SetUID("os-uid-1")
				os.SetStatusRemotePhases([]corev1alpha1.RemotePhaseReference{
					{Name: "phase-1", UID: "remote-phase-uid"},
				})
				return []adapters.ObjectSetAccessor{os}
			}(),
			selfUID:     "self-uid",
			matchUIDs:   []types.UID{"os-uid-1", "remote-phase-uid"},
			noMatchUIDs: []types.UID{"unknown-uid"},
		},
		{
			name: "includes remote phases of non-self ObjectSets",
			objectSets: func() []adapters.ObjectSetAccessor {
				os := adapters.NewObjectSet(scheme)
				os.ClientObject().SetUID("os-uid-1")
				os.SetStatusRemotePhases([]corev1alpha1.RemotePhaseReference{
					{Name: "phase-a", UID: "remote-a"},
					{Name: "phase-b", UID: "remote-b"},
				})
				return []adapters.ObjectSetAccessor{os}
			}(),
			selfUID:     "other-uid",
			matchUIDs:   []types.UID{"os-uid-1", "remote-a", "remote-b"},
			noMatchUIDs: []types.UID{"other-uid"},
		},
		{
			name: "excludes UIDs from ObjectSet UIDs",
			objectSets: func() []adapters.ObjectSetAccessor {
				self := adapters.NewObjectSet(scheme)
				self.ClientObject().SetUID("self-uid")
				sibling := adapters.NewObjectSet(scheme)
				sibling.ClientObject().SetUID("sibling-uid")
				return []adapters.ObjectSetAccessor{self, sibling}
			}(),
			selfUID:     "self-uid",
			matchUIDs:   []types.UID{"sibling-uid"},
			noMatchUIDs: []types.UID{"self-uid"},
		},
		{
			name: "excludes multiple UIDs",
			objectSets: func() []adapters.ObjectSetAccessor {
				os := adapters.NewObjectSet(scheme)
				os.ClientObject().SetUID("os-uid")
				os.SetStatusRemotePhases([]corev1alpha1.RemotePhaseReference{
					{Name: "rp-a", UID: "rp-a-uid"},
					{Name: "rp-b", UID: "rp-b-uid"},
				})
				sibling := adapters.NewObjectSet(scheme)
				sibling.ClientObject().SetUID("sibling-uid")
				return []adapters.ObjectSetAccessor{os, sibling}
			}(),
			selfUID:     "os-uid",
			matchUIDs:   []types.UID{"sibling-uid"},
			noMatchUIDs: []types.UID{"os-uid", "rp-a-uid", "rp-b-uid"},
		},
		{
			name: "multiple ObjectSets with remote phases",
			objectSets: func() []adapters.ObjectSetAccessor {
				os1 := adapters.NewObjectSet(scheme)
				os1.ClientObject().SetUID("os-uid-1")
				os1.SetStatusRemotePhases([]corev1alpha1.RemotePhaseReference{
					{Name: "phase-a", UID: "remote-a"},
				})
				os2 := adapters.NewObjectSet(scheme)
				os2.ClientObject().SetUID("os-uid-2")
				os2.SetStatusRemotePhases([]corev1alpha1.RemotePhaseReference{
					{Name: "phase-b", UID: "remote-b"},
				})
				return []adapters.ObjectSetAccessor{os1, os2}
			}(),
			selfUID:     "self-uid",
			matchUIDs:   []types.UID{"os-uid-1", "os-uid-2", "remote-a", "remote-b"},
			noMatchUIDs: []types.UID{"unknown-uid"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			classifier := classifierFromObjectSets(tc.objectSets, tc.selfUID)
			for _, uid := range tc.matchUIDs {
				assert.True(t, classifier(metav1.OwnerReference{UID: uid}),
					"expected UID %s to be classified as sibling", uid)
			}
			for _, uid := range tc.noMatchUIDs {
				assert.False(t, classifier(metav1.OwnerReference{UID: uid}),
					"expected UID %s to NOT be classified as sibling", uid)
			}
		})
	}
}

func TestSiblingOwnerLookup_ClassifierForObjectSetPhase_NoControllerRef(t *testing.T) {
	t.Parallel()
	scheme := testutil.NewTestSchemeWithCoreV1Alpha1()
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	lookup := NewSiblingOwnerLookup(scheme, fakeClient, adapters.NewObjectSet, adapters.NewObjectSetList)
	osp := adapters.NewObjectSetPhaseAccessor(scheme)

	classifier, err := lookup.ClassifierForObjectSetPhase(context.Background(), osp)
	require.NoError(t, err)
	assert.False(t, classifier(metav1.OwnerReference{UID: "any-uid"}))
}

func TestSiblingOwnerLookup_ClassifierForObjectSetPhase_WithDeploymentLabel(t *testing.T) {
	t.Parallel()
	scheme := testutil.NewTestSchemeWithCoreV1Alpha1()

	parentOS := &corev1alpha1.ObjectSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "parent-os",
			Namespace: "test-ns",
			UID:       "parent-uid",
			Labels:    map[string]string{DeploymentLabel: "my-deployment"},
		},
		Status: corev1alpha1.ObjectSetStatus{
			RemotePhases: []corev1alpha1.RemotePhaseReference{
				{Name: "my-osp", UID: "my-osp-uid"},
				{Name: "other-osp", UID: "other-osp-uid"},
			},
		},
	}
	siblingOS := &corev1alpha1.ObjectSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sibling-os",
			Namespace: "test-ns",
			UID:       "sibling-uid",
			Labels:    map[string]string{DeploymentLabel: "my-deployment"},
		},
		Status: corev1alpha1.ObjectSetStatus{
			RemotePhases: []corev1alpha1.RemotePhaseReference{
				{Name: "sibling-osp", UID: "sibling-osp-uid"},
			},
		},
	}
	unrelatedOS := &corev1alpha1.ObjectSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "unrelated-os",
			Namespace: "test-ns",
			UID:       "unrelated-uid",
			Labels:    map[string]string{DeploymentLabel: "other-deployment"},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(parentOS, siblingOS, unrelatedOS).
		Build()

	lookup := NewSiblingOwnerLookup(scheme, fakeClient, adapters.NewObjectSet, adapters.NewObjectSetList)

	osp := adapters.NewObjectSetPhaseAccessor(scheme)
	osp.ClientObject().SetNamespace("test-ns")
	osp.ClientObject().SetUID("my-osp-uid")
	osp.ClientObject().SetOwnerReferences([]metav1.OwnerReference{
		{
			APIVersion: corev1alpha1.GroupVersion.String(),
			Kind:       "ObjectSet",
			Name:       "parent-os",
			UID:        "parent-uid",
			Controller: new(true),
		},
	})

	classifier, err := lookup.ClassifierForObjectSetPhase(context.Background(), osp)
	require.NoError(t, err)

	// Parent ObjectSet and its remote phases are same-revision, not siblings.
	assert.False(t, classifier(metav1.OwnerReference{UID: "parent-uid"}))
	assert.False(t, classifier(metav1.OwnerReference{UID: "my-osp-uid"}))
	assert.False(t, classifier(metav1.OwnerReference{UID: "other-osp-uid"}))
	// Sibling ObjectSet and its remote phases are siblings.
	assert.True(t, classifier(metav1.OwnerReference{UID: "sibling-uid"}))
	assert.True(t, classifier(metav1.OwnerReference{UID: "sibling-osp-uid"}))
	// Unrelated deployment is not a sibling.
	assert.False(t, classifier(metav1.OwnerReference{UID: "unrelated-uid"}))
	assert.False(t, classifier(metav1.OwnerReference{UID: "unknown-uid"}))
}

func TestSiblingOwnerLookup_ClassifierForObjectSetPhase_NoDeploymentLabel(t *testing.T) {
	t.Parallel()
	scheme := testutil.NewTestSchemeWithCoreV1Alpha1()

	parentOS := &corev1alpha1.ObjectSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: "parent-os", Namespace: "test-ns", UID: "parent-uid",
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(parentOS).
		Build()

	lookup := NewSiblingOwnerLookup(scheme, fakeClient, adapters.NewObjectSet, adapters.NewObjectSetList)

	osp := adapters.NewObjectSetPhaseAccessor(scheme)
	osp.ClientObject().SetNamespace("test-ns")
	osp.ClientObject().SetUID("my-osp-uid")
	osp.ClientObject().SetOwnerReferences([]metav1.OwnerReference{
		{
			APIVersion: corev1alpha1.GroupVersion.String(),
			Kind:       "ObjectSet",
			Name:       "parent-os",
			UID:        "parent-uid",
			Controller: new(true),
		},
	})

	classifier, err := lookup.ClassifierForObjectSetPhase(context.Background(), osp)
	require.NoError(t, err)
	assert.False(t, classifier(metav1.OwnerReference{UID: "parent-uid"}))
	assert.False(t, classifier(metav1.OwnerReference{UID: "any-uid"}))
}

func TestSiblingOwnerLookup_ClassifierForObjectSet(t *testing.T) {
	t.Parallel()
	scheme := testutil.NewTestSchemeWithCoreV1Alpha1()

	selfOS := &corev1alpha1.ObjectSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "self-os",
			Namespace: "test-ns",
			UID:       "self-uid",
			Labels:    map[string]string{DeploymentLabel: "my-deployment"},
		},
		Status: corev1alpha1.ObjectSetStatus{
			RemotePhases: []corev1alpha1.RemotePhaseReference{
				{Name: "self-rp", UID: "self-rp-uid"},
			},
		},
	}
	siblingOS := &corev1alpha1.ObjectSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sibling-os",
			Namespace: "test-ns",
			UID:       "sibling-uid",
			Labels:    map[string]string{DeploymentLabel: "my-deployment"},
		},
	}
	unrelatedOS := &corev1alpha1.ObjectSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "unrelated-os",
			Namespace: "test-ns",
			UID:       "unrelated-uid",
			Labels:    map[string]string{DeploymentLabel: "other-deployment"},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(selfOS, siblingOS, unrelatedOS).
		Build()

	lookup := NewSiblingOwnerLookup(scheme, fakeClient, adapters.NewObjectSet, adapters.NewObjectSetList)

	selfAccessor := adapters.NewObjectSet(scheme)
	selfAccessor.ClientObject().SetName("self-os")
	selfAccessor.ClientObject().SetNamespace("test-ns")
	selfAccessor.ClientObject().SetUID("self-uid")
	selfAccessor.ClientObject().SetLabels(map[string]string{DeploymentLabel: "my-deployment"})
	selfAccessor.SetStatusRemotePhases([]corev1alpha1.RemotePhaseReference{
		{Name: "self-rp", UID: "self-rp-uid"},
	})

	classifier, err := lookup.ClassifierForObjectSet(context.Background(), selfAccessor)
	require.NoError(t, err)

	assert.False(t, classifier(metav1.OwnerReference{UID: "self-uid"}))
	assert.False(t, classifier(metav1.OwnerReference{UID: "self-rp-uid"}))
	assert.True(t, classifier(metav1.OwnerReference{UID: "sibling-uid"}))
	assert.False(t, classifier(metav1.OwnerReference{UID: "unrelated-uid"}))
	assert.False(t, classifier(metav1.OwnerReference{UID: "unknown-uid"}))
}

func TestSiblingOwnerLookup_ClassifierForObjectSet_NoLabel(t *testing.T) {
	t.Parallel()
	scheme := testutil.NewTestSchemeWithCoreV1Alpha1()

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	lookup := NewSiblingOwnerLookup(scheme, fakeClient, adapters.NewObjectSet, adapters.NewObjectSetList)

	selfAccessor := adapters.NewObjectSet(scheme)
	selfAccessor.ClientObject().SetName("self-os")
	selfAccessor.ClientObject().SetNamespace("test-ns")
	selfAccessor.ClientObject().SetUID("self-uid")

	classifier, err := lookup.ClassifierForObjectSet(context.Background(), selfAccessor)
	require.NoError(t, err)
	assert.False(t, classifier(metav1.OwnerReference{UID: "any-uid"}))
}
