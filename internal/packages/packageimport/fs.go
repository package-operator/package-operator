package packageimport

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-logr/logr"

	"package-operator.run/package-operator/internal/packages/packagecontent"
)

func FS(ctx context.Context, src fs.FS) (packagecontent.Files, error) {
	verboseLog := logr.FromContextOrDiscard(ctx).V(1)
	bundle := packagecontent.Files{}
	walker := func(path string, entry fs.DirEntry, ioErr error) error {
		switch {
		case ioErr != nil:
			return fmt.Errorf("access file %s: %w", path, ioErr)

		case entry.Name() == ".":
			// continue at root

		case isFileToBeExcluded(entry):
			verboseLog.Info("skipping file in source", "path", path)
			if entry.IsDir() {
				return filepath.SkipDir
			}

		case entry.IsDir():
			// no special handling for directories

		default:
			verboseLog.Info("adding source file", "path", path)
			data, err := fs.ReadFile(src, path)
			if err != nil {
				return fmt.Errorf("read file %s: %w", path, err)
			}
			bundle[path] = data
		}

		return nil
	}

	if err := fs.WalkDir(src, ".", walker); err != nil {
		return nil, fmt.Errorf("walk source dir: %w", err)
	}

	return bundle, nil
}

func Folder(ctx context.Context, path string) (packagecontent.Files, error) {
	return FS(ctx, os.DirFS(path))
}

func isFileToBeExcluded(entry fs.DirEntry) bool {
	return isFilenameToBeExcluded(entry.Name())
}

func isFilenameToBeExcluded(fileName string) bool {
	return strings.HasPrefix(fileName, ".")
}
