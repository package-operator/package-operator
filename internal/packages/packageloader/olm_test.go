package packageloader

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"package-operator.run/package-operator/internal/packages/packagecontent"
)

func TestIsOLMBundle(t *testing.T) {
	files := packagecontent.Files{
		"manifests/xxx": nil,
		"metadata/xxx":  nil,
	}
	ctx := context.Background()
	isOLM := IsOLMBundle(ctx, files)

	assert.True(t, isOLM)
}

func TestIsOLMBundleNegative(t *testing.T) {
	files := packagecontent.Files{
		"manifests/xxx": nil,
		"metadata/xxx":  nil,
		"manifest.yaml": nil,
	}
	ctx := context.Background()
	isOLM := IsOLMBundle(ctx, files)

	assert.False(t, isOLM)
}
