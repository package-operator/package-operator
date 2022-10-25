package v1alpha1

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

func TestPackageManifest_Validate(t *testing.T) {
	tests := []struct {
		name        string
		manifest    *PackageManifest
		errorString string // main error string that we are interested in, might return more.
	}{
		{
			name:        "missing .metadata.name",
			manifest:    &PackageManifest{},
			errorString: "metadata.name: Required value",
		},
		{
			name:        "missing .spec.scopes",
			manifest:    &PackageManifest{},
			errorString: "spec.scopes: Required value",
		},
		{
			name:        "missing .spec.availabilityProbes",
			manifest:    &PackageManifest{},
			errorString: "spec.availabilityProbes: Required value",
		},
		{
			name:        "missing .spec.phases",
			manifest:    &PackageManifest{},
			errorString: "spec.phases: Required value",
		},
		{
			name: "duplicated phase",
			manifest: &PackageManifest{
				Spec: PackageManifestSpec{
					Phases: []PackageManifestPhase{
						{Name: "test"},
						{Name: "test"},
					},
				},
			},
			errorString: "spec.phases[1].name: Invalid value: \"test\": must be unique",
		},
		{
			name: "duplicated phase",
			manifest: &PackageManifest{
				Spec: PackageManifestSpec{
					AvailabilityProbes: []corev1alpha1.ObjectSetProbe{
						{},
					},
				},
			},
			errorString: "spec.availabilityProbes[0].probes: Required value",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.manifest.Validate()
			var aggregateErr utilerrors.Aggregate
			require.True(t, errors.As(err, &aggregateErr))

			var errorStrings []string
			for _, err := range aggregateErr.Errors() {
				errorStrings = append(errorStrings, err.Error())
			}

			assert.Contains(t, errorStrings, test.errorString)
		})
	}
}
