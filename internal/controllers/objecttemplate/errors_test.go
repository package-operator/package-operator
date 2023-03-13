package objecttemplate

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

var errTemplate = errors.New(`template: :6:25: executing "" at <.config.password>: map has no entry for key "password"`)

func TestTemplateError(t *testing.T) {
	e := &TemplateError{Err: errTemplate}
	assert.Equal(t, `template: :6:25: at <.config.password>: map has no entry for key "password"`, e.Error())
}
