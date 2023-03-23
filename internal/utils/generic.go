package utils

import (
	"fmt"
	"os"
	"strings"

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

func ImageURLWithOverride(img string) (string, error) {
	if repoHostOverride := os.Getenv("PKO_REPOSITORY_HOST"); len(repoHostOverride) > 0 {
		ref, err := reference.ParseDockerRef(img)
		if err != nil {
			return "", fmt.Errorf("image \"%s\" with host \"%s\": %w", img, repoHostOverride, err)
		}
		return strings.Replace(ref.String(), reference.Domain(ref), repoHostOverride, 1), nil
	}
	return img, nil
}
