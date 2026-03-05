package testutil

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"k8s.io/client-go/rest"
)

// This function gets an endpoint on cluster for a given service and port.
func GetEndpointOnCluster(ctx context.Context, restConfig *rest.Config,
	namespace, service, path string, port int) (bytes []byte, err error,
) {
	transport, err := rest.TransportFor(restConfig)
	if err != nil {
		return nil, err
	}

	// Construct an http.Client that authenticates with the apiserver.
	client := &http.Client{
		Transport: transport,
	}

	// Construct request
	requestURL, err := createURL(restConfig.Host, namespace, service, path, port)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL.String(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req) //nolint:gosec // G704: URL is constructed from restConfig.Host (Kubernetes API)
	if err != nil {
		return nil, err
	}
	defer func() {
		err = errors.Join(err, resp.Body.Close())
	}()

	bytes, err = io.ReadAll(resp.Body)
	return
}

func createURL(host, namespace, service, path string, port int) (requestURL *url.URL, err error) {
	if !strings.HasSuffix(host, "/") {
		host += "/"
	}
	requestURL, err = url.Parse(host)
	if err != nil {
		return nil, err
	}
	requestURL.Path = fmt.Sprintf(
		"/api/v1/namespaces/%s/services/%s:%d/proxy%s",
		namespace,
		service,
		port,
		path,
	)
	return
}
