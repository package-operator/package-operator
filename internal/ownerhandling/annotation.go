package ownerhandling

import (
	"encoding/json"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const ownerStrategyAnnotation = "packages.thetechnick.ninja/owners"

var _ ownerStrategy = (*OwnerStrategyAnnotation)(nil)

// NativeOwner handling strategy uses .metadata.annotations
type OwnerStrategyAnnotation struct{}

func (s *OwnerStrategyAnnotation) EnqueueRequestForOwner(
	ownerType client.Object, isController bool,
) handler.EventHandler {
	return &AnnotationEnqueueOwnerHandler{
		OwnerType:    ownerType,
		IsController: isController,
	}
}

func (s *OwnerStrategyAnnotation) SetControllerReference(owner, obj metav1.Object, scheme *runtime.Scheme) error {
	ownerRefs := s.getOwnerReferences(obj)

	// Ensure that there is only a single controller
	for _, ownerRef := range ownerRefs {
		if ownerRef.Controller != nil && *ownerRef.Controller &&
			ownerRef.UID != owner.GetUID() {
			return &controllerutil.AlreadyOwnedError{
				Object: obj,
				Owner: metav1.OwnerReference{
					APIVersion: ownerRef.APIVersion,
					Kind:       ownerRef.Kind,
					Name:       ownerRef.Name,
					Controller: ownerRef.Controller,
					UID:        ownerRef.UID,
				},
			}
		}
	}

	gvk, err := apiutil.GVKForObject(owner.(runtime.Object), scheme)
	if err != nil {
		return err
	}
	ownerRef := annotationOwnerRef{
		APIVersion: gvk.GroupVersion().String(),
		Kind:       gvk.Kind,
		UID:        owner.GetUID(),
		Name:       owner.GetName(),
		Namespace:  owner.GetNamespace(),
		Controller: pointer.BoolPtr(true),
	}

	ownerIndex := s.indexOf(ownerRefs, ownerRef)
	if ownerIndex != -1 {
		ownerRefs[ownerIndex] = ownerRef
	} else {
		ownerRefs = append(ownerRefs, ownerRef)
	}
	s.setOwnerReferences(obj, ownerRefs)

	return nil
}

func (s *OwnerStrategyAnnotation) IsOwner(owner, obj metav1.Object) bool {
	ownerRefs := s.getOwnerReferences(obj)
	for _, ownerRef := range ownerRefs {
		if ownerRef.UID == owner.GetUID() {
			return true
		}
	}
	return false
}

func (s *OwnerStrategyAnnotation) ReleaseController(obj metav1.Object) {
	ownerRefs := s.getOwnerReferences(obj)
	for _, ownerRef := range ownerRefs {
		ownerRef.Controller = nil
	}
	s.setOwnerReferences(obj, ownerRefs)
}

func (s *OwnerStrategyAnnotation) getOwnerReferences(obj metav1.Object) []annotationOwnerRef {
	annotations := obj.GetAnnotations()
	if annotations == nil {
		return nil
	}
	if len(annotations[ownerStrategyAnnotation]) == 0 {
		return nil
	}

	var ownerReferences []annotationOwnerRef
	if err := json.Unmarshal([]byte(annotations[ownerStrategyAnnotation]), &ownerReferences); err != nil {
		panic(err)
	}

	return ownerReferences
}

func (s *OwnerStrategyAnnotation) setOwnerReferences(obj metav1.Object, owners []annotationOwnerRef) {
	annotations := obj.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	j, err := json.Marshal(owners)
	if err != nil {
		panic(err)
	}
	annotations[ownerStrategyAnnotation] = string(j)
	obj.SetAnnotations(annotations)
}

func (s *OwnerStrategyAnnotation) indexOf(ownerRefs []annotationOwnerRef, ownerRef annotationOwnerRef) int {
	for i := range ownerRefs {
		if ownerRefs[i].UID == ownerRef.UID {
			return i
		}
	}
	return -1
}

type annotationOwnerRef struct {
	// API version of the referent.
	APIVersion string `json:"apiVersion"`
	// Kind of the referent.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds
	Kind string `json:"kind"`
	// Name of the referent.
	// More info: http://kubernetes.io/docs/user-guide/identifiers#names
	Name string `json:"name"`
	// Name of the referent.
	// More info: http://kubernetes.io/docs/user-guide/identifiers#namespaces
	Namespace string `json:"namespace"`
	// UID of the referent.
	// More info: http://kubernetes.io/docs/user-guide/identifiers#uids
	UID types.UID `json:"uid"`
	// If true, this reference points to the managing controller.
	// +optional
	Controller *bool `json:"controller,omitempty"`
}

type AnnotationEnqueueOwnerHandler struct {
	OwnerType    client.Object
	IsController bool
	ownerGK      schema.GroupKind
}

// Create implements EventHandler
func (e *AnnotationEnqueueOwnerHandler) Create(evt event.CreateEvent, q workqueue.RateLimitingInterface) {
	for _, req := range e.getOwnerReconcileRequest(evt.Object) {
		q.Add(req)
	}
}

// Update implements EventHandler
func (e *AnnotationEnqueueOwnerHandler) Update(evt event.UpdateEvent, q workqueue.RateLimitingInterface) {
	for _, req := range e.getOwnerReconcileRequest(evt.ObjectOld) {
		q.Add(req)
	}
	for _, req := range e.getOwnerReconcileRequest(evt.ObjectNew) {
		q.Add(req)
	}
}

// Delete implements EventHandler
func (e *AnnotationEnqueueOwnerHandler) Delete(evt event.DeleteEvent, q workqueue.RateLimitingInterface) {
	for _, req := range e.getOwnerReconcileRequest(evt.Object) {
		q.Add(req)
	}
}

// Generic implements EventHandler
func (e *AnnotationEnqueueOwnerHandler) Generic(evt event.GenericEvent, q workqueue.RateLimitingInterface) {
	for _, req := range e.getOwnerReconcileRequest(evt.Object) {
		q.Add(req)
	}
}

func (e *AnnotationEnqueueOwnerHandler) InjectScheme(s *runtime.Scheme) error {
	return e.parseOwnerTypeGroupKind(s)
}

func (e *AnnotationEnqueueOwnerHandler) getOwnerReconcileRequest(object metav1.Object) []reconcile.Request {
	var requests []reconcile.Request
	ownerReferences := Annotation.getOwnerReferences(object)
	for _, ownerRef := range ownerReferences {
		ownerRefGV, err := schema.ParseGroupVersion(ownerRef.APIVersion)
		if err != nil {
			return nil
		}

		if ownerRefGV.Group != e.ownerGK.Group ||
			ownerRef.Kind != e.ownerGK.Kind {
			continue
		}

		if e.IsController &&
			ownerRef.Controller != nil &&
			!*ownerRef.Controller {
			// only continue if ownerRef is setup as Controller
			continue
		}

		requests = append(requests, reconcile.Request{
			NamespacedName: client.ObjectKey{
				Name:      ownerRef.Name,
				Namespace: ownerRef.Namespace,
			},
		})
	}

	return requests
}

// parseOwnerTypeGroupKind parses the OwnerType into a Group and Kind and caches the result.
func (e *AnnotationEnqueueOwnerHandler) parseOwnerTypeGroupKind(scheme *runtime.Scheme) error {
	// Get the kinds of the type
	kinds, _, err := scheme.ObjectKinds(e.OwnerType)
	if err != nil {
		return err
	}
	// Expect only 1 kind.  If there is more than one kind this is probably an edge case such as ListOptions.
	if len(kinds) != 1 {
		err := fmt.Errorf("Expected exactly 1 kind for OwnerType %T, but found %s kinds", e.OwnerType, kinds)
		return err

	}
	// Cache the Group and Kind for the OwnerType
	e.ownerGK = schema.GroupKind{Group: kinds[0].Group, Kind: kinds[0].Kind}
	return nil
}
