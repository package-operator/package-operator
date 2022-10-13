package metricsmocks

import (
	"github.com/stretchr/testify/mock"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type RecorderMock struct {
	mock.Mock
}

func (r *RecorderMock) RecordDynamicCacheInformers(informerCount int) {
	r.Called(informerCount)
}

func (r *RecorderMock) RecordDynamicCacheObjects(gvk schema.GroupVersionKind, count int) {
	r.Called(gvk, count)
}
