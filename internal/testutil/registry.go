package testutil

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"testing"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/registry"
	containerregistrypkgv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/stretchr/testify/require"
)

type InMemoryRegistry struct {
	RoundTripper http.RoundTripper
	Handler      http.Handler
	CraneOpt     crane.Option
}

type (
	inMemoryRegistryWriter struct {
		resp *http.Response
	}
	inMemoryRegistryRoundTripper struct {
		handler http.Handler
	}
)

func NewInMemoryRegistry() *InMemoryRegistry {
	r := &InMemoryRegistry{}
	r.Handler = registry.New(registry.Logger(log.Default()))
	r.RoundTripper = &inMemoryRegistryRoundTripper{r.Handler}
	r.CraneOpt = crane.WithTransport(r.RoundTripper)

	return r
}

func (w *inMemoryRegistryWriter) Header() http.Header { return w.resp.Header }

func (w *inMemoryRegistryWriter) Write(data []byte) (int, error) {
	w.resp.Body = io.NopCloser(bytes.NewBuffer(data))

	return len(data), nil
}

func (w *inMemoryRegistryWriter) WriteHeader(statusCode int) { w.resp.StatusCode = statusCode }

func (t inMemoryRegistryRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Body == nil {
		req.Body = io.NopCloser(&bytes.Buffer{})
	}
	resp := &http.Response{Status: "ok", StatusCode: http.StatusOK, Header: http.Header{}, Request: req}
	w := &inMemoryRegistryWriter{resp}
	t.handler.ServeHTTP(w, req)

	return resp, nil
}

func BuildImage(t *testing.T, layerData map[string][]byte, labels map[string]string) containerregistrypkgv1.Image {
	t.Helper()

	configFile := &containerregistrypkgv1.ConfigFile{
		Config: containerregistrypkgv1.Config{
			Labels: labels,
		},
		RootFS: containerregistrypkgv1.RootFS{Type: "layers"},
	}
	image, err := mutate.ConfigFile(empty.Image, configFile)
	require.NoError(t, err)

	layer, err := crane.Layer(layerData)
	require.NoError(t, err)

	image, err = mutate.AppendLayers(image, layer)
	require.NoError(t, err)

	image, err = mutate.Canonical(image)
	require.NoError(t, err)

	return image
}
