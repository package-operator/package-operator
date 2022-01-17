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

	"github.com/openshift/addon-operator/internal/version"
)

type Client struct {
	opts       ClientOptions
	httpClient *http.Client
}

// Creates a new OCM client with the given options.
func NewClient(ctx context.Context, opts ...Option) (*Client, error) {
	c := &Client{}
	for _, opt := range opts {
		opt(&c.opts)
	}

	c.httpClient = &http.Client{}

	// Getting the Cluster Internal ID from the External ID
	clusterInfo, err := c.GetCluster(ctx, ClusterGetRequest{})
	if err != nil {
		return nil, fmt.Errorf("getting cluster info: %w", err)
	}
	if len(clusterInfo.Items) == 0 {
		return nil, fmt.Errorf("cluster %s not found", c.opts.ClusterExternalID)
	}

	c.opts.ClusterID = clusterInfo.Items[0].Id
	return c, nil
}

type ClientOptions struct {
	Endpoint          string
	ClusterExternalID string
	ClusterID         string
	AccessToken       string
}

type Option func(o *ClientOptions)

func WithEndpoint(endpoint string) Option {
	return func(o *ClientOptions) {
		// ensure there is always a single trailing "/"
		o.Endpoint = strings.TrimRight(endpoint, "/") + "/"
	}
}

func WithClusterExternalID(clusterExternalID string) Option {
	return func(o *ClientOptions) {
		o.ClusterExternalID = clusterExternalID
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
	params url.Values,
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

	var fullUrl string
	if len(params) > 0 {
		fullUrl = reqURL.String() + "?" + params.Encode()
	} else {
		fullUrl = reqURL.String()
	}

	httpReq, err := http.NewRequestWithContext(ctx, httpMethod, fullUrl, resBody)
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
			return fmt.Errorf("reading error response body %s: %w", fullUrl, err)
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
			return fmt.Errorf("reading response body %s: %w", fullUrl, err)
		}

		if err := json.Unmarshal(body, result); err != nil {
			return fmt.Errorf("unmarshal json response %s: %w", fullUrl, err)
		}
	}

	return nil
}
