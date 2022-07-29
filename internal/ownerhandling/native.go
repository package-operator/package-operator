package ownerhandling

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
)

var _ ownerStrategy = (*OwnerStrategyNative)(nil)

// NativeOwner handling strategy uses .metadata.ownerReferences
type OwnerStrategyNative struct{}

func (s *OwnerStrategyNative) IsOwner(owner, obj metav1.Object) bool {
	for _, ownerRef := range obj.GetOwnerReferences() {
		if owner.GetUID() == ownerRef.UID {
			return true
		}
	}
	return false
}

func (s *OwnerStrategyNative) ReleaseController(obj metav1.Object) {
	ownerRefs := obj.GetOwnerReferences()
	for _, ownerRef := range ownerRefs {
		ownerRef.Controller = nil
	}
	obj.SetOwnerReferences(ownerRefs)
}

func (s *OwnerStrategyNative) SetControllerReference(owner, obj metav1.Object, scheme *runtime.Scheme) error {
	return controllerutil.SetControllerReference(owner, obj, scheme)
}

func (s *OwnerStrategyNative) EnqueueRequestForOwner(
	ownerType client.Object, isController bool,
) handler.EventHandler {
	return &handler.EnqueueRequestForOwner{
		OwnerType:    ownerType,
		IsController: isController,
	}
}
