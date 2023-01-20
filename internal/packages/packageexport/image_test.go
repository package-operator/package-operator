package packageexport_test

import (
	"bytes"
	"context"
	"io"
	"log"
	"net/http"
	"testing"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"package-operator.run/package-operator/internal/packages/packagecontent"
	"package-operator.run/package-operator/internal/packages/packageexport"
)

func TestImage(t *testing.T) {
	t.Parallel()

	seedingFileMap := map[string][]byte{"manifest.yaml": {5, 6}, "manifest.yml": {7, 8}, "subdir/somethingelse": {9, 10}}

	image, err := packageexport.Image(seedingFileMap)
	require.Nil(t, err)
	layers, err := image.Layers()
	require.Nil(t, err)
	require.Len(t, layers, 1)
}

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

	reg := registry.New(registry.Logger(log.Default()))
	tripper := &tripper{reg}

	transportOpt := crane.WithTransport(tripper)
	ref := "chickens:oldest"

	err := packageexport.PushedImage(ctx, []string{ref}, packagecontent.Files{}, transportOpt)
	assert.Nil(t, err)

	_, err = crane.Pull(ref, transportOpt)
	assert.Nil(t, err)
}
