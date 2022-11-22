package export_test

import (
	"bytes"
	"context"
	"io"
	"log"
	"net/http"
	"testing"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/stretchr/testify/assert"
	"package-operator.run/package-operator/cmd/kubectl-package/export"
)

type writer struct {
	resp *http.Response
}

func (w *writer) Header() http.Header { return w.resp.Header }

func (w *writer) Write(data []byte) (int, error) {
	buf := bytes.NewBuffer(data)
	w.resp.Body = io.NopCloser(buf)

	return len(data), nil
}

func (w *writer) WriteHeader(statusCode int) { w.resp.StatusCode = statusCode }

type tripper struct {
	handler http.Handler
}

func (t tripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Body == nil {
		req.Body = io.NopCloser(&bytes.Buffer{})
	}
	resp := &http.Response{Status: "ok", StatusCode: http.StatusOK, Header: http.Header{}, Request: req}
	w := &writer{resp}
	t.handler.ServeHTTP(w, req)

	return resp, nil
}

func TestPush(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	ref := name.MustParseReference("chickens:oldest")

	reg := registry.New(registry.Logger(log.Default()))
	tripper := &tripper{reg}

	transportOpt := crane.WithTransport(tripper)

	err := export.Push(ctx, []name.Reference{ref}, empty.Image, transportOpt)
	assert.Nil(t, err)

	_, err = crane.Pull(ref.String(), transportOpt)
	assert.Nil(t, err)
}
