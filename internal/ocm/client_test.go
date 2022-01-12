package ocm

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClientDo_Success(t *testing.T) {
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

	c, _ := NewClient(
		ctx,
		WithAccessToken("access-token"),
		WithClusterExternalID("123"),
		WithEndpoint(s.URL+"/proxy/apis"), // test existing path + trailing / handling
	)

	type TestRequest struct {
		Request string `json:"request"`
	}
	type TestResponse struct {
		Response string `json:"response"`
	}

	tests := []struct {
		name              string
		payload, response interface{}
		method            string
		path              string
		params            url.Values

		expectedResponse interface{}
		expectedPayload  string
	}{
		{
			name:     "post and receive",
			method:   http.MethodPost,
			payload:  &TestRequest{Request: "payload!"},
			response: &TestResponse{},
			path:     "test123",
			params:   url.Values{},

			expectedPayload:  `{"request":"payload!"}`,
			expectedResponse: &TestResponse{Response: "works!"},
		},
		{
			name:    "no response",
			method:  http.MethodPatch,
			payload: &TestRequest{Request: "payload!"},
			path:    "test124",
			params:  url.Values{},

			expectedPayload: `{"request":"payload!"}`,
		},
		{
			name:     "no request payload",
			method:   http.MethodGet,
			response: &TestResponse{},
			path:     "/test123/dkxxx",
			params:   url.Values{},

			expectedResponse: &TestResponse{Response: "works!"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()

			err := c.do(
				ctx, test.method, test.path, test.params, test.payload, test.response)
			require.NoError(t, err)

			assert.Equal(t, test.method, recordedHttpRequest.Method)
			assert.Equal(t,
				"/proxy/apis/"+strings.TrimLeft(test.path, "/"), recordedHttpRequest.URL.Path)
			assert.Equal(t,
				"application/json", recordedHttpRequest.Header.Get("Content-Type"))
			assert.Equal(t,
				"AccessToken 1ou:access-token", recordedHttpRequest.Header.Get("Authorization"))

			if test.expectedPayload != "" {
				assert.Equal(t, test.expectedPayload, string(recordedBody))
			}
			if test.expectedResponse != "" {
				assert.Equal(t, test.expectedResponse, test.response)
			}
		})
	}

}

func TestClientDo_Error(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/proxy/apis/api/clusters_mgmt/v1/clusters" {
			rw.WriteHeader(http.StatusOK)
			fmt.Fprintln(rw, clustersMockAPIResponseBody)
		} else {
			rw.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintln(rw, `{"code":"swordfish","reason":"olm dance"}`)
		}
	}))
	defer s.Close()

	ctx := context.Background()

	c, ocmClientError := NewClient(
		ctx,
		WithAccessToken("access-token"),
		WithClusterExternalID("123"),
		WithEndpoint(s.URL+"/proxy/apis"), // test existing path + trailing / handling
	)
	require.NoError(t, ocmClientError)

	err := c.do(
		ctx, http.MethodPatch, "/broken", nil, nil, nil)
	assert.EqualError(t, err, "HTTP 500: swordfish: olm dance")
}
