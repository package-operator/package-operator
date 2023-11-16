package packagerepository

import "fmt"

type NotFoundError struct {
	Name    string
	Version string
	Digest  string
}

func newPackageNotFoundError(name string) *NotFoundError {
	return &NotFoundError{
		Name: name,
	}
}

func newPackageVersionNotFoundError(name, version string) *NotFoundError {
	return &NotFoundError{
		Name:    name,
		Version: version,
	}
}

func newPackageDigestNotFoundError(name, digest string) *NotFoundError {
	return &NotFoundError{
		Name:   name,
		Digest: digest,
	}
}

func (e *NotFoundError) Error() string {
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
