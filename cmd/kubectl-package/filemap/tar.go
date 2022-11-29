package filemap

import (
	"archive/tar"
	"bytes"
	"errors"
	"fmt"
	"io"
	"sort"
)

func ToTar(fileMap FileMap) (tarBytes []byte, err error) {
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
		if err := tarWriter.WriteHeader(&tar.Header{Name: path, Size: int64(len(data))}); err != nil {
			return nil, fmt.Errorf("write tar header: %w", err)
		}

		if _, err := tarWriter.Write(data); err != nil {
			return nil, fmt.Errorf("write tar body: %w", err)
		}
	}

	return tarBuffer.Bytes(), nil
}

func FromTaredReader(reader io.Reader) (FileMap, error) {
	fileMap := FileMap{}
	tarReader := tar.NewReader(reader)
	for {
		hdr, err := tarReader.Next()
		if err != nil && errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			return nil, fmt.Errorf("read file header from layer: %w", err)
		}

		data, err := io.ReadAll(tarReader)
		if err != nil {
			return nil, fmt.Errorf("read file header from layer: %w", err)
		}

		fileMap[hdr.Name] = data
	}

	return fileMap, nil
}
