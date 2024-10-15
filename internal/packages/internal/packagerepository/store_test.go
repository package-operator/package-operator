package packagerepository

import (
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"package-operator.run/internal/apis/manifests"
)

func TestStore(t *testing.T) {
	t.Parallel()

	repo1 := types.NamespacedName{Namespace: "foo", Name: "repo"}
	repo2 := types.NamespacedName{Namespace: "bar", Name: "repo"}
	clRepo := types.NamespacedName{Name: "cluster-repo"}

	r1 := &RepositoryIndex{repo: &manifests.Repository{ObjectMeta: metav1.ObjectMeta{Namespace: "foo", Name: "repo"}}}
	r2 := &RepositoryIndex{repo: &manifests.Repository{ObjectMeta: metav1.ObjectMeta{Namespace: "bar", Name: "repo"}}}
	cr := &RepositoryIndex{repo: &manifests.Repository{ObjectMeta: metav1.ObjectMeta{Name: "cluster-repo"}}}

	store := NewRepositoryStore()
	require.False(t, store.Contains(repo1))
	require.False(t, store.Contains(repo2))
	require.False(t, store.Contains(clRepo))

	store.Store(r1, repo1)
	store.Store(r2, repo2)
	store.Store(cr, clRepo)

	require.ElementsMatch(t, store.GetKeys(), []string{"foo.repo", "bar.repo", "cluster-repo"})
	require.ElementsMatch(t, store.GetAll(), []*RepositoryIndex{r1, r2, cr})
	require.ElementsMatch(t, store.GetForNamespace("foo"), []*RepositoryIndex{r1, cr})
	require.ElementsMatch(t, store.GetForNamespace("bar"), []*RepositoryIndex{r2, cr})
	require.ElementsMatch(t, store.GetForNamespace("xyz"), []*RepositoryIndex{cr})

	store.Delete(repo1)
	store.Delete(repo2)
	store.Delete(clRepo)

	require.Empty(t, store.GetKeys())
	require.Empty(t, store.GetAll())
	require.Empty(t, store.GetForNamespace("foo"))
	require.Empty(t, store.GetForNamespace("bar"))
	require.Empty(t, store.GetForNamespace("xyz"))
}
