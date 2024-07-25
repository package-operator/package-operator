package presets

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// Determines a phase using the objects Group Kind from a list of presets.
// Defaults to the value of `presetPhaseOther` if no preset was found.
func DeterminePhase(gk schema.GroupKind) string {
	phase, ok := gkPhaseMap[gk]
	if !ok {
		return string(PhaseOther)
	}
	return string(phase)
}

// Phase represents a well-known phase.
type Phase string

const (
	PhaseNamespaces Phase = "namespaces"
	PhasePolicies   Phase = "policies"
	PhaseRBAC       Phase = "rbac"
	PhaseCRDs       Phase = "crds"
	PhaseStorage    Phase = "storage"
	PhaseDeploy     Phase = "deploy"
	PhasePublish    Phase = "publish"
	// Anything else that is not explicitly sorted into a phase.
	PhaseOther Phase = "other"
)

// Well known phases ordered.
var OrderedPhases = []Phase{
	PhaseNamespaces,
	PhasePolicies,
	PhaseRBAC,
	PhaseCRDs,
	PhaseStorage,
	PhaseDeploy,
	PhasePublish,
	PhaseOther,
}

var (
	// This will be populated from `phaseGKMap` in an init func!
	gkPhaseMap = map[schema.GroupKind]Phase{}
	phaseGKMap = map[Phase][]schema.GroupKind{
		PhaseNamespaces: {
			{Kind: "Namespace"},
		},

		PhasePolicies: {
			{Kind: "ResourceQuota"},
			{Kind: "LimitRange"},
			{Kind: "PriorityClass", Group: "scheduling.k8s.io"},
			{Kind: "NetworkPolicy", Group: "networking.k8s.io"},
			{Kind: "HorizontalPodAutoscaler", Group: "autoscaling"},
			{Kind: "PodDisruptionBudget", Group: "policy"},
		},

		PhaseRBAC: {
			{Kind: "ServiceAccount"},
			{Kind: "Role", Group: "rbac.authorization.k8s.io"},
			{Kind: "RoleRolebinding", Group: "rbac.authorization.k8s.io"},
			{Kind: "ClusterRole", Group: "rbac.authorization.k8s.io"},
			{Kind: "ClusterRoleBinding", Group: "rbac.authorization.k8s.io"},
		},

		PhaseCRDs: {
			{Kind: "CustomResourceDefinition", Group: "apiextensions.k8s.io"},
		},

		PhaseStorage: {
			{Kind: "PersistentVolume"},
			{Kind: "PersistentVolumeClaim"},
			{Kind: "StorageClass", Group: "storage.k8s.io"},
		},

		PhaseDeploy: {
			deployGVK.GroupKind(),
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

		PhasePublish: {
			{Kind: "Ingress", Group: "networking.k8s.io"},
			{Kind: "APIService", Group: "apiregistration.k8s.io"},
			{Kind: "Route", Group: "route.openshift.io"},
			{Kind: "MutatingWebhookConfiguration", Group: "admissionregistration.k8s.io"},
			{Kind: "ValidatingWebhookConfiguration", Group: "admissionregistration.k8s.io"},
		},
	}
)

func init() {
	for phase, gks := range phaseGKMap {
		for _, gk := range gks {
			gkPhaseMap[gk] = phase
		}
	}
}
