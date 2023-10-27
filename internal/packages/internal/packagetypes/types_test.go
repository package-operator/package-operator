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
	assert.NotSame(t, pkg, newPkg)                           // new instance
	assert.NotSame(t, pkg.Manifest, newPkg.Manifest)         // new instance
	assert.NotSame(t, pkg.ManifestLock, newPkg.ManifestLock) // new instance
	assertFilesCopy(t, pkg.Files, newPkg.Files)
}

func TestRawPackage_DeepCopy(t *testing.T) {
	t.Parallel()

	rawPkg := &RawPackage{
		Files: Files{"test": []byte("xxx")},
	}

	newRawPkg := rawPkg.DeepCopy()
	assert.NotSame(t, rawPkg, newRawPkg) // new instance
	assertFilesCopy(t, rawPkg.Files, newRawPkg.Files)
}

func TestFiles_DeepCopy(t *testing.T) {
	t.Parallel()

	f := Files{"test": []byte("xxx")}

	newF := f.DeepCopy()
	assertFilesCopy(t, f, newF)
}

func assertFilesCopy(t *testing.T, files, newFiles Files) {
	t.Helper()
	assert.NotSame(t, files, newFiles)                 // new map
	assert.NotSame(t, files["test"], newFiles["test"]) // new slice
	assert.Equal(t, files, newFiles)                   // equal content
}
