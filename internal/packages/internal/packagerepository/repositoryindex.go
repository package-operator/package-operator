package packagerepository

import (
	"archive/tar"
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"

	"github.com/google/go-containerregistry/pkg/crane"
	containerregistrypkgv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	"package-operator.run/internal/apis/manifests"
	"package-operator.run/internal/packages/internal/packagestructure"
)

const filePathInRepo = "repository/repository.yaml"

// RepositoryIndex ties multiple PackageIndex objects together to represent a whole repository.
type RepositoryIndex struct {
	repo           *manifests.Repository
	packageIndexes map[string]*packageIndex
}

func newRepositoryIndex() *RepositoryIndex {
	return &RepositoryIndex{
		packageIndexes: map[string]*packageIndex{},
	}
}

func NewRepositoryIndex(meta metav1.ObjectMeta) *RepositoryIndex {
	ri := newRepositoryIndex()
	ri.repo = &manifests.Repository{
		ObjectMeta: meta,
	}
	return ri
}

func SaveRepositoryToOCI(ctx context.Context, idx *RepositoryIndex) (containerregistrypkgv1.Image, error) {
	buf := &bytes.Buffer{}
	if err := idx.Export(ctx, buf); err != nil {
		return nil, err
	}

	layer, err := crane.Layer(map[string][]byte{filePathInRepo: buf.Bytes()})
	if err != nil {
		return nil, fmt.Errorf("create image layer: %w", err)
	}

	image, err := mutate.AppendLayers(empty.Image, layer)
	if err != nil {
		return nil, fmt.Errorf("add layer to image: %w", err)
	}

	image, err = mutate.Canonical(image)
	if err != nil {
		return nil, fmt.Errorf("make image canonical: %w", err)
	}

	return image, nil
}

func LoadRepositoryFromOCI(ctx context.Context, img containerregistrypkgv1.Image) (idx *RepositoryIndex, err error) {
	reader := mutate.Extract(img)

	defer func() {
		if cErr := reader.Close(); err == nil && cErr != nil {
			err = cErr
		}
	}()

	tarReader := tar.NewReader(reader)

	for {
		hdr, err := tarReader.Next()
		if err != nil {
			return nil, fmt.Errorf("read from image tar: %w", err)
		}

		if hdr.Name != filePathInRepo {
			continue
		}

		return LoadRepository(ctx, tarReader)
	}
}

func LoadRepositoryFromFile(ctx context.Context, path string) (idx *RepositoryIndex, err error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() {
		if cErr := file.Close(); cErr != nil && err == nil {
			err = cErr
		}
	}()
	ri, err := LoadRepository(ctx, file)
	if err != nil {
		return nil, err
	}
	return ri, nil
}

func SaveRepositoryToFile(ctx context.Context, path string, idx *RepositoryIndex) (err error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer func() {
		if cErr := file.Close(); cErr != nil && err == nil {
			err = cErr
		}
	}()

	if err := idx.Export(ctx, file); err != nil {
		return err
	}

	return nil
}

func LoadRepository(ctx context.Context, r io.Reader) (*RepositoryIndex, error) {
	ri := newRepositoryIndex()
	scanner := bufio.NewScanner(r)
	scanner.Split(splitAt("---\n"))

	var repositoryObjectRead bool
	for scanner.Scan() {
		chunk := scanner.Bytes()
		chunk = bytes.TrimSpace(chunk)
		if len(chunk) == 0 {
			continue
		}

		if !repositoryObjectRead {
			repo, err := packagestructure.RepositoryFromFile(ctx, "", scanner.Bytes())
			if err != nil {
				return nil, err
			}
			ri.repo = repo
			repositoryObjectRead = true
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

func (ri *RepositoryIndex) IsEmpty() bool {
	return len(ri.packageIndexes) == 0
}

func (ri *RepositoryIndex) ListEntries(pkgName string) []manifests.RepositoryEntry {
	pi, exists := ri.packageIndexes[pkgName]
	if !exists {
		return nil
	}
	return pi.ListEntries()
}

func (ri *RepositoryIndex) ListAllEntries() []manifests.RepositoryEntry {
	// Calculate total capacity needed
	capacity := 0
	for _, pkgIdx := range ri.packageIndexes {
		capacity += len(pkgIdx.ListEntries())
	}

	entries := make([]manifests.RepositoryEntry, 0, capacity)
	for _, pkgIdx := range ri.packageIndexes {
		entries = append(entries, pkgIdx.ListEntries()...)
	}

	return entries
}

func (ri *RepositoryIndex) Metadata() *manifests.Repository {
	return ri.repo
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

func (ri *RepositoryIndex) ListVersions(pkgName string) ([]string, error) {
	pi, exists := ri.packageIndexes[pkgName]
	if !exists {
		return nil, newPackageNotFoundError(pkgName)
	}
	return pi.ListVersions(), nil
}

func (ri *RepositoryIndex) Add(ctx context.Context, entry *manifests.RepositoryEntry) error {
	pi, exists := ri.packageIndexes[entry.Data.Name]
	if !exists {
		pi = newPackageIndex(entry.Data.Name)
		ri.packageIndexes[entry.Data.Name] = pi
	}
	return pi.Add(ctx, entry)
}

func (ri *RepositoryIndex) Remove(
	ctx context.Context, entry *manifests.RepositoryEntry,
) error {
	pi, exists := ri.packageIndexes[entry.Data.Name]
	if !exists {
		return nil
	}
	if err := pi.Remove(ctx, entry); err != nil {
		return err
	}
	if pi.IsEmpty() {
		delete(ri.packageIndexes, entry.Data.Name)
	}
	return nil
}

func (ri *RepositoryIndex) Export(_ context.Context, w io.Writer) error {
	v1Repo, err := packagestructure.ToV1Alpha1Repository(ri.repo)
	if err != nil {
		return err
	}
	if v1Repo.CreationTimestamp.IsZero() {
		v1Repo.CreationTimestamp = metav1.Now()
	}
	v1RepoJSON, err := yaml.Marshal(v1Repo)
	if err != nil {
		return err
	}
	if _, err := w.Write([]byte("---\n")); err != nil {
		return err
	}
	if _, err := w.Write(v1RepoJSON); err != nil {
		return err
	}

	for _, pi := range ri.packageIndexes {
		for _, entry := range pi.ListEntries() {
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
