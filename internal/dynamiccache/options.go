package dynamiccache

import (
	"time"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	_ CacheOption = (*FieldIndexersByGVK)(nil)
	_ CacheOption = (*SelectorsByGVK)(nil)
)

// FieldIndexers by GroupVersionKind.
type FieldIndexersByGVK map[schema.GroupVersionKind][]FieldIndexer

func (fi FieldIndexersByGVK) ApplyToCacheOptions(opts *CacheOptions) {
	opts.Indexers = fi
}

func (fi FieldIndexersByGVK) forGVK(gvk schema.GroupVersionKind) []FieldIndexer {
	if specific, found := fi[gvk]; found {
		return specific
	}
	if defaultIndexers, found := fi[schema.GroupVersionKind{}]; found {
		return defaultIndexers
	}

	return nil
}

// defined here, so we don't have to change selector.go,
// taken from controller-runtime.
func (s SelectorsByGVK) ApplyToCacheOptions(opts *CacheOptions) {
	opts.Selectors = s
}

// Time between full cache resyncs.
// A 10 percent jitter will be added to the resync period between informers,
// so that all informers will not send list requests simultaneously.
type ResyncInterval time.Duration

// Default cache resunc interval, if not specified.
const defaultResyncInterval = 10 * time.Hour

// FieldIndexer adds a custom index to the cache.
type FieldIndexer struct {
	// Field name to refer to the index later.
	Field string
	// IndexFunc extracts the indexed values from objects.
	Indexer client.IndexerFunc
}

// CacheOption customizes an informer creation and cache behavior.
type CacheOption interface {
	ApplyToCacheOptions(opts *CacheOptions)
}

// CacheOptions holds all Cache configuration parameters.
type CacheOptions struct {
	// Custom cache indexes.
	Indexers FieldIndexersByGVK
	// Selectors filter caches on the api server.
	Selectors SelectorsByGVK
	// Time between full cache resyncs.
	ResyncInterval time.Duration
}

func (co *CacheOptions) Default() {
	if co.ResyncInterval == 0 {
		co.ResyncInterval = defaultResyncInterval
	}
}
