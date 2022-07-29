package ownerhandling

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
)

type ownerStrategy interface {
	IsOwner(owner, obj metav1.Object) bool
	ReleaseController(obj metav1.Object)
	SetControllerReference(owner, obj metav1.Object, scheme *runtime.Scheme) error
	EnqueueRequestForOwner(ownerType client.Object, isController bool) handler.EventHandler
}

var (
	Annotation = &OwnerStrategyAnnotation{}
	Native     = &OwnerStrategyNative{}
)
