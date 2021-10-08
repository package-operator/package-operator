package webhooks

import (
	"context"
	"net/http"

	v1 "k8s.io/api/admission/v1"
	adminv1beta1 "k8s.io/api/admission/v1beta1"

	addonsv1alpha1 "github.com/openshift/addon-operator/apis/addons/v1alpha1"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// AddonWebhookHandler handles validating Addon objects
type AddonWebhookHandler struct {
	decoder *admission.Decoder
	Log     logr.Logger
	Client  client.Client
}

var _ admission.Handler = (*AddonWebhookHandler)(nil)

func (r *AddonWebhookHandler) Handle(ctx context.Context, req admission.Request) admission.Response {
	obj, err := r.decodeAddon(req)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	switch req.Operation {
	case v1.Operation(adminv1beta1.Create):
		return r.validateCreate(&obj)
	case v1.Operation(adminv1beta1.Update):
		oldObj := addonsv1alpha1.Addon{}
		if err := r.decoder.DecodeRaw(req.OldObject, &oldObj); err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}
		return r.validateUpdate(&obj, &oldObj)
	default:
		return admission.Allowed("operation allowed")
	}
}

func (r *AddonWebhookHandler) decodeAddon(req admission.Request) (addonsv1alpha1.Addon, error) {
	obj := addonsv1alpha1.Addon{}
	if req.Operation != v1.Operation(adminv1beta1.Delete) {
		if err := r.decoder.Decode(req, &obj); err != nil {
			return obj, err
		}
		return obj, nil
	}
	return obj, nil
}

func (r *AddonWebhookHandler) InjectDecoder(d *admission.Decoder) error {
	r.decoder = d
	return nil
}

func (r *AddonWebhookHandler) validateCreate(addon *addonsv1alpha1.Addon) admission.Response {
	if err := validateAddon(addon); err != nil {
		return admission.Denied(err.Error())
	}
	return admission.Allowed("operation allowed")
}

func (r *AddonWebhookHandler) validateUpdate(addon, oldAddon *addonsv1alpha1.Addon) admission.Response {
	if err := validateAddon(addon); err != nil {
		return admission.Denied(err.Error())
	}

	if err := validateAddonImmutability(addon, oldAddon); err != nil {
		return admission.Denied(err.Error())
	}
	return admission.Allowed("operation allowed")
}
