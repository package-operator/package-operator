package webhooks

import (
	"context"
	"net/http"

	"github.com/go-logr/logr"
	v1 "k8s.io/api/admission/v1"
	admissionv1beta1 "k8s.io/api/admission/v1beta1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/runtime"
	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
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
		log:    log,
		client: client,
	}
}

func NewClusterObjectSetWebhookHandler(
	log logr.Logger,
	client client.Client,
) *GenericObjectSetWebhookHandler[corev1alpha1.ClusterObjectSet] {
	return &GenericObjectSetWebhookHandler[corev1alpha1.ClusterObjectSet]{
		log:    log,
		client: client,
	}
}

func (wh *GenericObjectSetWebhookHandler[T]) newObjectSet() *T {
	return new(T)
}

func (wh *GenericObjectSetWebhookHandler[T]) decode(req admission.Request) (*T, error) {
	obj := wh.newObjectSet()
	if req.Operation == v1.Operation(admissionv1beta1.Delete) {
		return obj, nil
	}
	if err := wh.decoder.Decode(
		req, any(obj).(client.Object)); err != nil {
		return nil, err
	}
	return obj, nil
}

func (wh *GenericObjectSetWebhookHandler[T]) Handle(
	ctx context.Context, req admission.Request) admission.Response {
	obj, err := wh.decode(req)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	switch req.Operation {
	case v1.Operation(admissionv1beta1.Update):
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

func (wh *GenericObjectSetWebhookHandler[T]) InjectDecoder(d *admission.Decoder) error {
	wh.decoder = d
	return nil
}

func (wh *GenericObjectSetWebhookHandler[T]) validateUpdate(
	obj, oldObj *T) admission.Response {
	if err := validateGenericObjectSetImmutability(obj, oldObj); err != nil {
		return admission.Denied(err.Error())
	}
	return admission.Allowed("operation allowed")
}

func validateGenericObjectSetImmutability[T objectSets](obj, oldObj *T) error {
	oldFields := objectSetImmutableFields(oldObj)
	newFields := objectSetImmutableFields(obj)

	if !equality.Semantic.DeepEqual(
		newFields.ObjectSetTemplateSpec,
		oldFields.ObjectSetTemplateSpec) {
		return errObjectSetTemplateSpecImmutable
	}

	if !equality.Semantic.DeepEqual(
		newFields.Previous, oldFields.Previous) {
		return errPreviousImmutable
	}
	return nil
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
