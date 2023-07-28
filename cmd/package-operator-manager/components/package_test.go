package components

import (
	"testing"

	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/assert"
)

func Test_prepareRegistryHostOverrides(t *testing.T) {
	t.Parallel()
	log := testr.New(t)
	or := prepareRegistryHostOverrides(log, "quay.io=dev-registry.dev-registry.svc.cluster.local:5001")
	assert.Equal(t, map[string]string{"quay.io": "dev-registry.dev-registry.svc.cluster.local:5001"}, or)
}
