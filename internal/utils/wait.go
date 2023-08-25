package utils

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ConditionFnNotFound is a [wait.ConditionWithContextFunc] that reports done when the given obj is not found via the given c.
func ConditionFnNotFound(c client.Client, obj client.Object) wait.ConditionWithContextFunc {
	return func(ctx context.Context) (bool, error) {
		err := c.Get(ctx, client.ObjectKeyFromObject(obj), obj)
		switch {
		case err == nil:
			return false, nil
		case errors.IsNotFound(err):
			return true, nil
		default:
			return false, fmt.Errorf("waiting for object to be gone: %w", err)
		}
	}
}
