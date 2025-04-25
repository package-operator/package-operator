package logrutil

import (
	"context"

	"github.com/go-logr/logr"
)

func MustFromContext(ctx context.Context) logr.Logger {
	log, err := logr.FromContext(ctx)
	if err != nil {
		panic(err)
	}
	return log
}
