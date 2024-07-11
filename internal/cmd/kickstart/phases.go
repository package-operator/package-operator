package kickstart

import (
	"fmt"

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

// Known Objects that do not need probes defined.
var noProbeGK = map[schema.GroupKind]struct{}{
	{Kind: "Namespace"}:             {},
	{Kind: "ServiceAccount"}:        {},
	{Kind: "Endpoints"}:             {},
	{Kind: "EndpointSlice"}:         {},
	{Kind: "IngressClass"}:          {},
	{Kind: "Service"}:               {},
	{Kind: "Secret"}:                {},
	{Kind: "ConfigMap"}:             {},
	{Kind: "PersistentVolume"}:      {},
	{Kind: "PersistentVolumeClaim"}: {},
	{Kind: "ResourceQuota"}:         {},
	{Kind: "LimitRange"}:            {},

	{Kind: "Role", Group: "rbac.authorization.k8s.io"}:               {},
	{Kind: "RoleRolebinding", Group: "rbac.authorization.k8s.io"}:    {},
	{Kind: "ClusterRole", Group: "rbac.authorization.k8s.io"}:        {},
	{Kind: "ClusterRoleBinding", Group: "rbac.authorization.k8s.io"}: {},

	{Kind: "PriorityClass", Group: "scheduling.k8s.io"}:     {},
	{Kind: "Ingress", Group: "networking.k8s.io"}:           {},
	{Kind: "NetworkPolicy", Group: "networking.k8s.io"}:     {},
	{Kind: "HorizontalPodAutoscaler", Group: "autoscaling"}: {},
	{Kind: "PodDisruptionBudget", Group: "policy"}:          {},
	{Kind: "CronJob", Group: "batch"}:                       {},
	{Kind: "APIService", Group: "apiregistration.k8s.io"}:   {},

	{Kind: "StorageClass", Group: "storage.k8s.io"}:       {},
	{Kind: "CSIDriver", Group: "storage.k8s.io"}:          {},
	{Kind: "CSINode", Group: "storage.k8s.io"}:            {},
	{Kind: "CSIStorageCapacity", Group: "storage.k8s.io"}: {},

	{Kind: "MutatingWebhookConfiguration", Group: "admissionregistration.k8s.io"}:   {},
	{Kind: "ValidatingWebhookConfiguration", Group: "admissionregistration.k8s.io"}: {},
	{Kind: "ValidatingAdmissionPolicy", Group: "admissionregistration.k8s.io"}:      {},
}

// Determines a phase using the objects Group Kind from a list or presets.
func guessPresetPhase(gk schema.GroupKind) string {
	phase, ok := gkPhaseMap[gk]
	if !ok {
		return string(presetPhaseOther)
	}
	return string(phase)
}

func reportGKsWithoutProbes(gksWithoutProbes map[schema.GroupKind]struct{}) (report string, ok bool) {
	report = "[WARN] Some kinds don't have availability probes defined:\n"
	for gk := range gksWithoutProbes {
		if _, ok := noProbeGK[gk]; ok {
			continue
		}
		report += fmt.Sprintf("- %s\n", gk.String())
		ok = true
	}
	return report, ok
}
