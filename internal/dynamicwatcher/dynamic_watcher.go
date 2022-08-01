package dynamicwatcher

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	ctrlcache "sigs.k8s.io/controller-runtime/pkg/cache"
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

// Namespaced GroupVersionKind.
type NamespacedGKV struct {
	schema.GroupVersionKind
	Namespace string
}

// usually fulfilled by meta.RESTMapper.
type restMapper interface {
	RESTMapping(gk schema.GroupKind, versions ...string) (
		*meta.RESTMapping, error)
}

// DynamicWatcher is able to dynamically allocate new watches for arbitrary objects.
// Multiple watches to the same resource, will be de-duplicated.
type DynamicWatcher struct {
	log        logr.Logger
	scheme     *runtime.Scheme
	restMapper restMapper
	client     dynamic.Interface

	opts Options

	sinksLock sync.RWMutex
	sinks     []watchSink

	informersLock      sync.Mutex
	informersStopCh    map[NamespacedGKV]chan<- struct{}
	informerReferences map[NamespacedGKV]map[OwnerReference]struct{}
}

const defaultEventHandlerResyncPeriod = 10 * time.Hour

type informer interface {
	ctrlcache.Informer
	Run(stopCh <-chan struct{})
}

type NewInformerFunc func(
	lw cache.ListerWatcher, exampleObject runtime.Object,
	defaultEventHandlerResyncPeriod time.Duration,
	indexers cache.Indexers,
) informer

type Options struct {
	EventHandlerResyncPeriod time.Duration
	NewInformer              NewInformerFunc
}

func (opts *Options) Default() {
	opts.EventHandlerResyncPeriod = defaultEventHandlerResyncPeriod
	opts.NewInformer = func(
		lw cache.ListerWatcher, exampleObject runtime.Object,
		defaultEventHandlerResyncPeriod time.Duration, indexers cache.Indexers,
	) informer {
		return cache.NewSharedIndexInformer(lw, exampleObject, defaultEventHandlerResyncPeriod, indexers)
	}
}

type Option interface {
	ApplyToOptions(opts *Options)
}

type EventHandlerNewInformer NewInformerFunc

func (ii EventHandlerNewInformer) ApplyToOptions(opts *Options) {
	opts.NewInformer = NewInformerFunc(ii)
}

type EventHandlerResyncPeriod time.Duration

func (rp EventHandlerResyncPeriod) ApplyToOptions(opts *Options) {
	opts.EventHandlerResyncPeriod = time.Duration(rp)
}

var _ source.Source = (*DynamicWatcher)(nil)

type watchSink struct {
	ctx        context.Context
	handler    handler.EventHandler
	queue      workqueue.RateLimitingInterface
	predicates []predicate.Predicate
}

// Creates a new DynamicWatcher instance.
func New(
	log logr.Logger, scheme *runtime.Scheme,
	restMapper restMapper, client dynamic.Interface,
	opts ...Option,
) *DynamicWatcher {
	dw := &DynamicWatcher{
		log:        log,
		scheme:     scheme,
		restMapper: restMapper,
		client:     client,

		informersStopCh:    map[NamespacedGKV]chan<- struct{}{},
		informerReferences: map[NamespacedGKV]map[OwnerReference]struct{}{},
	}

	dw.opts.Default()
	for _, opt := range opts {
		opt.ApplyToOptions(&dw.opts)
	}

	return dw
}

// For printing in startup log messages.
func (dw *DynamicWatcher) String() string {
	return "DynamicWatcher"
}

