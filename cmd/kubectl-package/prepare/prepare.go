package prepare

import (
	"context"
	"path/filepath"

	"github.com/go-logr/logr"

	"package-operator.run/package-operator/cmd/kubectl-package/filemap"
)

func FileMap(ctx context.Context, fileMap filemap.FileMap) (filemap.FileMap, error) {
	return PrefixPaths(ctx, fileMap), nil
}

func PrefixPaths(ctx context.Context, input filemap.FileMap) (output filemap.FileMap) {
	verboseLog := logr.FromContextOrDiscard(ctx).V(1)
	verboseLog.Info("rewriting paths")

	output = filemap.FileMap{}

	for path, data := range input {
		output[filepath.Join("package", path)] = data
	}

	return
}
