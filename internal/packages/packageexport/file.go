package packageexport

import (
	"fmt"

	"github.com/google/go-containerregistry/pkg/crane"
	containerregistrypkgv1 "github.com/google/go-containerregistry/pkg/v1"

	"package-operator.run/internal/packages/packagecontent"
)

func File(dst string, tags []string, files packagecontent.Files) error {
	image, err := Image(files)
	if err != nil {
		return err
	}

	m := map[string]containerregistrypkgv1.Image{}
	for _, tag := range tags {
		m[tag] = image
	}
	if err := crane.MultiSave(m, dst); err != nil {
		return fmt.Errorf("dump to %s: %w", dst, err)
	}

	return nil
}
