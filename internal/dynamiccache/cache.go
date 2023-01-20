package dynamiccache

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"
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

type cacheSourcer interface {
	source.Source
	blockNewRegistrations()
	handleNewInformer(cache.SharedIndexInformer) error
}

var (
	_ client.Reader    = (*Cache)(nil)
	_ manager.Runnable = (*Cache)(nil)
)

type Cache struct {
	scheme      *runtime.Scheme
	opts        CacheOptions
	informerMap informerMap

	informerReferencesMux sync.RWMutex
	informerReferences    map[schema.GroupVersionKind]map[OwnerReference]struct{}

	recorder metricsRecorder

	cacheSource cacheSourcer
}

type metricsRecorder interface {
	RecordDynamicCacheInformers(total int)
	RecordDynamicCacheObjects(gvk schema.GroupVersionKind, count int)
}

func NewCache(
	config *rest.Config,
	scheme *runtime.Scheme,
	mapper meta.RESTMapper,
	recorder metricsRecorder,
	opts ...CacheOption,
) *Cache {
	c := &Cache{
		scheme:             scheme,
		informerReferences: map[schema.GroupVersionKind]map[OwnerReference]struct{}{},
		cacheSource:        &cacheSource{},
		recorder:           recorder,
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

func (c *Cache) Source() source.Source {
	return c.cacheSource
}

// Start implements manager.Runnable.
// While this cache is not running workers itself,
// we use it to block registration of new event handlers in the cache source.
func (c *Cache) Start(context.Context) error {
	c.cacheSource.blockNewRegistrations()
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
	defer c.sampleMetrics(ctx)

	log := logr.FromContextOrDiscard(ctx)

	gvk, err := apiutil.GVKForObject(obj, c.scheme)
	if err != nil {
		return fmt.Errorf("get GVK for object: %w", err)
	}
	ownerRef, err := c.ownerRef(owner)
	if err != nil {
		return err
	}

	// Remember Owner watching this GVK
	_, informerExists := c.informerReferences[gvk]
	if !informerExists {
		c.informerReferences[gvk] = map[OwnerReference]struct{}{}
	}
	c.informerReferences[gvk][ownerRef] = struct{}{}

	if !informerExists {
		log.Info("adding new watcher",
			"ownerGV", ownerRef.GroupKind,
			"forGVK", gvk.String(),
			"ownerNamespace", owner.GetNamespace())

		// Create/Get Informer
		informer, _, err := c.informerMap.Get(ctx, gvk, obj)
		if err != nil {
			return fmt.Errorf("getting informer from InformerMap: %w", err)
		}

		// ensure to add all event handlers to the new informer
		if err := c.cacheSource.handleNewInformer(informer); err != nil {
			return fmt.Errorf("registering EventHandlers for %v: %w", gvk, err)
		}
	}

	return nil
}

// Free all watches associated with the given owner.
func (c *Cache) Free(
	ctx context.Context, owner client.Object,
) error {
	c.informerReferencesMux.Lock()
	defer c.informerReferencesMux.Unlock()
	defer c.sampleMetrics(ctx)

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

func (CacheNotStartedError) Error() string {
	return "cache access before calling Watch, can not read objects"
}

// Get implements client.Reader.
func (c *Cache) Get(
	ctx context.Context,
	key client.ObjectKey, out client.Object,
	opts ...client.GetOption,
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

	return reader.Get(ctx, key, out, opts...)
}

// List implements client.Reader.
func (c *Cache) List(
	ctx context.Context,
	out client.ObjectList, opts ...client.ListOption,
) error {
	// Ensure we are not allocating a new cache implicitly here
	// And that the cache is not deleted while the list call is still in-flight.
	c.informerReferencesMux.RLock()
	defer c.informerReferencesMux.RUnlock()

	return c.list(ctx, out, opts...)
}

func (c *Cache) list(
	ctx context.Context,
	out client.ObjectList, opts ...client.ListOption,
) error {
	gvk, err := apiutil.GVKForObject(out, c.scheme)
	if err != nil {
		return err
	}
	gvk.Kind = strings.TrimSuffix(gvk.Kind, "List")

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

func (c *Cache) sampleMetrics(ctx context.Context) {
	if c.recorder == nil {
		return
	}

	log := logr.FromContextOrDiscard(ctx)

	informerCount := len(c.informerReferences)
	c.recorder.RecordDynamicCacheInformers(informerCount)

	for gvk := range c.informerReferences {
		listObj := &unstructured.UnstructuredList{}
		listObj.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   gvk.Group,
			Version: gvk.Version,
			Kind:    gvk.Kind + "List",
		})
		if err := c.list(ctx, listObj); err != nil {
			log.Error(err, fmt.Sprintf("listing %v to record metrics", gvk))
			continue
		}
		c.recorder.RecordDynamicCacheObjects(gvk, len(listObj.Items))
	}
}
