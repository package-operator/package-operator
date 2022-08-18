package dynamiccache

import (
	"context"
	"fmt"
	"sync"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// OwnerReference points to a single owner of a watch operation.
type OwnerReference struct {
	schema.GroupKind
	UID       types.UID
	Name      string
	Namespace string
}

type informerMap interface {
	Get(
		ctx context.Context,
		gvk schema.GroupVersionKind,
		obj runtime.Object,
	) (informer cache.SharedIndexInformer, reader client.Reader, err error)
	Delete(
		ctx context.Context,
		gvk schema.GroupVersionKind,
	) error
}

type eventHandler struct {
	ctx        context.Context
	handler    handler.EventHandler
	queue      workqueue.RateLimitingInterface
	predicates []predicate.Predicate
}

var (
	_ source.Source = (*Cache)(nil)
	_ client.Reader = (*Cache)(nil)
)

type Cache struct {
	scheme      *runtime.Scheme
	opts        CacheOptions
	informerMap informerMap

	informerReferencesMux sync.RWMutex
	informerReferences    map[schema.GroupVersionKind]map[OwnerReference]struct{}

	eventHandlersMux sync.Mutex
	eventHandlers    []eventHandler
}

func NewCache(
	config *rest.Config,
	scheme *runtime.Scheme,
	mapper meta.RESTMapper,
	opts ...CacheOption,
) *Cache {
	c := &Cache{
		scheme:             scheme,
		informerReferences: map[schema.GroupVersionKind]map[OwnerReference]struct{}{},
	}
	for _, opt := range opts {
		opt.ApplyToCacheOptions(&c.opts)
	}
	c.opts.Default()

	c.informerMap = NewInformerMap(
		config, scheme, mapper,
		c.opts.ResyncInterval, c.opts.Selectors, c.opts.Indexers)

	return c
}

// For printing in startup log messages.
func (c *Cache) String() string {
	return "dynamiccache.Cache"
}

// Implements source.Source interface to be used as event source when setting up controllers.
// All event handlers must be added before the first Watch is added dynamically.
func (c *Cache) Start(
	ctx context.Context,
	handler handler.EventHandler,
	queue workqueue.RateLimitingInterface,
	predicates ...predicate.Predicate,
) error {
	c.eventHandlersMux.Lock()
	defer c.eventHandlersMux.Unlock()
	c.eventHandlers = append(c.eventHandlers, eventHandler{
		ctx:        ctx,
		handler:    handler,
		queue:      queue,
		predicates: predicates,
	})
	return nil
}

// Returns all owners registered watching the given GroupVersionKind.
func (c *Cache) OwnersForGKV(gvk schema.GroupVersionKind) []OwnerReference {
	c.informerReferencesMux.RLock()
	defer c.informerReferencesMux.RUnlock()

	refs, ok := c.informerReferences[gvk]
	if !ok {
		return nil
	}

	ownerRefs := make([]OwnerReference, len(refs))
	var i int
	for ownerRef := range refs {
		ownerRefs[i] = ownerRef
		i++
	}
	return ownerRefs
}

// Watch the given object type and associate the watch with the given owner.
func (c *Cache) Watch(
	ctx context.Context, owner client.Object, obj runtime.Object,
) error {
	c.informerReferencesMux.Lock()
	defer c.informerReferencesMux.Unlock()

	log := logr.FromContextOrDiscard(ctx)

	gvk, err := apiutil.GVKForObject(obj, c.scheme)
	if err != nil {
		return fmt.Errorf("get GVK for object: %w", err)
	}
	ownerRef, err := c.ownerRef(owner)
	if err != nil {
		return err
	}

	// Create/Get Informer
	informer, _, err := c.informerMap.Get(ctx, gvk, obj)
	if err != nil {
		return fmt.Errorf("getting informer from InformerMap: %w", err)
	}

	if _, ok := c.informerReferences[gvk]; !ok {
		log.Info("adding new watcher",
			"ownerGV", ownerRef.GroupKind,
			"forGVK", gvk.String(),
			"ownerNamespace", owner.GetNamespace())

		// Remember GVK
		c.informerReferences[gvk] = map[OwnerReference]struct{}{}

		// ensure to add all event handlers to the new informer
		s := source.Informer{Informer: informer}
		for _, eh := range c.eventHandlers {
			if err := s.Start(eh.ctx, eh.handler, eh.queue, eh.predicates...); err != nil {
				return fmt.Errorf("starting EventHandler for %v: %w", gvk, err)
			}
		}
	}

	// Remember Owner watching this GVK
	c.informerReferences[gvk][ownerRef] = struct{}{}

	return nil
}

// Free all watches associated with the given owner.
func (c *Cache) Free(
	ctx context.Context, owner client.Object,
) error {
	c.informerReferencesMux.Lock()
	defer c.informerReferencesMux.Unlock()

	log := logr.FromContextOrDiscard(ctx)

	ownerRef, err := c.ownerRef(owner)
	if err != nil {
		return err
	}

	for gvk, refs := range c.informerReferences {
		if _, ok := refs[ownerRef]; ok {
			delete(refs, ownerRef)

			if len(refs) == 0 {
				log.Info("releasing watcher",
					"kind", gvk.Kind, "group", gvk.Group,
					"ownerNamespace", owner.GetNamespace())

				if err := c.informerMap.Delete(ctx, gvk); err != nil {
					return fmt.Errorf("releasing informer for %v: %w", gvk, err)
				}

				delete(c.informerReferences, gvk)
			}
		}
	}
	return nil
}

// CacheNotStartedError is returned when trying to read from a cache before starting a watch.
type CacheNotStartedError struct{}

func (*CacheNotStartedError) Error() string {
	return "cache access before calling Watch, can not read objects"
}

// Get implements client.Reader.
func (c *Cache) Get(
	ctx context.Context,
	key client.ObjectKey, out client.Object,
) error {
	gvk, err := apiutil.GVKForObject(out, c.scheme)
	if err != nil {
		return err
	}

	// Ensure we are not allocating a new cache implicitly here
	// And that the cache is not deleted while the get call is still in-flight.
	c.informerReferencesMux.RLock()
	defer c.informerReferencesMux.RUnlock()
	if _, ok := c.informerReferences[gvk]; !ok {
		return &CacheNotStartedError{}
	}

	_, reader, err := c.informerMap.Get(ctx, gvk, out)
	if err != nil {
		return fmt.Errorf("getting Informer from Map: %w", err)
	}

	return reader.Get(ctx, key, out)
}

// List implements client.Reader.
func (c *Cache) List(
	ctx context.Context,
	out client.ObjectList, opts ...client.ListOption,
) error {
	gvk, err := apiutil.GVKForObject(out, c.scheme)
	if err != nil {
		return err
	}

	// Ensure we are not allocating a new cache implicitly here
	// And that the cache is not deleted while the list call is still in-flight.
	c.informerReferencesMux.RLock()
	defer c.informerReferencesMux.RUnlock()
	if _, ok := c.informerReferences[gvk]; !ok {
		return &CacheNotStartedError{}
	}

	_, reader, err := c.informerMap.Get(ctx, gvk, out)
	if err != nil {
		return fmt.Errorf("getting Informer from Map: %w", err)
	}

	return reader.List(ctx, out, opts...)
}

func (c *Cache) ownerRef(owner client.Object) (OwnerReference, error) {
	ownerGVK, err := apiutil.GVKForObject(owner, c.scheme)
	if err != nil {
		return OwnerReference{}, fmt.Errorf("get GVK for object: %w", err)
	}

	return OwnerReference{
		GroupKind: schema.GroupKind{
			Group: ownerGVK.Group,
			Kind:  ownerGVK.Kind,
		},

		UID:       owner.GetUID(),
		Name:      owner.GetName(),
		Namespace: owner.GetNamespace(),
	}, nil
}
