package objectsync

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"package-operator.run/apis/core/v1alpha1"
)

type ObjectSyncController struct {
	log    logr.Logger
	client client.Client
	scheme *runtime.Scheme

	dynamicCache dynamicCache
}

type dynamicCache interface {
	client.Reader
	Source(handler handler.EventHandler, predicates ...predicate.Predicate) source.Source
	Free(ctx context.Context, obj client.Object) error
	Watch(ctx context.Context, owner client.Object, obj runtime.Object) error
}

func (c *ObjectSyncController) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.ObjectSync{}).
		WatchesRawSource(
			c.dynamicCache.Source(
				handler.EnqueueRequestForOwner(mgr.GetScheme(), mgr.GetRESTMapper(), &v1alpha1.ObjectSync{}),
				predicate.NewPredicateFuncs(func(object client.Object) bool {
					c.log.Info(
						"processing dynamic cache event",
						"object", client.ObjectKeyFromObject(object),
						"owners", object.GetOwnerReferences())
					return true
				}),
			),
		).
		Complete(c)
}

func (osc *ObjectSyncController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := osc.log.WithValues("ObjectSync", req.String())
	defer log.Info("reconciled")

	objectSync := &v1alpha1.ObjectSync{}
	if err := osc.client.Get(ctx, req.NamespacedName, objectSync); err != nil {
		return ctrl.Result{}, fmt.Errorf("getting objectsync: %w", err)
	}

	sourceObject := &unstructured.Unstructured{}
	sourceObject.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    objectSync.Src.Kind,
	})

	if err := osc.client.Get(ctx, types.NamespacedName{
		Namespace: objectSync.Src.Namespace,
		Name:      objectSync.Src.Name,
	}, sourceObject); err != nil {
		return ctrl.Result{}, fmt.Errorf("getting source object: %w", err)
	}

	for _, dest := range objectSync.Dest {
		targetObject := sourceObject.DeepCopy()
		// huh? this doesn't work... i'd like to copy the GVK and the body but override metadata...
		// ...does this work though?
		targetObject.Object["metadata"] = map[string]interface{}{
			"namespace": dest.Namespace,
			"name":      dest.Name,
		}
		// do a server side apply
	}

	return ctrl.Result{}, nil
}
