package packagekickstart

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

// Determines probes required for the given Group Kind.
// Returns a probe if one was found and an ok-style bool to let the caller know if a probe was found.
func getProbe(gk schema.GroupKind) (corev1alpha1.ObjectSetProbe, bool) {
	probes, ok := gkProbes[gk]
	if !ok {
		return corev1alpha1.ObjectSetProbe{}, false
	}
	return corev1alpha1.ObjectSetProbe{
		Selector: corev1alpha1.ProbeSelector{
			Kind: &corev1alpha1.PackageProbeKindSpec{
				Group: gk.Group,
				Kind:  gk.Kind,
			},
		},
		Probes: probes,
	}, true
}

var gkProbes = map[schema.GroupKind][]corev1alpha1.Probe{
	{
		Kind: "Deployment", Group: "apps",
	}: {
		availableProbe,
		replicasUpdatedProbe,
	},
	{
		Kind: "StatefulSet", Group: "apps",
	}: {
		availableProbe,
		replicasUpdatedProbe,
	},
	{
		Kind: "DaemonSet", Group: "apps",
	}: {
		{
			FieldsEqual: &corev1alpha1.ProbeFieldsEqualSpec{
				FieldA: ".status.desiredNumberScheduled",
				FieldB: ".status.numberAvailable",
			},
		},
	},
	{
		Kind: "ReplicaSet", Group: "apps",
	}: {
		availableProbe,
		replicasUpdatedProbe,
	},
	{
		Kind:  "CustomResourceDefinition",
		Group: "apiextensions.k8s.io",
	}: {
		{
			Condition: &corev1alpha1.ProbeConditionSpec{
				Type:   "Established",
				Status: string(metav1.ConditionTrue),
			},
		},
	},
	{
		Kind:  "Job",
		Group: "batch",
	}: {
		{
			Condition: &corev1alpha1.ProbeConditionSpec{
				Type:   "Complete",
				Status: string(metav1.ConditionTrue),
			},
		},
	},
	{
		Kind:  "Route",
		Group: "route.openshift.io",
	}: {
		{
			CEL: &corev1alpha1.ProbeCELSpec{
				Message: "not all ingress points are reporting ready",
				Rule:    `self.status.ingress.all(i, i.conditions.all(c, c.type == "Ready" && c.status == "True"))`,
			},
		},
	},
	{
		Kind:  "PersistentVolumeClaim",
		Group: "",
	}: {
		{
			CEL: &corev1alpha1.ProbeCELSpec{
				Message: "is not yet Bound",
				Rule:    `self.status.phase == "Bound"`,
			},
		},
	},
	{
		Kind:  "ClusterServiceVersion",
		Group: "operators.coreos.com",
	}: {
		{
			CEL: &corev1alpha1.ProbeCELSpec{
				Message: "CSV not succeeded",
				Rule:    `self.status.phase == "Succeeded"`,
			},
		},
	},
}

// Checks if the Available Condition is True.
var availableProbe = corev1alpha1.Probe{
	Condition: &corev1alpha1.ProbeConditionSpec{
		Type:   "Available",
		Status: string(metav1.ConditionTrue),
	},
}

// Checks if all replicas have been updated.
// Works for StatefulSets, Deployments and ReplicaSets.
var replicasUpdatedProbe = corev1alpha1.Probe{
	FieldsEqual: &corev1alpha1.ProbeFieldsEqualSpec{
		FieldA: ".status.updatedReplicas",
		FieldB: ".status.replicas",
	},
}

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

func init() {
	for phase, gks := range phaseGKMap {
		for _, gk := range gks {
			gkPhaseMap[gk] = phase
		}
	}
}
