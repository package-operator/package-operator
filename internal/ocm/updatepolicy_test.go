package ocm

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClientPatchUpgradePolicy(t *testing.T) {
	var (
		recordedHttpRequest *http.Request
		recordedBody        []byte
	)
	s := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		recordedHttpRequest = r
		recordedBody, _ = ioutil.ReadAll(recordedHttpRequest.Body)
		fmt.Fprintln(rw, `{"response":"works!"}`)
	}))
	defer s.Close()

	c := NewClient(
		WithAccessToken("access-token"),
		WithClusterID("123"),
		WithEndpoint(s.URL+"/proxy/apis"), // test existing path + trailing / handling
	)

	ctx := context.Background()

	_, err := c.PatchUpgradePolicy(
		ctx, UpgradePolicyPatchRequest{
			ID:          "123",
			Value:       "success",
			Description: "works",
		})
	require.NoError(t, err)

	assert.Equal(t, `{"id":"123","value":"success","description":"works"}`, string(recordedBody))
}
