package testutil

import (
	"time"

	"github.com/stretchr/testify/mock"
)

type TypedRateLimitingQueue[T comparable] struct {
	mock.Mock
}

func (q *TypedRateLimitingQueue[T]) Add(item T) {
	q.Called(item)
}

func (q *TypedRateLimitingQueue[T]) Len() int {
	args := q.Called()
	return args.Int(0)
}

func (q *TypedRateLimitingQueue[T]) Get() (item T, shutdown bool) {
	args := q.Called()
	return args.Get(0).(T), args.Bool(1)
}

func (q *TypedRateLimitingQueue[T]) Done(item T) {
	q.Called(item)
}

func (q *TypedRateLimitingQueue[T]) ShutDown() {
	q.Called()
}

func (q *TypedRateLimitingQueue[T]) ShutDownWithDrain() {
	q.Called()
}

func (q *TypedRateLimitingQueue[T]) ShuttingDown() bool {
	args := q.Called()
	return args.Bool(0)
}

func (q *TypedRateLimitingQueue[T]) AddAfter(item T, duration time.Duration) {
	q.Called(item, duration)
}

func (q *TypedRateLimitingQueue[T]) AddRateLimited(item T) {
	q.Called(item)
}

func (q *TypedRateLimitingQueue[T]) Forget(item T) {
	q.Called(item)
}

func (q *TypedRateLimitingQueue[T]) NumRequeues(item T) int {
	args := q.Called(item)
	return args.Int(0)
}
