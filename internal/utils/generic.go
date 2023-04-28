package utils

import (
	"fmt"
	"os"
	"strings"

	"package-operator.run/apis/manifests/v1alpha1"

	"github.com/docker/distribution/reference"
)

// Slice contains check.
func Contains[T comparable](elems []T, v T) bool {
	for _, s := range elems {
		if v == s {
			return true
		}
	}
	return false
}

func MergeKeysFrom(base, additional map[string]string) map[string]string {
	if base == nil {
		base = map[string]string{}
	}
	for k, v := range additional {
		base[k] = v
	}
	if len(base) == 0 {
		return nil
	}
	return base
}

func CopyMap[K comparable, V interface{}](toCopy map[K]V) map[K]V {
	out := map[K]V{}
	for k, v := range toCopy {
		out[k] = v
	}
	return out
}

func ImageURLWithOverrideFromEnv(img string) (string, error) {
	return ImageURLWithOverride(img, os.Getenv("PKO_REPOSITORY_HOST"))
}

func ImageURLWithOverride(img string, override string) (string, error) {
	if len(override) == 0 {
		return img, nil
	}
	ref, err := reference.ParseDockerRef(img)
	if err != nil {
		return "", fmt.Errorf("image \"%s\" with host \"%s\": %w", img, override, err)
	}
	return strings.Replace(ref.String(), reference.Domain(ref), override, 1), nil
}

// GenerateStaticImages generates a static set of images to be used for tests and other purposes
func GenerateStaticImages(manifest *v1alpha1.PackageManifest) map[string]string {
	images := map[string]string{}
	for _, v := range manifest.Spec.Images {
		images[v.Name] = "registry.package-operator.run/static-image"
	}
	return images
}
