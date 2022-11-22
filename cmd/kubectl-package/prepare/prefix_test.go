package prepare_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"package-operator.run/package-operator/cmd/kubectl-package/filemap"
	"package-operator.run/package-operator/cmd/kubectl-package/prepare"
)

func TestPrefix(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	input := filemap.FileMap{"chicken": {}, "nested/chicken": {}}
	expectedOutput := filemap.FileMap{"package/chicken": {}, "package/nested/chicken": {}}
	output := prepare.PrefixPaths(ctx, input)
	assert.Equal(t, expectedOutput, output)
}
