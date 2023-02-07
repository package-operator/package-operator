package testutil

import (
	"bytes"
	"io"
	"log"
	"net/http"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/registry"
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
