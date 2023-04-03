package dynamiccache

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimetav1 "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	cache "k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type mapEntry struct {
	Informer cache.SharedIndexInformer
	Reader   client.Reader
	StopCh   chan struct{}
}

// usually fulfilled by meta.RESTMapper.
type restMapper interface {
	RESTMapping(gk schema.GroupKind, versions ...string) (
		*apimetav1.RESTMapping, error)
}

func NewInformerMap(
	config *rest.Config,
	scheme *runtime.Scheme,
	mapper apimetav1.RESTMapper,
	resync time.Duration,
	selectors SelectorsByGVK,
	indexers FieldIndexersByGVK,
) *InformerMap {
	return &InformerMap{
		config:    config,
		scheme:    scheme,
		mapper:    mapper,
		resync:    resync,
		selectors: selectors.forGVK,
		indexers:  indexers.forGVK,

		informers:     map[schema.GroupVersionKind]mapEntry{},
		dynamicClient: dynamic.NewForConfigOrDie(config),
	}
}

// InformerMap caches informers and enables to shut them down later.
type InformerMap struct {
	// config is used to talk to the apiserver
	config *rest.Config

	// Scheme maps runtime.Objects to GroupVersionKinds
	scheme *runtime.Scheme

	// mapper maps GroupVersionKinds to Resources
	mapper restMapper

	// resync is the base frequency the informers are resynced
	// a 10 percent jitter will be added to the resync period between informers
	// so that all informers will not send list requests simultaneously.
	resync time.Duration

	// selectors are the label or field selectors that will be added to the
	// ListWatch ListOptions.
	selectors func(gvk schema.GroupVersionKind) Selector

	// indexers are index functions that create custom field indexes on the cache.
	indexers func(gvk schema.GroupVersionKind) []FieldIndexer

	informers    map[schema.GroupVersionKind]mapEntry
	informersMux sync.RWMutex

	// dynamicClient to create new ListWatches.
	dynamicClient dynamic.Interface
}

// Get returns a informer for the given GVK.
// If no informer is registered, a new Informer will be created.
func (im *InformerMap) Get(
	ctx context.Context,
	gvk schema.GroupVersionKind,
	obj runtime.Object,
) (informer cache.SharedIndexInformer, reader client.Reader, err error) {
	// Return the informer if it is found
	var ok bool
	informer, reader, ok = func() (
		cache.SharedIndexInformer, client.Reader, bool,
	) {
		im.informersMux.RLock()
		defer im.informersMux.RUnlock()
		entry, ok := im.informers[gvk]
		return entry.Informer, entry.Reader, ok
	}()

	if !ok {
		var err error
		if informer, reader, err = im.addInformerToMap(
			ctx, gvk, obj); err != nil {
			return nil, nil, err
		}
	}

	if !informer.HasSynced() {
		// Wait for it to sync before returning the Informer so that folks don't read from a stale cache.
		if !cache.WaitForCacheSync(ctx.Done(), informer.HasSynced) {
			return nil, nil, apierrors.NewTimeoutError(fmt.Sprintf("failed waiting for %T Informer to sync", obj), 0)
		}
	}
	return
}

// Delete shuts down an informer for the given GVK, if one is registered.
func (im *InformerMap) Delete(
	_ context.Context,
	gvk schema.GroupVersionKind,
) error {
	im.informersMux.Lock()
	defer im.informersMux.Unlock()

	entry, ok := im.informers[gvk]
	if !ok {
		return nil
	}

	close(entry.StopCh)
	delete(im.informers, gvk)
	return nil
}

