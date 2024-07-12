package packagekickstart

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type presetPhase string

const (
	presetPhaseNamespaces presetPhase = "namespaces"
	presetPhasePolicies   presetPhase = "policies"
	presetPhaseRBAC       presetPhase = "rbac"
	presetPhaseCRDs       presetPhase = "crds"
	presetPhaseStorage    presetPhase = "storage"
	presetPhaseDeploy     presetPhase = "deploy"
	presetPhasePublish    presetPhase = "publish"
	// anything else that is not explicitly sorted into a phase.
	presetPhaseOther presetPhase = "other"
)

var orderedPhases = []presetPhase{
	presetPhaseNamespaces,
	presetPhasePolicies,
	presetPhaseRBAC,
	presetPhaseCRDs,
	presetPhaseStorage,
	presetPhaseDeploy,
	presetPhasePublish,
	presetPhaseOther,
}

var (
	// This will be populated from `phaseGKMap` in an init func!
	gkPhaseMap = map[schema.GroupKind]presetPhase{}
	phaseGKMap = map[presetPhase][]schema.GroupKind{
		presetPhaseNamespaces: {
			{Kind: "Namespace"},
		},

		presetPhasePolicies: {
			{Kind: "ResourceQuota"},
			{Kind: "LimitRange"},
			{Kind: "PriorityClass", Group: "scheduling.k8s.io"},
			{Kind: "NetworkPolicy", Group: "networking.k8s.io"},
			{Kind: "HorizontalPodAutoscaler", Group: "autoscaling"},
			{Kind: "PodDisruptionBudget", Group: "policy"},
		},

		presetPhaseRBAC: {
			{Kind: "ServiceAccount"},
			{Kind: "Role", Group: "rbac.authorization.k8s.io"},
			{Kind: "RoleRolebinding", Group: "rbac.authorization.k8s.io"},
			{Kind: "ClusterRole", Group: "rbac.authorization.k8s.io"},
			{Kind: "ClusterRoleBinding", Group: "rbac.authorization.k8s.io"},
		},

		presetPhaseCRDs: {
			{Kind: "CustomResourceDefinition", Group: "apiextensions.k8s.io"},
		},

		presetPhaseStorage: {
			{Kind: "PersistentVolume"},
			{Kind: "PersistentVolumeClaim"},
			{Kind: "StorageClass", Group: "storage.k8s.io"},
		},

		presetPhaseDeploy: {
			{Kind: "Deployment", Group: "apps"},
			{Kind: "DaemonSet", Group: "apps"},
			{Kind: "StatefulSet", Group: "apps"},
			{Kind: "ReplicaSet"},
			{Kind: "Pod"}, // probing complicated, may be either Completed or Available.
			{Kind: "Job", Group: "batch"},
			{Kind: "CronJob", Group: "batch"},
			{Kind: "Service"},
			{Kind: "Secret"},
			{Kind: "ConfigMap"},
		},

		presetPhasePublish: {
			{Kind: "Ingress", Group: "networking.k8s.io"},
			{Kind: "APIService", Group: "apiregistration.k8s.io"},
			{Kind: "Route", Group: "route.openshift.io"},
			{Kind: "MutatingWebhookConfiguration", Group: "admissionregistration.k8s.io"},
			{Kind: "ValidatingWebhookConfiguration", Group: "admissionregistration.k8s.io"},
		},
	}
)

// Determines a phase using the objects Group Kind from a list of presets.
// Defaults to the value of `presetPhaseOther` if no preset was found.
func guessPresetPhase(gk schema.GroupKind) string {
	phase, ok := gkPhaseMap[gk]
	if !ok {
		return string(presetPhaseOther)
	}
	return string(phase)
}
