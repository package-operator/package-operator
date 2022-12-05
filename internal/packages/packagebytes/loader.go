package packagebytes

import (
	"archive/tar"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-logr/logr"
	"github.com/google/go-containerregistry/pkg/crane"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
)

// Maps filenames to file contents.
type FileMap = map[string][]byte

type Loader struct{}

func NewLoader() *Loader {
	return &Loader{}
}

// FromFolder returns a FileMap containing contents from the given path.
func (l *Loader) FromFolder(ctx context.Context, path string) (FileMap, error) {
	return l.FromFS(ctx, os.DirFS(path))
}

// FromFS returns a FileMap containing contents from the given fs.FS.
func (l *Loader) FromFS(ctx context.Context, src fs.FS) (FileMap, error) {
	verboseLog := logr.FromContextOrDiscard(ctx).V(1)
	bundle := FileMap{}
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

func (l *Loader) FromImage(ctx context.Context, image v1.Image) (m FileMap, err error) {
	fileMap := FileMap{}
	reader := mutate.Extract(image)
	defer func() {
		if cErr := reader.Close(); err == nil && cErr != nil {
			err = cErr
		}
	}()
	tarReader := tar.NewReader(reader)

	for {
		hdr, err := tarReader.Next()
		if err != nil && errors.Is(err, io.EOF) {
			break
		}

		tarPath := hdr.Name
		path, err := filepath.Rel(tarPrefixPath, tarPath)
		if err != nil {
			return nil, fmt.Errorf("package image contains files not under the dir %s: %w", tarPrefixPath, err)
		}

		if err != nil {
			return nil, fmt.Errorf("read file header from layer: %w", err)
		}

		data, err := io.ReadAll(tarReader)
		if err != nil {
			return nil, fmt.Errorf("read file header from layer: %w", err)
		}

		fileMap[path] = data
	}

	return fileMap, nil
}

func (l *Loader) FromPulledImage(ctx context.Context, ref string) (FileMap, error) {
	img, err := crane.Pull(ref)
	if err != nil {
		return nil, err
	}

	return l.FromImage(ctx, img)
}

func isFileToBeExcluded(entry fs.DirEntry) bool {
	return strings.HasPrefix(entry.Name(), ".")
}
