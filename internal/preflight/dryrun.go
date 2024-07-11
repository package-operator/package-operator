package preflight

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	apimachineryerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type autoImpersonatingWriter interface {
	client.Writer
	Impersonate()
}

type DryRun struct {
	writer autoImpersonatingWriter
}

func NewDryRun(writer autoImpersonatingWriter) *DryRun {
	return &DryRun{
		writer: writer,
	}
}

func (p *DryRun) Check(ctx context.Context, _, obj client.Object) (violations []Violation, err error) {
	defer addPositionToViolations(ctx, obj, &violations)
	obj = obj.DeepCopyObject().(client.Object)

	// Pretend any new owner is not marked as Controller to get around
	// preflight issues when adoption objects from other Controllers.
	ownerRefs := make([]metav1.OwnerReference, len(obj.GetOwnerReferences()))
	for i, ownerRef := range obj.GetOwnerReferences() {
		ownerRef.Controller = nil
		ownerRefs[i] = ownerRef
	}
	obj.SetOwnerReferences(ownerRefs)

	objectPatch, mErr := json.Marshal(obj)
	if mErr != nil {
		return []Violation{{Error: fmt.Errorf("creating patch: %w", mErr).Error()}}, nil
	}

	patch := client.RawPatch(types.ApplyPatchType, objectPatch)
	dst := obj.DeepCopyObject().(*unstructured.Unstructured)
	err = p.writer.Patch(ctx, dst, patch, client.FieldOwner("package-operator"), client.ForceOwnership, client.DryRunAll)

	var apiErr apimachineryerrors.APIStatus

	switch {
	case err == nil:
		return
	case errors.As(err, &apiErr):
		switch apiErr.Status().Reason {
		case metav1.StatusReasonUnauthorized,
			metav1.StatusReasonForbidden,
			metav1.StatusReasonAlreadyExists,
			metav1.StatusReasonConflict,
			metav1.StatusReasonInvalid,
			metav1.StatusReasonBadRequest,
			metav1.StatusReasonMethodNotAllowed,
			metav1.StatusReasonRequestEntityTooLarge,
			metav1.StatusReasonUnsupportedMediaType,
			metav1.StatusReasonNotAcceptable,
			metav1.StatusReasonNotFound:
			return []Violation{{Error: err.Error(), Reason: apiErr.Status().Reason}}, nil
		case "":
			logr.FromContextOrDiscard(ctx).Info("API status error with empty reason string", "err", apiErr.Status())

			if strings.Contains(apiErr.Status().Message, "failed to create typed patch object") {
				return []Violation{{Error: err.Error()}}, nil
			}
		}
	}

	return
}
