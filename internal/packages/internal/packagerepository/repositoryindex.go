package packagerepository

import (
	"context"

	"package-operator.run/internal/apis/manifests"
)

type RepositoryIndex struct {
	packageIndexes map[string]*packageIndex
}

func NewRepositoryIndex() *RepositoryIndex {
	return &RepositoryIndex{
		packageIndexes: map[string]*packageIndex{},
	}
}

func (ri *RepositoryIndex) Add(ctx context.Context, entry *manifests.RepositoryEntry) error {
	pi, exists := ri.packageIndexes[entry.Name]
	if !exists {
		pi = newPackageIndex(entry.Name)
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
