package webhooks

import (
	"context"
	"net/http"

	v1 "k8s.io/api/admission/v1"
	admissionv1beta1 "k8s.io/api/admission/v1beta1"
	corev1alpha1 "package-operator.run/apis/core/v1alpha1"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// ObjectSetPhaseWebhookHandler handles validating ObjectSetPhase objects
type ObjectSetPhaseWebhookHandler struct {
	decoder *admission.Decoder
	Log     logr.Logger
	Client  client.Client
}

var _ admission.Handler = (*ObjectSetPhaseWebhookHandler)(nil)

// Handle implements admission.Handler interface
func (r *ObjectSetPhaseWebhookHandler) Handle(_ context.Context, req admission.Request) admission.Response {
	obj, err := r.decodeObjectSetPhase(req)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	switch req.Operation {
	case v1.Operation(admissionv1beta1.Update):
		oldObj := corev1alpha1.ObjectSetPhase{}
		if err := r.decoder.DecodeRaw(req.OldObject, &oldObj); err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}
		return r.validateUpdate(&obj, &oldObj)
	default:
		return admission.Allowed("operation allowed")
	}
}

//type union interface {
//	corev1alpha1.ClusterObjectSetPhase | corev1alpha1.ObjectSetPhase | corev1alpha1.ObjectSetPhase | corev1alpha1.ObjectSetPhasePhase
//}

//func decodeObject[T union](req admission.Request, decoder *admission.Decoder) (T, error) {
//	obj := new(T)
//	if req.Operation != v1.Operation(admissionv1beta1.Delete) {
//		err := decoder.Decode(req, obj.(runtime.Object))
//		return *obj, err
//	}
//	return *obj, nil
//}

func (r *ObjectSetPhaseWebhookHandler) decodeObjectSetPhase(req admission.Request) (corev1alpha1.ObjectSetPhase, error) {
	obj := corev1alpha1.ObjectSetPhase{}
	if req.Operation != v1.Operation(admissionv1beta1.Delete) {
		err := r.decoder.Decode(req, &obj)
		return obj, err
	}
	return obj, nil
}

func (r *ObjectSetPhaseWebhookHandler) InjectDecoder(d *admission.Decoder) error {
	r.decoder = d
	return nil
}

func (r *ObjectSetPhaseWebhookHandler) validateUpdate(os, oldOs *corev1alpha1.ObjectSetPhase) admission.Response {
	if err := validateObjectSetPhaseImmutability(os, oldOs); err != nil {
		return admission.Denied(err.Error())
	}
	return admission.Allowed("operation allowed")
}
