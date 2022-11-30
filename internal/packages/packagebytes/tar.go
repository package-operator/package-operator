package packagebytes

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"sort"
)

const tarPrefixPath = "package"

func toTar(fileMap FileMap) (tarBytes []byte, err error) {
	paths := make([]string, 0, len(fileMap))

	for path := range fileMap {
		paths = append(paths, path)
	}

	sort.Strings(paths)

	tarBuffer := &bytes.Buffer{}
	tarWriter := tar.NewWriter(tarBuffer)
	defer func() {
		if cErr := tarWriter.Close(); cErr != nil && err == nil {
			err = cErr
		}
	}()

	for _, path := range paths {
		data := fileMap[path]

		tarPath := filepath.Join(tarPrefixPath, path)
		if err := tarWriter.WriteHeader(&tar.Header{Name: tarPath, Size: int64(len(data))}); err != nil {
			return nil, fmt.Errorf("write tar header: %w", err)
		}

		if _, err := tarWriter.Write(data); err != nil {
			return nil, fmt.Errorf("write tar body: %w", err)
		}
	}

	return tarBuffer.Bytes(), nil
}

func fromTaredReader(ctx context.Context, reader io.Reader) (FileMap, error) {
	fileMap := FileMap{}
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
