package packagetypes

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"package-operator.run/internal/apis/manifests"
)

var errValidation = errors.New("validation error")

func TestValidateEachComponent(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		Package         *Package
		ShouldFail      bool
		ValidateFnCount int
	}{
		"SimpleValid": {
			Package: &Package{
				Manifest: &manifests.PackageManifest{ObjectMeta: metav1.ObjectMeta{Name: "pkg"}},
			},
			ShouldFail:      false,
			ValidateFnCount: 1,
		},
		"SimpleInvalid": {
			Package: &Package{
				Manifest: &manifests.PackageManifest{ObjectMeta: metav1.ObjectMeta{Name: "pkg"}},
			},
			ShouldFail:      true,
			ValidateFnCount: 1,
		},
		"MultiValid": {
			Package: &Package{
				Manifest: &manifests.PackageManifest{ObjectMeta: metav1.ObjectMeta{Name: "pkg"}},
				Components: []Package{
					{Manifest: &manifests.PackageManifest{ObjectMeta: metav1.ObjectMeta{Name: "cmp1"}}},
					{Manifest: &manifests.PackageManifest{ObjectMeta: metav1.ObjectMeta{Name: "cmp2"}}},
					{Manifest: &manifests.PackageManifest{ObjectMeta: metav1.ObjectMeta{Name: "cmp3"}}},
				},
			},
			ShouldFail:      false,
			ValidateFnCount: 4,
		},
		"MultiInvalid": {
			Package: &Package{
				Manifest: &manifests.PackageManifest{ObjectMeta: metav1.ObjectMeta{Name: "pkg"}},
				Components: []Package{
					{Manifest: &manifests.PackageManifest{ObjectMeta: metav1.ObjectMeta{Name: "cmp1"}}},
					{Manifest: &manifests.PackageManifest{ObjectMeta: metav1.ObjectMeta{Name: "cmp2"}}},
					{Manifest: &manifests.PackageManifest{ObjectMeta: metav1.ObjectMeta{Name: "cmp3"}}},
				},
			},
			ShouldFail:      true,
			ValidateFnCount: 1,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ctx := t.Context()

			validateFnCount := 0
			validateFn := func(context.Context, *Package, bool) error {
				validateFnCount++
				if tc.ShouldFail {
					return errValidation
				}
				return nil
			}

			err := ValidateEachComponent(ctx, tc.Package, validateFn)
			if tc.ShouldFail {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tc.ValidateFnCount, validateFnCount)
		})
	}
}
