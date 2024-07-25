package presets

import (
	"fmt"
	"sort"

	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
)

// Remembers image references and gives them a unique name.
type ImageContainer map[string]string

func (c *ImageContainer) Add(containerName, image string) (imageName string) {
	m := *c

	imageName = containerName
	var i int
	for {
		i++
		_, nameAlreadyRegistered := m[imageName]
		if !nameAlreadyRegistered {
			m[imageName] = image
			break
		}
		imageName = fmt.Sprintf("%s-%d", containerName, i)
	}
	return
}

func (c *ImageContainer) List() []manifestsv1alpha1.PackageManifestImage {
	m := *c

	imageNames := make([]string, len(m))
	var i int
	for imageName := range m {
		imageNames[i] = imageName
		i++
	}
	sort.Strings(imageNames) // ensure output is deterministic

	images := make([]manifestsv1alpha1.PackageManifestImage, len(m))
	for i, imageName := range imageNames {
		images[i] = manifestsv1alpha1.PackageManifestImage{
			Name:  imageName,
			Image: m[imageName],
		}
	}
	return images
}
