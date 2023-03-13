package objecttemplate

import (
	"fmt"
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type SourceError struct {
	Source client.Object
	Err    error
}

func (e *SourceError) Error() string {
	return fmt.Sprintf("for source %s %s: %s",
		e.Source.GetObjectKind().GroupVersionKind().Kind,
		client.ObjectKeyFromObject(e.Source), e.Err)
}

type SourceKeyNotFoundError struct {
	Key string
}

func (e *SourceKeyNotFoundError) Error() string {
	return fmt.Sprintf("key %s not found", e.Key)
}

type TemplateError struct {
	Err error
}

func (e *TemplateError) Error() string {
	// sanitize template error output a bit
	return strings.Replace(e.Err.Error(), `executing "" `, "", 1)
}
