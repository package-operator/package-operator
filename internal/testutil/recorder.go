package testutil

import (
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/mock"
)

// Summary is a mock object for `prometheus.Summary`
type Summary struct {
	mock.Mock
}

func (s *Summary) Observe(d float64) {
	s.Called(d)
}

// implement `prometheus.Metric` interface
func (s *Summary) Desc() *prometheus.Desc {
	s.Called()
	return &prometheus.Desc{}
}

// implement `prometheus.Metric` interface
func (s *Summary) Write(m *dto.Metric) error {
	args := s.Called(m)
	return args.Error(0)
}

// implement `prometheus.Collector` interface
func (s *Summary) Describe(c chan<- *prometheus.Desc) {
	s.Called(c)
}

// implement `prometheus.Collector` interface
func (s *Summary) Collect(m chan<- prometheus.Metric) {
	s.Called(m)
}

func NewSummaryMock() *Summary {
	return &Summary{}
}