func (im *InformerMap) addInformerToMap(
	_ context.Context, gvk schema.GroupVersionKind, obj runtime.Object,
) (informer cache.SharedIndexInformer, reader client.Reader, err error) {
	im.informersMux.Lock()
	defer im.informersMux.Unlock()

	// Ensure we are not creating multiple informers for the same type.
	if entry, ok := im.informers[gvk]; ok {
		return entry.Informer, entry.Reader, nil
	}

	// Create a new Informer and add it to the map.
	lw, err := im.createListWatch(context.Background(), gvk)
	if err != nil {
		return nil, nil, err
	}
	ni := cache.NewSharedIndexInformer(lw, obj, resyncPeriod(im.resync)(), cache.Indexers{
		cache.NamespaceIndex: cache.MetaNamespaceIndexFunc,
	})
	for _, indexer := range im.indexers(gvk) {
		if err := indexByField(ni, indexer.Field, indexer.Indexer); err != nil {
			return nil, nil, fmt.Errorf(
				"registering field indexer for field %q: %w", indexer.Field, err)
		}
	}

	rm, err := im.mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return nil, nil, err
	}

	e := mapEntry{
		Informer: ni,
		Reader: &CacheReader{
			indexer:          ni.GetIndexer(),
			groupVersionKind: gvk,
			scopeName:        rm.Scope.Name(),
		},
		StopCh: make(chan struct{}, 1),
	}
	im.informers[gvk] = e
	go e.Informer.Run(e.StopCh)

	return e.Informer, e.Reader, nil
}

// newListWatch returns a new ListWatch object that can be used to create a SharedIndexInformer.
func (im *InformerMap) createListWatch(
	ctx context.Context, gvk schema.GroupVersionKind,
) (*cache.ListWatch, error) {
	// Kubernetes APIs work against Resources, not GroupVersionKinds.  Map the
	// groupVersionKind to the Resource API we will use.
	mapping, err := im.mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return nil, err
	}

	client := im.dynamicClient.Resource(mapping.Resource)

	return &cache.ListWatch{
		ListFunc: func(opts metav1.ListOptions) (runtime.Object, error) {
			im.selectors(gvk).ApplyToList(&opts)
			return client.List(ctx, opts)
		},
		WatchFunc: func(opts metav1.ListOptions) (watch.Interface, error) {
			im.selectors(gvk).ApplyToList(&opts)
			return client.Watch(ctx, opts)
		},
	}, nil
}

// resyncPeriod returns a function which generates a duration each time it is
// invoked; this is so that multiple controllers don't get into lock-step and all
// hammer the apiserver with list requests simultaneously.
func resyncPeriod(resync time.Duration) func() time.Duration {
	return func() time.Duration {
		// the factor will fall into [0.9, 1.1)
		factor := rand.Float64()/5.0 + 0.9 //nolint:gosec
		return time.Duration(float64(resync.Nanoseconds()) * factor)
	}
}

// stolen from controller-runtime
// sigs.k8s.io/controller-runtime@v0.12.3/pkg/cache/informer_cache.go.
func indexByField(indexer cache.SharedIndexInformer, field string, extractor client.IndexerFunc) error {
	return indexer.AddIndexers(cache.Indexers{FieldIndexName(field): indexFuncForExtractor(extractor)})
}

func indexFuncForExtractor(extractor client.IndexerFunc) func(objRaw interface{}) ([]string, error) {
	return func(objRaw interface{}) ([]string, error) {
		// TODO(directxman12): check if this is the correct type?
		obj, isObj := objRaw.(client.Object)
		if !isObj {
			//nolint:goerr113
			return nil, fmt.Errorf("object of type %T is not an Object", objRaw)
		}
		meta, err := apimetav1.Accessor(obj)
		if err != nil {
			return nil, err
		}
		ns := meta.GetNamespace()

		rawVals := extractor(obj)
		var vals []string
		if ns == "" {
			// if we're not doubling the keys for the namespaced case, just create a new slice with same length
			vals = make([]string, len(rawVals))
		} else {
			// if we need to add non-namespaced versions too, double the length
			vals = make([]string, len(rawVals)*2)
		}
		for i, rawVal := range rawVals {
			// save a namespaced variant, so that we can ask
			// "what are all the object matching a given index *in a given namespace*"
			vals[i] = KeyToNamespacedKey(ns, rawVal)
			if ns != "" {
				// if we have a namespace, also inject a special index key for listing
				// regardless of the object namespace
				vals[i+len(rawVals)] = KeyToNamespacedKey("", rawVal)
			}
		}

		return vals, nil
	}
}
