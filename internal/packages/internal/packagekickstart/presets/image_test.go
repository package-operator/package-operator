package presets

import (
	"testing"

	"github.com/stretchr/testify/assert"

	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
)

func TestImageContainer(t *testing.T) {
	t.Parallel()
	containerName := "banana"
	image := "quay.io/xxx/xxx:v1"

	ic := ImageContainer{}
	a1 := ic.Add(containerName, image)
	assert.Equal(t, containerName, a1)
	a2 := ic.Add(containerName, image)
	assert.Equal(t, "banana-1", a2)
	a3 := ic.Add(containerName, image)
	assert.Equal(t, "banana-2", a3)

	assert.Equal(t, []manifestsv1alpha1.PackageManifestImage{
		{
			Name:  a1,
			Image: image,
		},
		{
			Name:  a2,
			Image: image,
		},
		{
			Name:  a3,
			Image: image,
		},
	}, ic.List())
}
