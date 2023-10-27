package utils

import (
	"fmt"
	"os"

	"github.com/google/go-containerregistry/pkg/name"

	"package-operator.run/internal/apis/manifests"
)

func ImageURLWithOverrideFromEnv(img string) (string, error) {
	return ImageURLWithOverride(img, os.Getenv("PKO_REPOSITORY_HOST"))
}

// ImageURLWithOverride replaces the registry of the given image reference with the registry defined by override.
func ImageURLWithOverride(img string, override string) (string, error) {
	if override == "" {
		return img, nil
	}

	// Parse img and override parameter into something we cann use.

	registry, err := name.NewRegistry(override)
	if err != nil {
		return "", fmt.Errorf("image registry override: %w", err)
	}

	ref, err := name.ParseReference(img)
	if err != nil {
		return "", fmt.Errorf(`image reference: %w`, err)
	}

	// Reference can either be tagged by name or by digest. Handle both explicitly.
	switch v := ref.(type) {
	case name.Digest:
		// Override registry.
		v.Registry = registry
		// Rewrite repository the be below the registry.
		v.Repository = registry.Repo(ref.Context().RepositoryStr())
		// Return full name of the reference
		return v.Name(), nil
	case name.Tag:
		// Override registry.
		v.Registry = registry
		// Rewrite repository the be below the registry.
		v.Repository = registry.Repo(ref.Context().RepositoryStr())
		// Return full name of the reference
		return v.Name(), nil
	default:
		panic(fmt.Sprintf("unknown reference type: %T", v))
	}
}

// GenerateStaticImages generates a static set of images to be used for tests and other purposes.
func GenerateStaticImages(manifest *manifests.PackageManifest) map[string]string {
	images := map[string]string{}
	for _, v := range manifest.Spec.Images {
		images[v.Name] = "registry.package-operator.run/static-image"
	}
	return images
}
