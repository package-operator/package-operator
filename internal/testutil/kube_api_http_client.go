package testutil

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

func GetEndpointOnCluster(ctx context.Context, namespace, service, path string, port int) ([]byte, error) {
	restConfig := config.GetConfigOrDie()
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

	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, requestURL.String(), nil) // OK
	resp, err := client.Do(req)
	// resp, err := client.Get(requestURL.String())
	if err != nil {
		return nil, err
	}
	defer func() {
		err = resp.Body.Close()
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return body, nil
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
