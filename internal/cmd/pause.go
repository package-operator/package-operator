package cmd

import (
	"context"
	"errors"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"pkg.package-operator.run/cardboard/kubeutils/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"package-operator.run/internal/adapters"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

const PauseMessageAnnotation = "package-operator.run/pause-message"

var (
	errPausingPackage   = errors.New("pausing package")
	errUnpausingPackage = errors.New("unpausing package")
)

func (c *Client) PackageSetPaused(
	ctx context.Context, waiter Waiter,
	kind, name, namespace string, pause bool, message string,
) error {
	var pkg adapters.PackageAccessor
	switch kind {
	case "package":
		pkg = adapters.NewGenericPackage(c.client.Scheme())
	case "clusterpackage":
		pkg = adapters.NewGenericClusterPackage(c.client.Scheme())
	default:
		panic("This path must never be taken. Caller has to check for valid kind!")
	}

	if err := c.client.Get(ctx, client.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}, pkg.ClientObject()); err != nil {
		return fmt.Errorf("getting package object: %w", err)
	}

	pkg.SetSpecPaused(pause)
	pkgObj := pkg.ClientObject()
	if pause {
		annotations := pkgObj.GetAnnotations()
		if annotations == nil {
			annotations = map[string]string{}
		}
		annotations[PauseMessageAnnotation] = message
		pkgObj.SetAnnotations(annotations)
	} else {
		annotations := pkgObj.GetAnnotations()
		delete(annotations, PauseMessageAnnotation)
		pkgObj.SetAnnotations(annotations)
	}

	if err := c.client.Update(ctx, pkgObj); err != nil {
		if pause {
			return fmt.Errorf("%w: %w", errPausingPackage, err)
		}
		return fmt.Errorf("%w: %w", errUnpausingPackage, err)
	}

	conditionType := corev1alpha1.PackagePaused
	if !pause {
		conditionType = corev1alpha1.PackageAvailable
	}

	return waiter.WaitForCondition(ctx, pkgObj,
		conditionType, metav1.ConditionTrue,
		wait.WithTimeout(60*time.Second),
	)
}
