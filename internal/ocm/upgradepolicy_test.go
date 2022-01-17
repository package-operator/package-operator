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
		if r.URL.Path == "/proxy/apis/api/clusters_mgmt/v1/clusters" {
			fmt.Fprintln(rw, clustersMockAPIResponseBody)
		} else {
			fmt.Fprintln(rw, `{"response":"works!"}`)
		}
	}))
	defer s.Close()

	ctx := context.Background()

	c, ocmClientError := NewClient(
		ctx,
		WithClusterExternalID("123"),
		WithEndpoint(s.URL+"/proxy/apis"), // test existing path + trailing / handling
	)
	require.NoError(t, ocmClientError)

	_, err := c.PatchUpgradePolicy(
		ctx, UpgradePolicyPatchRequest{
			ID:          "123",
			Value:       "success",
			Description: "works",
		})
	require.NoError(t, err)

	assert.Equal(t, http.MethodPatch, recordedHttpRequest.Method)
	assert.Equal(t, `{"id":"123","value":"success","description":"works"}`, string(recordedBody))
	assert.Equal(t, "/proxy/apis/api/clusters_mgmt/v1/clusters/1ou/upgrade_policies/123/state", recordedHttpRequest.URL.Path)
}

func TestClientGetUpgradePolicy(t *testing.T) {
	var (
		recordedHttpRequest *http.Request
	)
	s := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		recordedHttpRequest = r
		if r.URL.Path == "/proxy/apis/api/clusters_mgmt/v1/clusters" {
			fmt.Fprintln(rw, clustersMockAPIResponseBody)
		} else {
			fmt.Fprintln(rw, `{"value":"completed","description":"1234"}`)
		}
	}))
	defer s.Close()

	ctx := context.Background()

	c, ocmClientError := NewClient(
		ctx,
		WithClusterExternalID("123"),
		WithEndpoint(s.URL+"/proxy/apis"), // test existing path + trailing / handling
	)
	require.NoError(t, ocmClientError)

	res, err := c.GetUpgradePolicy(
		ctx, UpgradePolicyGetRequest{
			ID: "678",
		})
	require.NoError(t, err)

	assert.Equal(t, http.MethodGet, recordedHttpRequest.Method)
	assert.Equal(t, UpgradePolicyGetResponse{
		Value:       "completed",
		Description: "1234",
	}, res)
	assert.Equal(t, "/proxy/apis/api/clusters_mgmt/v1/clusters/1ou/upgrade_policies/678/state", recordedHttpRequest.URL.Path)
}
