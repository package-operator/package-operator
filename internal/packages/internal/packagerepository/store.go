package packagerepository

import (
	"fmt"
	"strings"
	"sync"

	"k8s.io/apimachinery/pkg/types"
)

type RepositoryStore interface {
	Contains(nsName types.NamespacedName) bool
	GetKeys() []string
	GetAll() []*RepositoryIndex
	GetForNamespace(namespace string) []*RepositoryIndex
	Store(idx *RepositoryIndex, nsName types.NamespacedName)
	Delete(nsName types.NamespacedName)
}

func NewRepositoryStore() RepositoryStore {
	return &MapRepositoryStore{}
}

type MapRepositoryStore struct {
	data sync.Map
}

func (s *MapRepositoryStore) Contains(nsName types.NamespacedName) bool {
	_, ok := s.data.Load(s.key(nsName))
	return ok
}

func (s *MapRepositoryStore) get(filter func(string) bool) []*RepositoryIndex {
	var results []*RepositoryIndex
	appender := func(key, value any) bool {
		if filter(key.(string)) {
			results = append(results, value.(*RepositoryIndex))
		}
		return true
	}
	s.data.Range(appender)
	return results
}

func (s *MapRepositoryStore) GetKeys() []string {
	var results []string
	appender := func(key, _ any) bool {
		results = append(results, key.(string))
		return true
	}
	s.data.Range(appender)
	return results
}

func (s *MapRepositoryStore) GetAll() []*RepositoryIndex {
	return s.get(func(_ string) bool {
		return true
	})
}

func (s *MapRepositoryStore) GetForNamespace(namespace string) []*RepositoryIndex {
	return s.get(func(key string) bool {
		return !strings.Contains(key, ".") || strings.HasPrefix(key, namespace+".")
	})
}

func (s *MapRepositoryStore) Store(idx *RepositoryIndex, nsName types.NamespacedName) {
	s.data.Store(s.key(nsName), idx)
}

func (s *MapRepositoryStore) Delete(nsName types.NamespacedName) {
	s.data.Delete(s.key(nsName))
}

func (s *MapRepositoryStore) key(nsName types.NamespacedName) string {
	if nsName.Namespace == "" {
		return nsName.Name
	}
	return fmt.Sprintf("%s.%s", nsName.Namespace, nsName.Name)
}
