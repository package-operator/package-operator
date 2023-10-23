package components

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
)

//nolint:paralleltest
func TestProvideOptions(t *testing.T) {
	t.Setenv("PKO_SUB_COMPONENT_TOLERATIONS",
		`[{"effect":"NoSchedule","key":"node-role.kubernetes.io/infra"},`+
			`{"effect":"NoSchedule","key":"hypershift.openshift.io/hosted-control-plane"}]`,
	)
	t.Setenv("PKO_SUB_COMPONENT_AFFINITY",
		`{ "nodeAffinity": { "requiredDuringSchedulingIgnoredDuringExecution": { "nodeSelectorTerms": [ `+
			`{ "matchExpressions": [ { "key": "node-role.kubernetes.io/infra", "operator": "Exists" } ] }, `+
			`{ "matchExpressions": [ { "key": "hypershift.openshift.io/hosted-control-plane", "operator": "Exists" }`+
			` ] } ] } } }`,
	)
	opts, err := ProvideOptions()
	require.NoError(t, err)

	assert.Equal(t, Options{
		EnableLeaderElection: true,
		MetricsAddr:          ":8080",
		ProbeAddr:            ":8081",
		SubComponentTolerations: []corev1.Toleration{
			{
				Key:    "node-role.kubernetes.io/infra",
				Effect: corev1.TaintEffectNoSchedule,
			},
			{
				Key:    "hypershift.openshift.io/hosted-control-plane",
				Effect: corev1.TaintEffectNoSchedule,
			},
		},
		SubComponentAffinity: &corev1.Affinity{
			NodeAffinity: &corev1.NodeAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
					NodeSelectorTerms: []corev1.NodeSelectorTerm{
						{
							MatchExpressions: []corev1.NodeSelectorRequirement{
								{
									Key:      "node-role.kubernetes.io/infra",
									Operator: corev1.NodeSelectorOpExists,
								},
							},
						},
						{
							MatchExpressions: []corev1.NodeSelectorRequirement{
								{
									Key:      "hypershift.openshift.io/hosted-control-plane",
									Operator: corev1.NodeSelectorOpExists,
								},
							},
						},
					},
				},
			},
		},
	}, opts)
}
