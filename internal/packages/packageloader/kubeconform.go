package packageloader

import (
	"bytes"
	"io"
	"strings"

	"github.com/yannh/kubeconform/pkg/validator"
	"k8s.io/utils/ptr"

	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
	"package-operator.run/internal/packages"
)

type kubeconformValidator interface {
	Validate(filename string, r io.ReadCloser) []validator.Result
}

type noopKubeconformValidator struct{}

func (nv *noopKubeconformValidator) Validate(_ string, _ io.ReadCloser) []validator.Result {
	return nil
}

const (
	defaultKubeSchemaLocation = "https://raw.githubusercontent.com/yannh/kubernetes-json-schema/master/{{ .NormalizedKubernetesVersion }}-standalone{{ .StrictSuffix }}/{{ .ResourceKind }}{{ .KindSuffix }}.json"
	defaultCRDSSchemaLocation = "https://raw.githubusercontent.com/datreeio/CRDs-catalog/main/{{.Group}}/{{.ResourceKind}}_{{.ResourceAPIVersion}}.json"
)

func defaultKubeconformSchemaLocations(
	manifest *manifestsv1alpha1.PackageManifest,
) []string {
	if len(manifest.Test.Kubeconform.SchemaLocations) > 0 {
		return manifest.Test.Kubeconform.SchemaLocations
	}
	return []string{
		defaultKubeSchemaLocation,
		defaultCRDSSchemaLocation,
	}
}

func kubeconformValidatorFromManifest(
	manifest *manifestsv1alpha1.PackageManifest,
) (kubeconformValidator, error) {
	if manifest.Test.Kubeconform == nil {
		return &noopKubeconformValidator{}, nil
	}

	schemaLocations := defaultKubeconformSchemaLocations(manifest)
	return validator.New(schemaLocations, validator.Opts{
		Strict:               true,
		KubernetesVersion:    strings.TrimPrefix(manifest.Test.Kubeconform.KubernetesVersion, "v"),
		IgnoreMissingSchemas: true,
	})
}

type bufferCloser struct {
	*bytes.Buffer
}

func (bc *bufferCloser) Close() error {
	return nil
}

func runKubeconformForFile(
	path string, file []byte,
	kcV kubeconformValidator,
) (validationErrors []error, err error) {
	if !packages.IsYAMLFile(path) {
		return nil, nil
	}

	buf := bytes.NewBuffer(file)
	for i, res := range kcV.Validate(path, &bufferCloser{Buffer: buf}) {
		if res.Status == validator.Invalid ||
			res.Status == validator.Error {
			validationErrors = append(validationErrors, packages.ViolationError{
				Reason:  packages.ViolationReasonKubeconform,
				Path:    path,
				Index:   ptr.To(i),
				Details: res.Err.Error(),
			})
		}
	}
	return validationErrors, nil
}
