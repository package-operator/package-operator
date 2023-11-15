package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//nolint:paralleltest // testing can't set env vars on parallel tests.
func TestImageURLWithOverride(t *testing.T) {
	t.Run("default", func(t *testing.T) {
		img := "quay.io/something/else:tag"
		r, err := ImageURLWithOverrideFromEnv(img)
		require.NoError(t, err)
		assert.Equal(t, img, r)
	})

	regHost := "localhost:123"
	testDgst := "sha256:52a6b1268e32ed5b6f59da8222f7627979bfb739f32aae3fb5b5ed31b8bf80c4" //nolint:gosec // no credential.

	testsOk := []struct {
		image  string
		expOut string
	}{
		{"nginx", regHost + "/library/nginx:latest"},
		{"nginx:1.23.3", regHost + "/library/nginx:1.23.3"},
		{"nginx@" + testDgst, regHost + "/library/nginx@" + testDgst},
		{"nginx:1.23.3@" + testDgst, regHost + "/library/nginx@" + testDgst},
		{"jboss/keycloak", regHost + "/jboss/keycloak:latest"},
		{"jboss/keycloak:16.1.1", regHost + "/jboss/keycloak:16.1.1"},
		{"jboss/keycloak@" + testDgst, regHost + "/jboss/keycloak@" + testDgst},
		{"jboss/keycloak:16.1.1@" + testDgst, regHost + "/jboss/keycloak@" + testDgst},
		{"quay.io/keycloak/keycloak", regHost + "/keycloak/keycloak:latest"},
		{"quay.io/keycloak/keycloak:20.0.3", regHost + "/keycloak/keycloak:20.0.3"},
		{"quay.io/keycloak/keycloak@" + testDgst, regHost + "/keycloak/keycloak@" + testDgst},
		{"quay.io/keycloak/keycloak:20.0.3@" + testDgst, regHost + "/keycloak/keycloak@" + testDgst},
		{"example.com:12345/imggroup/imgname", regHost + "/imggroup/imgname:latest"},
		{"example.com:12345/imggroup/imgname:1.0.0", regHost + "/imggroup/imgname:1.0.0"},
		{"example.com:12345/imggroup/imgname@" + testDgst, regHost + "/imggroup/imgname@" + testDgst},
		{"example.com:12345/imggroup/imgname:1.0.0@" + testDgst, regHost + "/imggroup/imgname@" + testDgst},
	}

	for i := range testsOk {
		test := testsOk[i]
		t.Run("ok/"+test.image, func(t *testing.T) {
			t.Setenv("PKO_REPOSITORY_HOST", regHost)
			out, err := ImageURLWithOverrideFromEnv(test.image)
			require.NoError(t, err)
			assert.Equal(t, test.expOut, out)
		})
	}

	t.Run("errorImg", func(t *testing.T) {
		t.Setenv("PKO_REPOSITORY_HOST", regHost)
		i, err := ImageURLWithOverrideFromEnv("")
		require.Error(t, err, i)
	})

	t.Run("errorOverride", func(t *testing.T) {
		t.Setenv("PKO_REPOSITORY_HOST", "https://ʕ˵•ᴥ •˵ʔ")
		i, err := ImageURLWithOverrideFromEnv("")
		require.Error(t, err, i)
	})
}
