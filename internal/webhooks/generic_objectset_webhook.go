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

type objectSets interface {
	corev1alpha1.ObjectSet |
		corev1alpha1.ClusterObjectSet
}

type GenericObjectSetWebhookHandler[T objectSets] struct {
	decoder *admission.Decoder
	log     logr.Logger
	client  client.Client
}

func NewObjectSetWebhookHandler(
	log logr.Logger,
	client client.Client,
) *GenericObjectSetWebhookHandler[corev1alpha1.ObjectSet] {
	return &GenericObjectSetWebhookHandler[corev1alpha1.ObjectSet]{
		decoder: admission.NewDecoder(client.Scheme()),
		log:     log,
		client:  client,
	}
}

func NewClusterObjectSetWebhookHandler(
	log logr.Logger,
	client client.Client,
) *GenericObjectSetWebhookHandler[corev1alpha1.ClusterObjectSet] {
	return &GenericObjectSetWebhookHandler[corev1alpha1.ClusterObjectSet]{
		decoder: admission.NewDecoder(client.Scheme()),
		log:     log,
		client:  client,
	}
}

func (wh *GenericObjectSetWebhookHandler[T]) newObjectSet() *T {
	return new(T)
}

func (wh *GenericObjectSetWebhookHandler[T]) decode(req admission.Request) (*T, error) {
	obj := wh.newObjectSet()
	if req.Operation == admissionv1.Operation(admissionv1beta1.Delete) {
		return obj, nil
	}
	if err := wh.decoder.Decode(
		req, any(obj).(client.Object)); err != nil {
		return nil, err
	}
	return obj, nil
}

func (wh *GenericObjectSetWebhookHandler[T]) Handle(_ context.Context, req admission.Request) admission.Response {
	obj, err := wh.decode(req)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	switch req.Operation {
	case admissionv1.Operation(admissionv1beta1.Update):
		oldObj := wh.newObjectSet()
		if err := wh.decoder.DecodeRaw(
			req.OldObject, any(oldObj).(runtime.Object)); err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}
		return wh.validateUpdate(obj, oldObj)
	default:
		return admission.Allowed("operation allowed")
	}
}

func (wh *GenericObjectSetWebhookHandler[T]) validateUpdate(
	obj, oldObj *T,
) admission.Response {
	if err := validateGenericObjectSetImmutability(obj, oldObj); err != nil {
		return admission.Denied(err.Error())
	}
	return admission.Allowed("operation allowed")
}

func validateGenericObjectSetImmutability[T objectSets](obj, oldObj *T) error {
	oldFields := objectSetImmutableFields(oldObj)
	newFields := objectSetImmutableFields(obj)

	var allErrs field.ErrorList

	specFields := field.NewPath("spec")
	if !equality.Semantic.DeepEqual(
		newFields.ObjectSetTemplateSpec,
		oldFields.ObjectSetTemplateSpec) {
		allErrs = append(allErrs,
			field.Invalid(specFields.Child("phases"), "", "is immutable"))
		allErrs = append(allErrs,
			field.Invalid(specFields.Child("availabilityProbes"), "", "is immutable"))
	}

	if !equality.Semantic.DeepEqual(
		newFields.Previous, oldFields.Previous) {
		allErrs = append(allErrs,
			field.Invalid(specFields.Child("previous"), "", "is immutable"))
	}

	if len(allErrs) == 0 {
		return nil
	}
	return allErrs.ToAggregate()
}

type genericImmutableFields struct {
	Previous                           []corev1alpha1.PreviousRevisionReference `json:"previous,omitempty"`
	corev1alpha1.ObjectSetTemplateSpec `json:",inline"`
}

func objectSetImmutableFields[T objectSets](obj *T) genericImmutableFields {
	var (
		previous []corev1alpha1.PreviousRevisionReference
		template *corev1alpha1.ObjectSetTemplateSpec
	)

	switch v := any(obj).(type) {
	case *corev1alpha1.ClusterObjectSet:
		previous = v.Spec.Previous
		template = &v.Spec.ObjectSetTemplateSpec
	case *corev1alpha1.ObjectSet:
		previous = v.Spec.Previous
		template = &v.Spec.ObjectSetTemplateSpec
	}

	return genericImmutableFields{
		Previous:              previous,
		ObjectSetTemplateSpec: *template,
	}
}
