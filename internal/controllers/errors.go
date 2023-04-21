package controllers

import (
	"errors"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

func IsExternalResourceNotFound(err error) bool {
	var ctrlErr ControllerError

	return errors.As(err, &ctrlErr) && ctrlErr.CausedBy(ErrorReasonExternalResourceNotFound)
}

type ControllerError interface {
	CausedBy(reason ErrorReason) bool
}

type ErrorReason string

func (r ErrorReason) String() string {
	return string(r)
}

const (
	ErrorReasonExternalResourceNotFound ErrorReason = "external resource not found"
)

func NewExternalResourceNotFoundError(rsrc client.Object) *PhaseReconcilerError {
	return &PhaseReconcilerError{
		rsrc:   rsrc,
		reason: ErrorReasonExternalResourceNotFound,
	}
}

type PhaseReconcilerError struct {
	rsrc   client.Object
	reason ErrorReason
}

func (e *PhaseReconcilerError) Error() string {
	var (
		gvk       = e.rsrc.GetObjectKind().GroupVersionKind()
		name      = e.rsrc.GetName()
		namespace = e.rsrc.GetNamespace()
	)

	return fmt.Sprintf(
		"%s/%s %s/%s: %s", gvk.Group, gvk.Kind, namespace, name, e.reason,
	)
}

func (e *PhaseReconcilerError) CausedBy(reason ErrorReason) bool {
	return e.reason == reason
}
