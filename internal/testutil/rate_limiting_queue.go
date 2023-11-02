package testutil

import (
	"time"

	"github.com/stretchr/testify/mock"
)

type RateLimitingQueue struct {
	mock.Mock
}

func (q *RateLimitingQueue) Add(item any) {
	q.Called(item)
}

func (q *RateLimitingQueue) Len() int {
	args := q.Called()
	return args.Int(0)
}

func (q *RateLimitingQueue) Get() (item any, shutdown bool) {
	args := q.Called()
	return args.Get(0), args.Bool(1)
}

func (q *RateLimitingQueue) Done(item any) {
	q.Called(item)
}

func (q *RateLimitingQueue) ShutDown() {
	q.Called()
}

func (q *RateLimitingQueue) ShutDownWithDrain() {
	q.Called()
}

func (q *RateLimitingQueue) ShuttingDown() bool {
	args := q.Called()
	return args.Bool(0)
}

func (q *RateLimitingQueue) AddAfter(item any, duration time.Duration) {
	q.Called(item, duration)
}

func (q *RateLimitingQueue) AddRateLimited(item any) {
	q.Called(item)
}

func (q *RateLimitingQueue) Forget(item any) {
	q.Called(item)
}

func (q *RateLimitingQueue) NumRequeues(item any) int {
	args := q.Called(item)
	return args.Int(0)
}
