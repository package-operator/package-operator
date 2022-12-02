package packagestructure

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
)

func TestValidatorList(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		t1 := &ValidatorMock{}
		t2 := &ValidatorMock{}

		t1.On("Validate", mock.Anything, mock.Anything).Return(nil)
		t2.On("Validate", mock.Anything, mock.Anything).Return(nil)

		tl := ValidatorList{
			t1, t2,
		}

		ctx := context.Background()
		err := tl.Validate(ctx, nil)
		require.Nil(t, err)

		t1.AssertCalled(t, "Validate", mock.Anything, mock.Anything)
		t2.AssertCalled(t, "Validate", mock.Anything, mock.Anything)
	})

	t.Run("gathers all errors", func(t *testing.T) {
		t1 := &ValidatorMock{}
		t2 := &ValidatorMock{}

		t1.On("Validate", mock.Anything, mock.Anything).
			Return(NewInvalidError(Violation{Reason: "too wet"}))
		t2.On("Validate", mock.Anything, mock.Anything).
			Return(NewInvalidError(Violation{Reason: "on fire"}))

		tl := ValidatorList{
			t1, t2,
		}

		ctx := context.Background()
		err := tl.Validate(ctx, nil)
		assert.EqualError(t, err, `Package validation errors:
- too wet
- on fire
`)

		t1.AssertCalled(t, "Validate", mock.Anything, mock.Anything)
		t2.AssertCalled(t, "Validate", mock.Anything, mock.Anything)
	})
}

var (
	_ Validator = (*ValidatorMock)(nil)
)

type ValidatorMock struct {
	mock.Mock
}

func (m *ValidatorMock) Validate(ctx context.Context, packageContent *PackageContent) *InvalidError {
	args := m.Called(ctx, packageContent)
	if e, ok := args.Get(0).(*InvalidError); ok {
		return e
	}
	return nil
}

func TestObjectPhaseAnnotationValidator(t *testing.T) {
	opav := &ObjectPhaseAnnotationValidator{}

	okObj := unstructured.Unstructured{}
	okObj.SetAnnotations(map[string]string{
		manifestsv1alpha1.PackagePhaseAnnotation: "something",
	})
	packageContent := &PackageContent{
		Manifests: ManifestMap{
			"test.yaml": []unstructured.Unstructured{
				{}, okObj,
			},
		},
	}

	ctx := context.Background()
	err := opav.Validate(ctx, packageContent)
	require.EqualError(t, err, "Package validation errors:\n- Missing package-operator.run/phase Annotation in test.yaml#0\n")
}

func TestObjectGVKValidator(t *testing.T) {
	ogvkv := &ObjectGVKValidator{}

	okObj := unstructured.Unstructured{}
	okObj.SetGroupVersionKind(schema.GroupVersionKind{
		Version: "v1",
		Kind:    "Secret",
	})
	packageContent := &PackageContent{
		Manifests: ManifestMap{
			"test.yaml": []unstructured.Unstructured{
				{}, okObj,
			},
		},
	}

	ctx := context.Background()
	err := ogvkv.Validate(ctx, packageContent)
	require.EqualError(t, err, "Package validation errors:\n- GroupVersionKind not set in test.yaml#0\n")
}

func TestObjectLabelsValidator(t *testing.T) {
	olv := &ObjectLabelsValidator{}

	failObj := unstructured.Unstructured{}
	failObj.SetLabels(map[string]string{
		"/123": "test",
	})
	packageContent := &PackageContent{
		Manifests: ManifestMap{
			"test.yaml": []unstructured.Unstructured{
				{}, failObj,
			},
		},
	}

	ctx := context.Background()
	err := olv.Validate(ctx, packageContent)
	require.EqualError(t, err, "Package validation errors:\n- Labels invalid metadata.labels: Invalid value: \"/123\": prefix part must be non-empty in test.yaml#1\n")
}

func TestPackageScopeValidator(t *testing.T) {
	scopeV := PackageScopeValidator(
		manifestsv1alpha1.PackageManifestScopeCluster)

	ctx := context.Background()
	err := scopeV.Validate(ctx, &PackageContent{
		PackageManifest: &manifestsv1alpha1.PackageManifest{
			Spec: manifestsv1alpha1.PackageManifestSpec{
				Scopes: []manifestsv1alpha1.PackageManifestScope{
					manifestsv1alpha1.PackageManifestScopeNamespaced,
				},
			},
		},
	})
	require.EqualError(t, err, "Package validation errors:\n- Package unsupported scope in manifest.yaml\n")
}