// Returns all owners registered watching the given GroupVersionKind.
// If namespace is set, only owners watching the same namespace will be returned.
func (dw *DynamicWatcher) OwnersForNamespacedGKV(ngvk NamespacedGKV) []OwnerReference {
	dw.informersLock.Lock()
	defer dw.informersLock.Unlock()

	refs, ok := dw.informerReferences[ngvk]
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

// Starts this event source.
func (dw *DynamicWatcher) Start(
	ctx context.Context,
	handler handler.EventHandler,
	queue workqueue.RateLimitingInterface,
	predicates ...predicate.Predicate,
) error {
	dw.sinksLock.Lock()
	defer dw.sinksLock.Unlock()
	dw.sinks = append(dw.sinks, watchSink{
		ctx:        ctx,
		handler:    handler,
		queue:      queue,
		predicates: predicates,
	})
	return nil
}

// Watch the given object type and associate the watch with the given owner.
// If the owner has a Namespace set, the watch will be allocated namespaced as well.
func (dw *DynamicWatcher) Watch(owner client.Object, obj runtime.Object) error {
	dw.informersLock.Lock()
	defer dw.informersLock.Unlock()

	gvk, err := apiutil.GVKForObject(obj, dw.scheme)
	if err != nil {
		return fmt.Errorf("get GVK for object: %w", err)
	}
	ngvk := NamespacedGKV{
		Namespace:        owner.GetNamespace(),
		GroupVersionKind: gvk,
	}
	ownerRef, err := dw.ownerRef(owner)
	if err != nil {
		return err
	}

	// Check if informer is already registered.
	if _, ok := dw.informersStopCh[ngvk]; !ok {
		dw.informerReferences[ngvk] = map[OwnerReference]struct{}{}
	}
	dw.informerReferences[ngvk][ownerRef] = struct{}{}
	if _, ok := dw.informersStopCh[ngvk]; ok {
		dw.log.Info(
			"reusing existing watcher",
			"owner", schema.GroupKind{Group: ownerRef.Group, Kind: ownerRef.Kind},
			"for", schema.GroupKind{Group: gvk.Group, Kind: gvk.Kind}, "namespace", owner.GetNamespace())
		return nil
	}

	// Adding new watcher.
	informerStopChannel := make(chan struct{})
	dw.informersStopCh[ngvk] = informerStopChannel
	dw.log.Info("adding new watcher",
		"owner", schema.GroupKind{Group: ownerRef.Group, Kind: ownerRef.Kind},
		"for", schema.GroupKind{Group: gvk.Group, Kind: gvk.Kind}, "namespace", owner.GetNamespace())

	// Build client
	restMapping, err := dw.restMapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return fmt.Errorf("unable to map object to rest endpoint: %w", err)
	}
	client := dw.client.Resource(restMapping.Resource)

	ctx := context.Background()
	var informer informer
	if owner.GetNamespace() == "" {
		informer = dw.opts.NewInformer(
			&cache.ListWatch{
				ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
					return client.List(ctx, options)
				},
				WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
					return client.Watch(ctx, options)
				},
			},
			obj, dw.opts.EventHandlerResyncPeriod, nil)
	} else {
		informer = dw.opts.NewInformer(
			&cache.ListWatch{
				ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
					return client.Namespace(owner.GetNamespace()).List(ctx, options)
				},
				WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
					return client.Namespace(owner.GetNamespace()).Watch(ctx, options)
				},
			},
			obj, dw.opts.EventHandlerResyncPeriod, nil)
	}
	s := source.Informer{
		Informer: informer,
	}

	dw.sinksLock.Lock()
	defer dw.sinksLock.Unlock()
	for _, sink := range dw.sinks {
		if err := s.Start(sink.ctx, sink.handler, sink.queue, sink.predicates...); err != nil {
			return err
		}
	}
	go informer.Run(informerStopChannel)
	return nil
}

// Free all watches associated with the given owner.
func (dw *DynamicWatcher) Free(owner client.Object) error {
	dw.informersLock.Lock()
	defer dw.informersLock.Unlock()

	ownerRef, err := dw.ownerRef(owner)
	if err != nil {
		return err
	}

	for gvk, refs := range dw.informerReferences {
		if _, ok := refs[ownerRef]; ok {
			delete(refs, ownerRef)

			if len(refs) == 0 {
				close(dw.informersStopCh[gvk])
				delete(dw.informersStopCh, gvk)
				dw.log.Info("releasing watcher",
					"kind", gvk.Kind, "group", gvk.Group, "namespace", owner.GetNamespace())
			}
		}
	}
	return nil
}

func (dw *DynamicWatcher) ownerRef(owner client.Object) (OwnerReference, error) {
	ownerGVK, err := apiutil.GVKForObject(owner, dw.scheme)
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
