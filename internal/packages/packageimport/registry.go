package packageimport

import (
	"context"
	"strings"
	"sync"

	"package-operator.run/package-operator/internal/packages/packagecontent"
	"package-operator.run/package-operator/internal/utils"
)

// Registry handles pulling images from a registry during PKO runtime.
type Registry struct {
	registryHostOverrides map[string]string

	inFlight     map[string]pullRequest
	inFlightLock sync.Mutex
}

func NewRegistry(registryHostOverrides map[string]string) *Registry {
	return &Registry{
		registryHostOverrides: registryHostOverrides,
		inFlight:              map[string]pullRequest{},
	}
}

func (r *Registry) Pull(ctx context.Context, image string) (packagecontent.Files, error) {
	return r.pullOnce(ctx, image)
}

func (r *Registry) applyOverride(image string) (string, error) {
	for original, override := range r.registryHostOverrides {
		if strings.HasPrefix(image, original) {
			return utils.ImageURLWithOverride(image, override)
		}
	}
	return image, nil
}

// pull the same image only once in parallel.
func (r *Registry) pullOnce(ctx context.Context, image string) (packagecontent.Files, error) {
	image, err := r.applyOverride(image)
	if err != nil {
		return nil, err
	}

	r.inFlightLock.Lock()
	if request, ok := r.inFlight[image]; ok {
		r.inFlightLock.Unlock()
		return request.Wait()
	}

	request := newPullRequest()
	r.inFlight[image] = request
	r.inFlightLock.Unlock()

	files, err := PulledImage(ctx, image)
	request.Done(files, err)

	r.inFlightLock.Lock()
	delete(r.inFlight, image)
	r.inFlightLock.Unlock()

	return files, err
}

type pullRequest struct {
	Files  packagecontent.Files
	Err    error
	doneCh chan struct{}
}

func newPullRequest() pullRequest {
	return pullRequest{
		doneCh: make(chan struct{}),
	}
}

func (r *pullRequest) Done(files packagecontent.Files, err error) {
	r.Files = files
	r.Err = err
	close(r.doneCh)
}

func (r *pullRequest) Wait() (packagecontent.Files, error) {
	<-r.doneCh
	return r.Files, r.Err
}
