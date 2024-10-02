package dynamiccache

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// Enqueues all objects watching the object mentioned in the event, filtered by WatcherType.
type EnqueueWatchingObjects struct {
	WatcherRefGetter ownerRefGetter
	// WatcherType is the type of the Owner object to look for in OwnerReferences.  Only Group and Kind are compared.
	WatcherType runtime.Object

	scheme *runtime.Scheme
	// groupKind is the cached Group and Kind from WatcherType
	groupKind schema.GroupKind
}

var _ handler.EventHandler = (*EnqueueWatchingObjects)(nil)

func NewEnqueueWatchingObjects(watcherRefGetter ownerRefGetter,
	watcherType runtime.Object,
	scheme *runtime.Scheme,
) *EnqueueWatchingObjects {
	e := &EnqueueWatchingObjects{
		WatcherRefGetter: watcherRefGetter,
		WatcherType:      watcherType,
		scheme:           scheme,
	}
	if err := e.parseWatcherTypeGroupKind(scheme); err != nil {
		// This (passing a type that is not in the scheme) HAS
		// to be a programmer error and can't be recovered at runtime anyways.
		panic(err)
	}
	return e
}

type ownerRefGetter interface {
	OwnersForGKV(gvk schema.GroupVersionKind) []OwnerReference
}

func (e *EnqueueWatchingObjects) Create(
	_ context.Context, evt event.CreateEvent,
	q workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	e.enqueueWatchers(evt.Object, q)
}

func (e *EnqueueWatchingObjects) Update(
	_ context.Context, evt event.UpdateEvent,
	q workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	e.enqueueWatchers(evt.ObjectNew, q)
	e.enqueueWatchers(evt.ObjectOld, q)
}

func (e *EnqueueWatchingObjects) Delete(
	_ context.Context, evt event.DeleteEvent,
	q workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	e.enqueueWatchers(evt.Object, q)
}

func (e *EnqueueWatchingObjects) Generic(
	_ context.Context, evt event.GenericEvent,
	q workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	e.enqueueWatchers(evt.Object, q)
}

func (e *EnqueueWatchingObjects) enqueueWatchers(
	obj client.Object,
	q workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	if obj == nil {
		return
	}

	gvk, err := apiutil.GVKForObject(obj, e.scheme)
	if err != nil {
		// TODO: error reporting?
		panic(err)
	}

	ownerRefs := e.WatcherRefGetter.OwnersForGKV(gvk)
	for _, ownerRef := range ownerRefs {
		if ownerRef.Kind != e.groupKind.Kind ||
			ownerRef.Group != e.groupKind.Group {
			continue
		}

		q.Add(reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      ownerRef.Name,
				Namespace: ownerRef.Namespace,
			},
		})
	}
}

// parseOwnerTypeGroupKind parses the WatcherType into a Group and Kind and caches the result.  Returns false
// if the WatcherType could not be parsed using the scheme.
func (e *EnqueueWatchingObjects) parseWatcherTypeGroupKind(scheme *runtime.Scheme) error {
	// Get the kinds of the type
	kinds, _, err := scheme.ObjectKinds(e.WatcherType)
	if err != nil {
		return err
	}
	// Expect only 1 kind.  If there is more than one kind this is probably an edge case such as ListOptions.
	if len(kinds) != 1 {
		panic(fmt.Sprintf("Expected exactly 1 kind for WatcherType %T, but found %s kinds", e.WatcherType, kinds))
	}
	// Cache the Group and Kind for the WatcherType
	e.groupKind = schema.GroupKind{Group: kinds[0].Group, Kind: kinds[0].Kind}
	return nil
}
