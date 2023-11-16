package packagerepository

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/Masterminds/semver/v3"
	"golang.org/x/exp/slices"

	"package-operator.run/internal/apis/manifests"
	"package-operator.run/internal/packages/internal/packagemanifestvalidation"
)

type packageIndex struct {
	// name of the package
	name string

	orderedVersions semver.Collection
	versions        map[string]struct{}
	versionToDigest map[string]string
	digestToEntry   map[string]manifests.RepositoryEntry
}

func newPackageIndex(name string) *packageIndex {
	return &packageIndex{
		name:            name,
		versions:        map[string]struct{}{},
		versionToDigest: map[string]string{},
		digestToEntry:   map[string]manifests.RepositoryEntry{},
	}
}

func (pi *packageIndex) GetName() string {
	return pi.name
}

func (pi *packageIndex) IsEmpty() bool {
	return len(pi.digestToEntry) == 0
}

func (pi *packageIndex) GetLatestEntry() (*manifests.RepositoryEntry, error) {
	if len(pi.orderedVersions) == 0 {
		return nil, newPackageNotFoundError(pi.name)
	}
	latest := versionToString(pi.orderedVersions[0])
	return pi.GetVersion(latest)
}

func (pi *packageIndex) GetVersion(version string) (*manifests.RepositoryEntry, error) {
	digest, ok := pi.versionToDigest[version]
	if !ok {
		return nil, newPackageVersionNotFoundError(pi.name, version)
	}
	return pi.GetDigest(digest)
}

func (pi *packageIndex) GetDigest(digest string) (*manifests.RepositoryEntry, error) {
	entry, ok := pi.digestToEntry[digest]
	if !ok {
		return nil, newPackageDigestNotFoundError(pi.name, digest)
	}
	return entry.DeepCopy(), nil
}

func (pi *packageIndex) ListVersions() []string {
	list := make([]string, len(pi.orderedVersions))
	for i, sv := range pi.orderedVersions {
		v := versionToString(sv)
		list[i] = v
	}
	return list
}

func (pi *packageIndex) ListEntries() []manifests.RepositoryEntry {
	list := make([]manifests.RepositoryEntry, len(pi.digestToEntry))
	var i int
	for _, entry := range pi.digestToEntry {
		list[i] = entry
		i++
	}
	return list
}

func (pi *packageIndex) Add(ctx context.Context, entry *manifests.RepositoryEntry) error {
	if errs, err := packagemanifestvalidation.ValidateRepositoryEntry(ctx, entry); err != nil {
		return err
	} else if len(errs) > 0 {
		return errs.ToAggregate()
	}

	if pi.name != entry.Name {
		panic(fmt.Sprintf("package index for package named %s, got: %s", pi.name, entry.Name))
	}

	var entryOrderedVersions semver.Collection
	for _, v := range entry.Data.Versions {
		if _, ok := pi.versions[v]; ok {
			return newPackageVersionAlreadyExistsError(pi.name, v)
		}

		sv, err := semver.StrictNewVersion(strings.TrimPrefix(v, "v"))
		if err != nil {
			return err
		}
		pi.versions[v] = struct{}{}
		entryOrderedVersions = append(entryOrderedVersions, sv)
		pi.orderedVersions = append(pi.orderedVersions, sv)
		pi.versionToDigest[v] = entry.Data.Digest
	}
	sort.Sort(pi.orderedVersions)
	slices.Reverse(pi.orderedVersions)
	sort.Sort(entryOrderedVersions)
	slices.Reverse(entryOrderedVersions)

	entry.Data.Versions = nil
	for _, sv := range entryOrderedVersions {
		entry.Data.Versions = append(entry.Data.Versions, versionToString(sv))
	}
	pi.digestToEntry[entry.Data.Digest] = *entry
	return nil
}

func (pi *packageIndex) Remove(_ context.Context, entry *manifests.RepositoryEntry) error {
	var (
		orderedVersions  semver.Collection
		versionsToRemove = map[string]struct{}{}
	)
	entry, err := pi.GetDigest(entry.Data.Digest)
	if err != nil {
		return err
	}
	for _, v := range entry.Data.Versions {
		delete(pi.versions, v)
		delete(pi.versionToDigest, v)
		versionsToRemove[v] = struct{}{}
	}
	for _, sv := range pi.orderedVersions {
		if _, remove := versionsToRemove[versionToString(sv)]; remove {
			continue
		}
		orderedVersions = append(orderedVersions, sv)
	}
	pi.orderedVersions = orderedVersions
	delete(pi.digestToEntry, entry.Data.Digest)
	return nil
}

func versionToString(sv *semver.Version) string {
	return "v" + sv.String()
}
