package components

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
)

//nolint:paralleltest
//nolint:nolintlint // directive `//nolint:paralleltest` is unused for linter "paralleltest" (nolintlint)
func TestProvideOptions(t *testing.T) {
	// t.Parallel()
	// panic: testing: t.Setenv called after t.Parallel; cannot set environment variables in parallel tests

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
		ObjectTemplateOptionalResourceRetryInterval: time.Second * 60,
		ObjectTemplateResourceRetryInterval:         time.Second * 30,
		EnableSecurityEnhancedPackages:              false,
	}, opts)
}

//nolint:paralleltest
//nolint:nolintlint // directive `//nolint:paralleltest` is unused for linter "paralleltest" (nolintlint)
func TestEnvToInt(t *testing.T) {
	t.Run("environment varialble not set", func(t *testing.T) {
		// t.Parallel()
		val, err := envToInt("")

		assert.Equal(t, 0, val)
		assert.NoError(t, err)
	})

	t.Run("environment variable is set", func(t *testing.T) {
		t.Setenv("FOO", "1")
		val, err := envToInt("FOO")

		assert.Equal(t, 1, val)
		assert.NoError(t, err)
	})

	t.Run("environment variable set with random value", func(t *testing.T) {
		t.Setenv("FOO_FOO", "some random -- val")
		val, err := envToInt("FOO_FOO")

		assert.Equal(t, 0, val)
		assert.EqualError(t, err, "unable to parse environment variable 'FOO_FOO' as integer:"+
			" strconv.Atoi: parsing \"some random -- val\": invalid syntax")
	})
}
