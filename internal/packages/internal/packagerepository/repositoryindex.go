package packagerepository

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"os"

	"sigs.k8s.io/yaml"

	"package-operator.run/internal/apis/manifests"
	"package-operator.run/internal/packages/internal/packagestructure"
)

type RepositoryIndex struct {
	packageIndexes map[string]*packageIndex
}

func NewRepositoryIndex() *RepositoryIndex {
	return &RepositoryIndex{
		packageIndexes: map[string]*packageIndex{},
	}
}

func LoadRepositoryFromFile(ctx context.Context, path string) (*RepositoryIndex, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	ri, err := LoadRepository(ctx, file)
	if err != nil {
		return nil, err
	}
	return ri, nil
}

func LoadRepository(ctx context.Context, r io.Reader) (*RepositoryIndex, error) {
	ri := NewRepositoryIndex()
	scanner := bufio.NewScanner(r)
	scanner.Split(splitAt("---\n"))
	for scanner.Scan() {
		chunk := scanner.Bytes()
		chunk = bytes.TrimSpace(chunk)
		if len(chunk) == 0 {
			continue
		}

		entry, err := packagestructure.RepositoryEntryFromFile(ctx, "", scanner.Bytes())
		if err != nil {
			return nil, err
		}
		if err := ri.Add(ctx, entry); err != nil {
			return nil, err
		}
	}
	return ri, nil
}

func (ri *RepositoryIndex) ListEntries(pkgName string) []manifests.RepositoryEntry {
	pi, exists := ri.packageIndexes[pkgName]
	if !exists {
		return nil
	}
	return pi.ListEntries()
}

func (ri *RepositoryIndex) GetLatestEntry(pkgName string) (*manifests.RepositoryEntry, error) {
	pi, exists := ri.packageIndexes[pkgName]
	if !exists {
		return nil, newPackageNotFoundError(pkgName)
	}
	return pi.GetLatestEntry()
}

func (ri *RepositoryIndex) GetVersion(pkgName, version string) (*manifests.RepositoryEntry, error) {
	pi, exists := ri.packageIndexes[pkgName]
	if !exists {
		return nil, newPackageNotFoundError(pkgName)
	}
	return pi.GetVersion(version)
}

func (ri *RepositoryIndex) GetDigest(pkgName, digest string) (*manifests.RepositoryEntry, error) {
	pi, exists := ri.packageIndexes[pkgName]
	if !exists {
		return nil, newPackageNotFoundError(pkgName)
	}
	return pi.GetDigest(digest)
}

func (ri *RepositoryIndex) ListVersions(pkgName string) []string {
	pi, exists := ri.packageIndexes[pkgName]
	if !exists {
		return nil
	}
	return pi.ListVersions()
}

func (ri *RepositoryIndex) Add(ctx context.Context, entry *manifests.RepositoryEntry) error {
	pi, exists := ri.packageIndexes[entry.Name]
	if !exists {
		pi = newPackageIndex(entry.Name)
		ri.packageIndexes[entry.Name] = pi
	}
	return pi.Add(ctx, entry)
}

func (ri *RepositoryIndex) Remove(
	ctx context.Context, entry *manifests.RepositoryEntry,
) error {
	pi, exists := ri.packageIndexes[entry.Name]
	if !exists {
		return nil
	}
	if err := pi.Remove(ctx, entry); err != nil {
		return err
	}
	if pi.IsEmpty() {
		delete(ri.packageIndexes, entry.Name)
	}
	return nil
}

func (ri *RepositoryIndex) Export(_ context.Context, w io.Writer) error {
	for _, pi := range ri.packageIndexes {
		for _, entry := range pi.ListEntries() {
			entry := entry

			v1Entry, err := packagestructure.ToV1Alpha1RepositoryEntry(&entry)
			if err != nil {
				return err
			}
			v1json, err := yaml.Marshal(v1Entry)
			if err != nil {
				return err
			}
			if _, err := w.Write([]byte("---\n")); err != nil {
				return err
			}
			if _, err := w.Write(v1json); err != nil {
				return err
			}
		}
	}
	return nil
}

func splitAt(substring string) func(data []byte, atEOF bool) (advance int, token []byte, err error) {
	searchBytes := []byte(substring)
	searchLen := len(searchBytes)
	return func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		dataLen := len(data)

		// Return nothing if at end of file and no data passed
		if atEOF && dataLen == 0 {
			return 0, nil, nil
		}

		// Find next separator and return token
		if i := bytes.Index(data, searchBytes); i >= 0 {
			return i + searchLen, data[0:i], nil
		}

		// If we're at EOF, we have a final, non-terminated line. Return it.
		if atEOF {
			return dataLen, data, nil
		}

		// Request more data.
		return 0, nil, nil
	}
}
