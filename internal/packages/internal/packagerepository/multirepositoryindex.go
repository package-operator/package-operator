package packagerepository

import (
	"context"
	"fmt"
	"io"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"package-operator.run/internal/apis/manifests"
)

// MultiRepositoryIndex provides an interface to interact with repositories.
type MultiRepositoryIndex struct {
	repos map[string]*RepositoryIndex
}

func NewMultiRepositoryIndex() *MultiRepositoryIndex {
	return &MultiRepositoryIndex{
		repos: map[string]*RepositoryIndex{},
	}
}

type Entry struct {
	*manifests.RepositoryEntry
	RepositoryName string
}

func (e *Entry) FQDN() string {
	return fmt.Sprintf("%s.%s", e.Data.Name, e.RepositoryName)
}

func (mri *MultiRepositoryIndex) Add(ctx context.Context, entry Entry) error {
	repo, exists := mri.repos[entry.RepositoryName]
	if !exists {
		repo = NewRepositoryIndex(metav1.ObjectMeta{
			Name: entry.RepositoryName,
		})
		mri.repos[entry.RepositoryName] = repo
	}
	return repo.Add(ctx, entry.RepositoryEntry)
}

func (mri *MultiRepositoryIndex) Remove(ctx context.Context, entry Entry) error {
	repo, exists := mri.repos[entry.RepositoryName]
	if !exists {
		return nil
	}

	if err := repo.Remove(ctx, entry.RepositoryEntry); err != nil {
		return err
	}
	if repo.IsEmpty() {
		delete(mri.repos, entry.RepositoryName)
	}
	return nil
}

func (mri *MultiRepositoryIndex) ListEntries(repoName, pkgName string) []Entry {
	repo, exists := mri.repos[repoName]
	if !exists {
		return nil
	}

	entries := repo.ListEntries(pkgName)
	outEntries := make([]Entry, len(entries))
	for i := range entries {
		outEntries[i] = Entry{
			RepositoryEntry: &entries[i],
			RepositoryName:  repoName,
		}
	}
	return outEntries
}

func (mri *MultiRepositoryIndex) GetLatestEntry(repoName, pkgName string) (Entry, error) {
	repo, exists := mri.repos[repoName]
	if !exists {
		return Entry{}, newPackageNotFoundError(pkgName)
	}
	entry, err := repo.GetLatestEntry(pkgName)
	if err != nil {
		return Entry{}, err
	}
	return Entry{
		RepositoryEntry: entry,
		RepositoryName:  repoName,
	}, nil
}

func (mri *MultiRepositoryIndex) GetVersion(repoName, pkgName, version string) (Entry, error) {
	repo, exists := mri.repos[repoName]
	if !exists {
		return Entry{}, newPackageNotFoundError(pkgName)
	}
	entry, err := repo.GetVersion(pkgName, version)
	if err != nil {
		return Entry{}, err
	}
	return Entry{
		RepositoryEntry: entry,
		RepositoryName:  repoName,
	}, nil
}

func (mri *MultiRepositoryIndex) GetDigest(repoName, pkgName, digest string) (Entry, error) {
	repo, exists := mri.repos[repoName]
	if !exists {
		return Entry{}, newPackageNotFoundError(pkgName)
	}
	entry, err := repo.GetDigest(pkgName, digest)
	if err != nil {
		return Entry{}, err
	}
	return Entry{
		RepositoryEntry: entry,
		RepositoryName:  repoName,
	}, nil
}

func (mri *MultiRepositoryIndex) ListVersions(repoName, pkgName string) ([]string, error) {
	repo, exists := mri.repos[repoName]
	if !exists {
		return nil, nil
	}
	return repo.ListVersions(pkgName)
}

func (mri *MultiRepositoryIndex) GetRepository(repoName string) (*RepositoryIndex, error) {
	repo, exists := mri.repos[repoName]
	if !exists {
		return nil, newRepositoryNotFoundError(repoName)
	}
	return repo, nil
}

func (mri *MultiRepositoryIndex) LoadRepositoryFromFile(ctx context.Context, fileName string) error {
	repo, err := LoadRepositoryFromFile(ctx, fileName)
	if err != nil {
		return err
	}
	mri.repos[repo.Metadata().Name] = repo
	return nil
}

func (mri *MultiRepositoryIndex) LoadRepository(ctx context.Context, file io.Reader) error {
	repo, err := LoadRepository(ctx, file)
	if err != nil {
		return err
	}
	mri.repos[repo.Metadata().Name] = repo
	return nil
}
