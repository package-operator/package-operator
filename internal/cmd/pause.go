package cmd

import (
	"context"
	"errors"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"pkg.package-operator.run/cardboard/kubeutils/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

const PauseMessageAnnotation = "package-operator.run/pause-message"

var (
	errPausingPackage   = errors.New("pausing package")
	errUnpausingPackage = errors.New("unpausing package")
)

func (c *Client) PackageSetPaused(
	ctx context.Context, waiter Waiter,
	name, namespace string, pause bool, message string,
) error {
	pkg := &corev1alpha1.Package{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	if err := c.client.Get(ctx, client.ObjectKeyFromObject(pkg), pkg); err != nil {
		return fmt.Errorf("getting package object: %w", err)
	}

	pkg.Spec.Paused = pause
	if pause {
		if pkg.Annotations == nil {
			pkg.Annotations = map[string]string{}
		}

		pkg.Annotations[PauseMessageAnnotation] = message
	} else {
		delete(pkg.Annotations, PauseMessageAnnotation)
	}

	if err := c.client.Update(ctx, pkg); err != nil {
		if pause {
			return fmt.Errorf("%w: %w", errPausingPackage, err)
		}
		return fmt.Errorf("%w: %w", errUnpausingPackage, err)
	}

	conditionType := corev1alpha1.PackagePaused
	if !pause {
		conditionType = corev1alpha1.PackageAvailable
	}

	return waiter.WaitForCondition(ctx, pkg,
		conditionType, metav1.ConditionTrue,
		wait.WithTimeout(60*time.Second),
	)
}
