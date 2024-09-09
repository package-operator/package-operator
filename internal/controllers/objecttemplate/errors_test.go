package objecttemplate

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var errTemplate = errors.New(`template: :6:25: executing "" at <.config.password>: map has no entry for key "password"`)

func TestTemplateError(t *testing.T) {
	t.Parallel()

	e := &TemplateError{Err: errTemplate}
	assert.Equal(t, `template: :6:25: at <.config.password>: map has no entry for key "password"`, e.Error())
}

func TestJSONPathFormatError(t *testing.T) {
	t.Parallel()

	e := &JSONPathFormatError{Path: "test"}
	assert.Equal(t, "path test must be a JSONPath with a leading dot", e.Error())
}

func TestSourceKeyNotFoundError(t *testing.T) {
	t.Parallel()

	e := &SourceKeyNotFoundError{Key: "test-key"}
	assert.Equal(t, "key test-key not found", e.Error())
}

func TestSourceError(t *testing.T) {
	t.Parallel()

	e := &SourceError{
		Source: &unstructured.Unstructured{},
	}
	assert.Equal(t, "for source  /: %!s(<nil>)", e.Error())
}
