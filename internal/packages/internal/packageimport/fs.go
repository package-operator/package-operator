package packageimport

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-logr/logr"

	"package-operator.run/internal/packages/internal/packagetypes"
)

// Import a RawPackage from the given folder path.
func FromFolder(ctx context.Context, path string) (*packagetypes.RawPackage, error) {
	return FromFS(ctx, os.DirFS(path))
}

// Import a RawPackage from the given FileSystem.
func FromFS(ctx context.Context, src fs.FS) (*packagetypes.RawPackage, error) {
	verboseLog := logr.FromContextOrDiscard(ctx).V(1)
	files := packagetypes.Files{}

	walker := func(path string, entry fs.DirEntry, ioErr error) error {
		switch {
		case ioErr != nil:
			return fmt.Errorf("access file %s: %w", path, ioErr)

		case entry.Name() == ".":
			// continue at root

		case entry.IsDir() && entry.Name() == packagetypes.PackageTestFixturesFolder:
			// include test fixtures

		case strings.HasPrefix(entry.Name(), "."):
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
			files[path] = data
		}

		return nil
	}

	if err := fs.WalkDir(src, ".", walker); err != nil {
		return nil, fmt.Errorf("walk source dir: %w", err)
	}
	return &packagetypes.RawPackage{
		Files: files,
	}, nil
}
