package packageexport

import (
	"fmt"

	"github.com/google/go-containerregistry/pkg/crane"
	v1 "github.com/google/go-containerregistry/pkg/v1"

	"package-operator.run/internal/packages/packagecontent"
)

func File(dst string, tags []string, pkg *packagecontent.Package) error {
	image, err := Image(pkg)
	if err != nil {
		return err
	}

	m := map[string]v1.Image{}
	for _, tag := range tags {
		m[tag] = image
	}
	if err := crane.MultiSave(m, dst); err != nil {
		return fmt.Errorf("dump to %s: %w", dst, err)
	}

	return nil
}
