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
		log:    log,
		client: client,
	}
}

func NewClusterObjectSetPhaseWebhookHandler(
	log logr.Logger,
	client client.Client,
) *GenericObjectSetPhaseWebhookHandler[corev1alpha1.ClusterObjectSetPhase] {
	return &GenericObjectSetPhaseWebhookHandler[corev1alpha1.ClusterObjectSetPhase]{
		log:    log,
		client: client,
	}
}

func (wh *GenericObjectSetPhaseWebhookHandler[T]) newObjectSetPhase() *T {
	return new(T)
}

func (wh *GenericObjectSetPhaseWebhookHandler[T]) decode(req admission.Request) (*T, error) {
	obj := wh.newObjectSetPhase()
	if req.Operation == v1.Operation(admissionv1beta1.Delete) {
		return obj, nil
	}
	if err := wh.decoder.Decode(
		req, any(obj).(client.Object)); err != nil {
		return nil, err
	}
	return obj, nil
}

func (wh *GenericObjectSetPhaseWebhookHandler[T]) Handle(
	ctx context.Context, req admission.Request) admission.Response {
	obj, err := wh.decode(req)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	switch req.Operation {
	case v1.Operation(admissionv1beta1.Update):
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

func (wh *GenericObjectSetPhaseWebhookHandler[T]) InjectDecoder(d *admission.Decoder) error {
	wh.decoder = d
	return nil
}

func (wh *GenericObjectSetPhaseWebhookHandler[T]) validateUpdate(
	obj, oldObj *T) admission.Response {
	if err := validateGenericObjectSetPhaseImmutability(obj, oldObj); err != nil {
		return admission.Denied(err.Error())
	}
	return admission.Allowed("operation allowed")
}

func validateGenericObjectSetPhaseImmutability[T objectSetPhases](obj, oldObj *T) error {
	oldFields := objectSetPhaseImmutableFields(oldObj)
	newFields := objectSetPhaseImmutableFields(obj)

	if !equality.Semantic.DeepEqual(
		newFields.ObjectSetTemplatePhase,
		oldFields.ObjectSetTemplatePhase) {
		return errObjectSetTemplatePhaseImmutable
	}

	if !equality.Semantic.DeepEqual(
		newFields.Previous, oldFields.Previous) {
		return errPreviousImmutable
	}

	if newFields.Revision != oldFields.Revision {
		return errRevisionImmutable
	}

	if !equality.Semantic.DeepEqual(newFields.AvailabilityProbes, oldFields.AvailabilityProbes) {
		return errAvailabilityProbesImmutable
	}

	return nil
}

type genericObjectSetPhaseImmutableFields struct {
	Previous                            []corev1alpha1.PreviousRevisionReference `json:"previous,omitempty"`
	corev1alpha1.ObjectSetTemplatePhase `json:",inline"`
	Revision                            int64                         // TODO: tag
	AvailabilityProbes                  []corev1alpha1.ObjectSetProbe // TODO: tag
}

func objectSetPhaseImmutableFields[T objectSetPhases](obj *T) genericObjectSetPhaseImmutableFields {
	var (
		previous []corev1alpha1.PreviousRevisionReference
		template *corev1alpha1.ObjectSetTemplatePhase
		revision int64
		probes   []corev1alpha1.ObjectSetProbe
	)

	switch v := any(obj).(type) {
	case *corev1alpha1.ClusterObjectSetPhase:
		previous = v.Spec.Previous
		template = &v.Spec.ObjectSetTemplatePhase
		revision = v.Spec.Revision
		probes = v.Spec.AvailabilityProbes
	case *corev1alpha1.ObjectSetPhase:
		previous = v.Spec.Previous
		template = &v.Spec.ObjectSetTemplatePhase
		revision = v.Spec.Revision
		probes = v.Spec.AvailabilityProbes
	}

	return genericObjectSetPhaseImmutableFields{
		Previous:               previous,
		ObjectSetTemplatePhase: *template,
		Revision:               revision,
		AvailabilityProbes:     probes,
	}
}
