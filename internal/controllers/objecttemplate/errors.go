package objecttemplate

import (
	"fmt"

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
