package utils

import (
	"fmt"
	"os"
	"strings"

	"github.com/docker/distribution/reference"

	"package-operator.run/apis/manifests/v1alpha1"
)

func ImageURLWithOverrideFromEnv(img string) (string, error) {
	return ImageURLWithOverride(img, os.Getenv("PKO_REPOSITORY_HOST"))
}

func ImageURLWithOverride(img string, override string) (string, error) {
	if len(override) == 0 {
		return img, nil
	}

	ref, err := reference.ParseDockerRef(img)
	if err != nil {
		return "", fmt.Errorf(`image "%s" with host "%s": %w`, img, override, err)
	}

	return strings.Replace(ref.String(), reference.Domain(ref), override, 1), nil
}

// GenerateStaticImages generates a static set of images to be used for tests and other purposes.
func GenerateStaticImages(manifest *v1alpha1.PackageManifest) map[string]string {
	images := map[string]string{}
	for _, v := range manifest.Spec.Images {
		images[v.Name] = "registry.package-operator.run/static-image"
	}
	return images
}
