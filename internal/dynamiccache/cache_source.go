package dynamiccache

import (
	"context"
	"sync"

	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

type eventHandler struct {
	ctx        context.Context
	handler    handler.EventHandler
	queue      workqueue.RateLimitingInterface
	predicates []predicate.Predicate
}

var _ source.Source = (*cacheSource)(nil)

type cacheSource struct {
	mu       sync.RWMutex
	handlers []eventHandler
	blockNew bool
}

// For printing in startup log messages.
func (e *cacheSource) String() string {
	return "dynamiccache.CacheSource"
}

// Implements source.Source interface to be used as event source when setting up controllers.
func (e *cacheSource) Start(
	ctx context.Context,
	handler handler.EventHandler,
	queue workqueue.RateLimitingInterface,
	predicates ...predicate.Predicate,
) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.blockNew {
		panic("Trying to add EventHandlers to dynamiccache.CacheSource after manager start")
	}
	e.handlers = append(e.handlers, eventHandler{
		ctx:        ctx,
		handler:    handler,
		queue:      queue,
		predicates: predicates,
	})
	return nil
}

// Disables registration of new EventHandlers.
func (e *cacheSource) blockNewRegistrations() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.blockNew = true
}

// Adds all registered EventHandlers to the given informer.
func (e *cacheSource) handleNewInformer(
	informer cache.SharedIndexInformer,
) error {
	// this read lock should not be needed,
	// because the cacheSource should block registration
	// of new event handlers at this point
	e.mu.RLock()
	defer e.mu.RUnlock()

	// ensure to add all event handlers to the new informer
	s := source.Informer{Informer: informer}
	for _, eh := range e.handlers {
		if err := s.Start(eh.ctx, eh.handler, eh.queue, eh.predicates...); err != nil {
			return err
		}
	}
	return nil
}
