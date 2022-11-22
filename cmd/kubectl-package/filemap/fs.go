package filemap

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/go-logr/logr"
)

func isFileToBeExcluded(entry fs.DirEntry) bool {
	return strings.HasPrefix(entry.Name(), ".")
}

func FromFS(ctx context.Context, source fs.FS) (FileMap, error) {
	verboseLog := logr.FromContextOrDiscard(ctx).V(1)
	bundle := FileMap{}
	walker := func(path string, entry fs.DirEntry, ioErr error) error {
		switch {
		case ioErr != nil:
			return fmt.Errorf("access file %s: %w", path, ioErr)
		case entry.Name() == ".":
		case isFileToBeExcluded(entry):
			verboseLog.Info("skipping file in source", "path", path)
			if entry.IsDir() {
				return filepath.SkipDir
			}
		case entry.IsDir():
		default:
			verboseLog.Info("adding source file", "path", path)
			data, err := fs.ReadFile(source, path)
			if err != nil {
				return fmt.Errorf("read file %s: %w", path, err)
			}
			bundle[path] = data
		}

		return nil
	}

	if err := fs.WalkDir(source, ".", walker); err != nil {
		return nil, fmt.Errorf("walk source dir: %w", err)
	}

	return bundle, nil
}
