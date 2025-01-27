package packagetypes

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"package-operator.run/internal/apis/manifests"
)

func TestPackage_DeepCopy(t *testing.T) {
	t.Parallel()
	pkg := &Package{
		Manifest:     &manifests.PackageManifest{},
		ManifestLock: &manifests.PackageManifestLock{},
		Files:        Files{"test": []byte("xxx")},
	}
	newPkg := pkg.DeepCopy()
	assert.NotSame(t, pkg, newPkg)                                     // new instance
	assert.NotSame(t, pkg.Manifest, newPkg.Manifest)                   // new instance
	assert.NotSame(t, pkg.ManifestLock, newPkg.ManifestLock)           // new instance
	assert.NotSame(t, &pkg.Files, &newPkg.Files)                       // new map
	assert.NotSame(t, &pkg.Files["test"][0], &newPkg.Files["test"][0]) // new slice
	assert.Equal(t, pkg.Files, newPkg.Files)                           // equal content
}

func TestRawPackage_DeepCopy(t *testing.T) {
	t.Parallel()

	rawPkg := &RawPackage{
		Files: Files{"test": []byte("xxx")},
	}

	newRawPkg := rawPkg.DeepCopy()
	assert.NotSame(t, rawPkg, newRawPkg)                                     // new instance
	assert.NotSame(t, &rawPkg.Files, &newRawPkg.Files)                       // new map
	assert.NotSame(t, &rawPkg.Files["test"][0], &newRawPkg.Files["test"][0]) // new slice
	assert.Equal(t, rawPkg.Files, newRawPkg.Files)                           // equal content
}

func TestFiles_DeepCopy(t *testing.T) {
	t.Parallel()

	f := Files{"test": []byte("xxx")}

	newF := f.DeepCopy()
	assert.NotSame(t, &f["test"][0], &newF["test"][0]) // new slice
	assert.Equal(t, f, newF)                           // equal content
}
