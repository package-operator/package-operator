package packages

import (
	"fmt"
	"path/filepath"
)

// imageFilenamePrefixPath defines under which subfolder files within a package container should be located.
const imageFilenamePrefixPath = "package"

// StripPathPrefix removes the path that is prefixed to packed package image files.
//
// This is done the prevent clutter in the top level directory of images and to avoid confusion.
func StripPathPrefix(path string) (string, error) {
	strippedPath, err := filepath.Rel(imageFilenamePrefixPath, path)
	if err != nil {
		return strippedPath, fmt.Errorf("package image contains files not under the dir %s: %w", imageFilenamePrefixPath, err)
	}

	return strippedPath, nil
}

// AddPathPrefix adds the path that is prefixed to packed package image files.
//
// This is done the prevent clutter in the top level directory of images and to avoid confusion.
func AddPathPrefix(path string) string { return filepath.Join(imageFilenamePrefixPath, path) }
