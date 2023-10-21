package webhooks

import (
	"context"
	"net/http"

	"github.com/go-logr/logr"
	admissionv1 "k8s.io/api/admission/v1"
	admissionv1beta1 "k8s.io/api/admission/v1beta1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

type objectSetPhases interface {
	corev1alpha1.ObjectSetPhase |
		corev1alpha1.ClusterObjectSetPhase
}

type GenericObjectSetPhaseWebhookHandler[T objectSetPhases] struct {
	decoder *admission.Decoder
	log     logr.Logger
	client  client.Client
}

func NewObjectSetPhaseWebhookHandler(
	log logr.Logger,
	client client.Client,
) *GenericObjectSetPhaseWebhookHandler[corev1alpha1.ObjectSetPhase] {
	return &GenericObjectSetPhaseWebhookHandler[corev1alpha1.ObjectSetPhase]{
		decoder: admission.NewDecoder(client.Scheme()),
		log:     log,
		client:  client,
	}
}

func NewClusterObjectSetPhaseWebhookHandler(
	log logr.Logger,
	client client.Client,
) *GenericObjectSetPhaseWebhookHandler[corev1alpha1.ClusterObjectSetPhase] {
	return &GenericObjectSetPhaseWebhookHandler[corev1alpha1.ClusterObjectSetPhase]{
		decoder: admission.NewDecoder(client.Scheme()),
		log:     log,
		client:  client,
	}
}

func (wh *GenericObjectSetPhaseWebhookHandler[T]) newObjectSetPhase() *T {
	return new(T)
}

func (wh *GenericObjectSetPhaseWebhookHandler[T]) decode(req admission.Request) (*T, error) {
	obj := wh.newObjectSetPhase()
	if req.Operation == admissionv1.Operation(admissionv1beta1.Delete) {
		return obj, nil
	}
	if err := wh.decoder.Decode(
		req, any(obj).(client.Object)); err != nil {
		return nil, err
	}
	return obj, nil
}

func (wh *GenericObjectSetPhaseWebhookHandler[T]) Handle(
	_ context.Context, req admission.Request,
) admission.Response {
	obj, err := wh.decode(req)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	switch req.Operation {
	case admissionv1.Operation(admissionv1beta1.Update):
		oldObj := wh.newObjectSetPhase()
		if err := wh.decoder.DecodeRaw(
			req.OldObject, any(oldObj).(runtime.Object)); err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}
		return wh.validateUpdate(obj, oldObj)
	default:
		return admission.Allowed("operation allowed")
	}
}

func (wh *GenericObjectSetPhaseWebhookHandler[T]) validateUpdate(
	obj, oldObj *T,
) admission.Response {
	if err := validateGenericObjectSetPhaseImmutability(obj, oldObj); err != nil {
		return admission.Denied(err.Error())
	}
	return admission.Allowed("operation allowed")
}

func validateGenericObjectSetPhaseImmutability[T objectSetPhases](obj, oldObj *T) error {
	oldFields := objectSetPhaseImmutableFields(oldObj)
	newFields := objectSetPhaseImmutableFields(obj)

	var allErrs field.ErrorList
	specFields := field.NewPath("spec")
	if !equality.Semantic.DeepEqual(
		newFields.Objects,
		oldFields.Objects) {
		allErrs = append(allErrs,
			field.Invalid(specFields.Child("objects"), "", "is immutable"))
	}

	if !equality.Semantic.DeepEqual(
		newFields.Previous, oldFields.Previous) {
		allErrs = append(allErrs,
			field.Invalid(specFields.Child("previous"), "", "is immutable"))
	}

	if newFields.Revision != oldFields.Revision {
		allErrs = append(allErrs,
			field.Invalid(specFields.Child("revision"), "", "is immutable"))
	}

	if !equality.Semantic.DeepEqual(newFields.AvailabilityProbes, oldFields.AvailabilityProbes) {
		allErrs = append(allErrs,
			field.Invalid(specFields.Child("availabilityProbes"), "", "is immutable"))
	}

	if len(allErrs) == 0 {
		return nil
	}
	return allErrs.ToAggregate()
}

type genericObjectSetPhaseImmutableFields struct {
	Previous           []corev1alpha1.PreviousRevisionReference `json:"previous,omitempty"`
	Objects            []corev1alpha1.ObjectSetObject           `json:",inline"`
	Revision           int64                                    `json:"revision"`
	AvailabilityProbes []corev1alpha1.ObjectSetProbe            `json:"availabilityProbes"`
}

func objectSetPhaseImmutableFields[T objectSetPhases](obj *T) genericObjectSetPhaseImmutableFields {
	var (
		previous []corev1alpha1.PreviousRevisionReference
		objects  []corev1alpha1.ObjectSetObject
		revision int64
		probes   []corev1alpha1.ObjectSetProbe
	)

	switch v := any(obj).(type) {
	case *corev1alpha1.ClusterObjectSetPhase:
		previous = v.Spec.Previous
		objects = v.Spec.Objects
		revision = v.Spec.Revision
		probes = v.Spec.AvailabilityProbes
	case *corev1alpha1.ObjectSetPhase:
		previous = v.Spec.Previous
		objects = v.Spec.Objects
		revision = v.Spec.Revision
		probes = v.Spec.AvailabilityProbes
	}

	return genericObjectSetPhaseImmutableFields{
		Previous:           previous,
		Objects:            objects,
		Revision:           revision,
		AvailabilityProbes: probes,
	}
}
