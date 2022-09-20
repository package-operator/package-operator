package dynamiccache

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCacheSource(t *testing.T) {
	cs := &cacheSource{}

	ctx := context.Background()
	err := cs.Start(ctx, nil, nil)
	require.NoError(t, err)

	// just checking that the underlying function is called.
	err = cs.handleNewInformer(nil)
	require.EqualError(t, err, "must specify Informer.Informer")

	cs.blockNewRegistrations()

	require.PanicsWithValue(t,
		"Trying to add EventHandlers to dynamiccache.CacheSource after manager start",
		func() {
			_ = cs.Start(ctx, nil, nil)
		},
	)
}

func TestCacheSource1(t *testing.T) {
	a := map[string]map[string]string{}
	// a["a"]["a"] = "a"
	b := a
	b["b"] = map[string]string{}
	b["b"]["b"] = "b"
	fmt.Println(a)
}
