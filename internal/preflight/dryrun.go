package preflight

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	k8serrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type DryRun struct {
	client client.Writer
}

func NewDryRun(client client.Writer) *DryRun { return &DryRun{client: client} }

func (p *DryRun) Check(ctx context.Context, _, obj client.Object) (violations []Violation, err error) {
	defer addPositionToViolations(ctx, obj, &violations)

	objectPatch, mErr := json.Marshal(obj)
	if mErr != nil {
		return []Violation{{Error: fmt.Errorf("creating patch: %w", mErr).Error()}}, nil
	}

	patch := client.RawPatch(types.ApplyPatchType, objectPatch)
	dst := obj.DeepCopyObject().(*unstructured.Unstructured)
	err = p.client.Patch(ctx, dst, patch, client.FieldOwner("package-operator"), client.ForceOwnership, client.DryRunAll)

	if k8serrs.IsNotFound(err) {
		err = p.client.Create(ctx, obj.DeepCopyObject().(client.Object), client.DryRunAll)
	}

	var apiErr k8serrs.APIStatus

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
			metav1.StatusReasonNotAcceptable:
			return []Violation{{Error: err.Error()}}, nil
		case "":
			logr.FromContextOrDiscard(ctx).Info("API status error with empty reason string", "err", apiErr.Status())

			if strings.Contains(apiErr.Status().Message, "failed to create typed patch object") {
				return []Violation{{Error: err.Error()}}, nil
			}
		}
	}

	return
}
