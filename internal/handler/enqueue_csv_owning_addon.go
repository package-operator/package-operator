package handler

import (
	"sync"

	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	addonsv1alpha1 "github.com/openshift/addon-operator/apis/addons/v1alpha1"
)

var _ handler.EventHandler = (*csvEventHandler)(nil)

// CSV event handler mapping CSV events to registered watching Addons.
type csvEventHandler struct {
	addonKeytoCSVKeys map[client.ObjectKey][]client.ObjectKey
	csvKeyToAddon     map[client.ObjectKey]client.ObjectKey
	mux               sync.RWMutex
}

func NewCSVEventHandler() *csvEventHandler {
	return &csvEventHandler{
		addonKeytoCSVKeys: map[client.ObjectKey][]client.ObjectKey{},
		csvKeyToAddon:     map[client.ObjectKey]client.ObjectKey{},
	}
}

// Free removes all event mappings associated with the given Addon.
func (h *csvEventHandler) Free(addon *addonsv1alpha1.Addon) {
	h.mux.Lock()
	defer h.mux.Unlock()

	addonKey := client.ObjectKeyFromObject(addon)
	for _, csvKey := range h.addonKeytoCSVKeys[addonKey] {
		delete(h.csvKeyToAddon, csvKey)
	}
	delete(h.addonKeytoCSVKeys, addonKey)
}

// ReplaceMap tells the event handler about a Addon > CSV relation and setup mapping of future events.
// It returns true when the existing mapping had to be changed.
// WARNING:
// This method is potentially racy when the Addon object is not reenqueued by the calling reconcile loop when the mapping changes,
// as incomming events might be dropped before this method completes and the event mapping is updated.
// Calling code needs to make sure to reenqueue the Addon object for _every_ mapping change or CSV events might not be processed.
func (h *csvEventHandler) ReplaceMap(
	addon *addonsv1alpha1.Addon, csvKeys ...client.ObjectKey,
) (changed bool) {
	h.mux.Lock()
	defer h.mux.Unlock()

	addonKey := client.ObjectKeyFromObject(addon)
	h.addonKeytoCSVKeys[addonKey] = csvKeys

	// Ensure all new CSV keys are present in our mapping
	// and remember all keys that we actually want.
	wantedCSVKeys := map[client.ObjectKey]struct{}{}
	for _, csvKey := range csvKeys {
		if _, ok := h.csvKeyToAddon[csvKey]; !ok {
			changed = true
		}
		h.csvKeyToAddon[csvKey] = addonKey
		wantedCSVKeys[csvKey] = struct{}{}
	}

	// Remove all keys from our mapping that we don't need.
	for csvKey := range h.csvKeyToAddon {
		if _, ok := wantedCSVKeys[csvKey]; !ok {
			delete(h.csvKeyToAddon, csvKey)
			changed = true
		}
	}

	return changed
}

// Create is called in response to an create event.
func (h *csvEventHandler) Create(evt event.CreateEvent, q workqueue.RateLimitingInterface) {
	h.enqueueObject(evt.Object, q)
}

// Update is called in response to an update event.
func (h *csvEventHandler) Update(evt event.UpdateEvent, q workqueue.RateLimitingInterface) {
	h.enqueueObject(evt.ObjectNew, q)
}

// Delete is called in response to a delete event.
func (h *csvEventHandler) Delete(evt event.DeleteEvent, q workqueue.RateLimitingInterface) {
	h.enqueueObject(evt.Object, q)
}

// Generic is called in response to an event of an unknown type or a synthetic event triggered as a cron or
// external trigger request - e.g. reconcile Autoscaling, or a Webhook.
func (h *csvEventHandler) Generic(evt event.GenericEvent, q workqueue.RateLimitingInterface) {
	h.enqueueObject(evt.Object, q)
}

func (h *csvEventHandler) enqueueObject(obj client.Object, q workqueue.RateLimitingInterface) {
	h.mux.RLock()
	defer h.mux.RUnlock()

	csvKey := client.ObjectKeyFromObject(obj)
	addonKey, ok := h.csvKeyToAddon[csvKey]
	if !ok {
		return
	}

	q.Add(reconcile.Request{NamespacedName: addonKey})
}
