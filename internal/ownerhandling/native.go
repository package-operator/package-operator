package ownerhandling

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
)

var _ ownerStrategy = (*OwnerStrategyNative)(nil)

// NativeOwner handling strategy uses .metadata.ownerReferences.
type OwnerStrategyNative struct {
	scheme *runtime.Scheme
}

func NewNative(scheme *runtime.Scheme) *OwnerStrategyNative {
	return &OwnerStrategyNative{
		scheme: scheme,
	}
}

func (s *OwnerStrategyNative) IsOwner(owner, obj metav1.Object) bool {
	ownerRefComp := s.ownerRefForCompare(owner)
	for _, ownerRef := range obj.GetOwnerReferences() {
		if s.referSameObject(ownerRefComp, ownerRef) {
			return true
		}
	}
	return false
}

func (s *OwnerStrategyNative) IsController(
	owner, obj metav1.Object,
) bool {
	ownerRefComp := s.ownerRefForCompare(owner)
	for _, ownerRef := range obj.GetOwnerReferences() {
		if s.referSameObject(ownerRefComp, ownerRef) &&
			ownerRef.Controller != nil &&
			*ownerRef.Controller {
			return true
		}
	}
	return false
}

func (s *OwnerStrategyNative) RemoveOwner(owner, obj metav1.Object) {
	ownerRefComp := s.ownerRefForCompare(owner)
	ownerRefs := obj.GetOwnerReferences()
	foundIndex := -1
	for i, ownerRef := range ownerRefs {
		if s.referSameObject(ownerRefComp, ownerRef) {
			foundIndex = i
			break
		}
	}
	if foundIndex != -1 {
		obj.SetOwnerReferences(remove(ownerRefs, foundIndex))
	}
}

func (s *OwnerStrategyNative) ReleaseController(obj metav1.Object) {
	ownerRefs := obj.GetOwnerReferences()
	for i := range ownerRefs {
		ownerRefs[i].Controller = nil
	}
	obj.SetOwnerReferences(ownerRefs)
}

func (s *OwnerStrategyNative) SetControllerReference(owner, obj metav1.Object) error {
	return controllerutil.SetControllerReference(owner, obj, s.scheme)
}

func (s *OwnerStrategyNative) EnqueueRequestForOwner(
	ownerType client.Object, isController bool,
) handler.EventHandler {
	return &handler.EnqueueRequestForOwner{
		OwnerType:    ownerType,
		IsController: isController,
	}
}

func (s *OwnerStrategyNative) ownerRefForCompare(owner metav1.Object) metav1.OwnerReference {
	// Validate the owner.
	ro, ok := owner.(runtime.Object)
	if !ok {
		panic(fmt.Sprintf("%T is not a runtime.Object, cannot call SetOwnerReference", owner))
	}

	// Create a new owner ref.
	gvk, err := apiutil.GVKForObject(ro, s.scheme)
	if err != nil {
		panic(err)
	}
	ref := metav1.OwnerReference{
		APIVersion: gvk.GroupVersion().String(),
		Kind:       gvk.Kind,
		UID:        owner.GetUID(),
		Name:       owner.GetName(),
	}
	return ref
}

// Returns true if a and b point to the same object.
func (s *OwnerStrategyNative) referSameObject(a, b metav1.OwnerReference) bool {
	aGV, err := schema.ParseGroupVersion(a.APIVersion)
	if err != nil {
		panic(err)
	}

	bGV, err := schema.ParseGroupVersion(b.APIVersion)
	if err != nil {
		panic(err)
	}

	return aGV.Group == bGV.Group && a.Kind == b.Kind && a.Name == b.Name
}
