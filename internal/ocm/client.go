package ocm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"github.com/openshift/addon-operator/internal/metrics"

	"github.com/openshift/addon-operator/internal/version"
)

type Client struct {
	opts       ClientOptions
	httpClient *http.Client
}

// Creates a new OCM client with the given options.
func NewClient(opts ...Option) *Client {
	c := &Client{}
	for _, opt := range opts {
		opt(&c.opts)
	}

	c.httpClient = &http.Client{}
	return c
}

type ClientOptions struct {
	Endpoint    string
	ClusterID   string
	AccessToken string
	Recorder    *metrics.Recorder
}

type Option func(o *ClientOptions)

func WithRecorder(recorder *metrics.Recorder) Option {
	return func(o *ClientOptions) {
		o.Recorder = recorder
	}
}

func WithEndpoint(endpoint string) Option {
	return func(o *ClientOptions) {
		// ensure there is always a single trailing "/"
		o.Endpoint = strings.TrimRight(endpoint, "/") + "/"
	}
}

func WithClusterID(clusterID string) Option {
	return func(o *ClientOptions) {
		o.ClusterID = clusterID
	}
}

func WithAccessToken(accessToken string) Option {
	return func(o *ClientOptions) {
		o.AccessToken = accessToken
	}
}

type OCMError struct {
	StatusCode int
	Code       string `json:"code"`
	Reason     string `json:"reason"`
}

func (e OCMError) Error() string {
	return fmt.Sprintf("HTTP %d: %s: %s", e.StatusCode, e.Code, e.Reason)
}

func (c *Client) do(
	ctx context.Context,
	httpMethod string,
	path string,
	payload, result interface{},
) error {
	// Build URL
	reqURL, err := url.Parse(c.opts.Endpoint)
	if err != nil {
		return fmt.Errorf("parsing endpoint URL: %w", err)
	}
	reqURL = reqURL.ResolveReference(&url.URL{
		Path: strings.TrimLeft(path, "/"), // trim first slash to always be relative to baseURL
	})

	// Payload
	var resBody io.Reader
	if payload != nil {
		j, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("marshaling json: %w", err)
		}

		resBody = bytes.NewBuffer(j)
	}

	// Request
	httpReq, err := http.NewRequestWithContext(ctx, httpMethod, reqURL.String(), resBody)
	if err != nil {
		return fmt.Errorf("creating http request: %w", err)
	}

	// Headers
	httpReq.Header.Add("Authorization", fmt.Sprintf(
		"AccessToken %s:%s", c.opts.ClusterID, c.opts.AccessToken))
	httpReq.Header.Add("User-Agent", fmt.Sprintf("AddonOperator/%s", version.Version))
	httpReq.Header.Add("Content-Type", "application/json")

	httpRes, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("executing http request: %w", err)
	}
	defer httpRes.Body.Close()

	// HTTP Error handling
	if httpRes.StatusCode >= 400 && httpRes.StatusCode <= 599 {
		body, err := ioutil.ReadAll(httpRes.Body)
		if err != nil {
			return fmt.Errorf("reading error response body: %w", err)
		}

		var ocmErr OCMError
		if err := json.Unmarshal(body, &ocmErr); err != nil {
			return fmt.Errorf(
				"HTTP %d: unmarshal json error response %s: %w", httpRes.StatusCode, string(body), err)
		}
		ocmErr.StatusCode = httpRes.StatusCode
		return ocmErr
	}

	// Read response
	if result != nil {
		body, err := ioutil.ReadAll(httpRes.Body)
		if err != nil {
			return fmt.Errorf("reading response body: %w", err)
		}

		if err := json.Unmarshal(body, result); err != nil {
			return fmt.Errorf("unmarshal json response: %w", err)
		}
	}

	return nil
}
