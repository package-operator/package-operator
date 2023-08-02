package ownerhandling

import (
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
)

type ownerStrategy interface {
	IsOwner(owner, obj metav1.Object) bool
	IsController(owner, obj metav1.Object) bool
	ReleaseController(obj metav1.Object)
	RemoveOwner(owner, obj metav1.Object)
	SetControllerReference(owner, obj metav1.Object) error
	EnqueueRequestForOwner(ownerType client.Object, mapper meta.RESTMapper, isController bool) handler.EventHandler
	OwnerPatch(owner metav1.Object) ([]byte, error)
}

// Removes the given index from the slice.
// does not perform an out-of-bounds check.
func remove[T any](s []T, i int) []T {
	s[i] = s[len(s)-1]
	return s[:len(s)-1]
}
