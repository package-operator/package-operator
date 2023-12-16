package packagerepository

import "fmt"

type RepositoryNotFoundError struct {
	Name string
}

func (e *RepositoryNotFoundError) Error() string {
	return fmt.Sprintf("repository %q not found", e.Name)
}

func newRepositoryNotFoundError(name string) *RepositoryNotFoundError {
	return &RepositoryNotFoundError{
		Name: name,
	}
}

type PackageNotFoundError struct {
	Name    string
	Version string
	Digest  string
}

func newPackageNotFoundError(name string) *PackageNotFoundError {
	return &PackageNotFoundError{
		Name: name,
	}
}

func newPackageVersionNotFoundError(name, version string) *PackageNotFoundError {
	return &PackageNotFoundError{
		Name:    name,
		Version: version,
	}
}

func newPackageDigestNotFoundError(name, digest string) *PackageNotFoundError {
	return &PackageNotFoundError{
		Name:   name,
		Digest: digest,
	}
}

func (e *PackageNotFoundError) Error() string {
	msg := fmt.Sprintf("package %q", e.Name)
	switch {
	case len(e.Version) > 0:
		msg = fmt.Sprintf("%s version %q", msg, e.Version)
	case len(e.Digest) > 0:
		msg = fmt.Sprintf("%s digest %q", msg, e.Digest)
	}
	return msg + " not found"
}

type AlreadyExistsError struct {
	Name    string
	Version string
}

func newPackageVersionAlreadyExistsError(name, version string) *AlreadyExistsError {
	return &AlreadyExistsError{
		Name:    name,
		Version: version,
	}
}

func (e *AlreadyExistsError) Error() string {
	return fmt.Sprintf("package %q version %q already exists", e.Name, e.Version)
}
