package objectdeployments

import (
	"context"
	"fmt"
	"sort"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	ObjectSetHashAnnotation      = "package-operator.run/hash"
	DeploymentRevisionAnnotation = "package-operator.run/deployment-revision"
)

type reconciler interface {
	Reconcile(ctx context.Context, objectSetDeployment genericObjectDeployment) (ctrl.Result, error)
}

type GenericObjectDeploymentController struct {
	gvk        schema.GroupVersionKind
	childGvk   schema.GroupVersionKind
	client     client.Client
	log        logr.Logger
	scheme     *runtime.Scheme
	reconciler []reconciler
}

func NewGenericObjectDeploymentController(
	gvk schema.GroupVersionKind,
	childGVK schema.GroupVersionKind,
	c client.Client, log logr.Logger, scheme *runtime.Scheme,
) *GenericObjectDeploymentController {
	controller := &GenericObjectDeploymentController{
		gvk:      gvk,
		childGvk: childGVK,
		client:   c,
		log:      log,
		scheme:   scheme,
	}
	controller.reconciler = []reconciler{
		&hashReconciler{
			client: c,
		},
		&objectSetReconciler{
			client:                      c,
			listObjectSetsForDeployment: controller.listObjectSetsByRevision,
			reconcilers: []objectSetSubReconciler{
				&newRevisionReconciler{
					client:       c,
					newObjectSet: controller.newOperandChild,
					scheme:       scheme,
				},
				&archiveReconciler{
					client: c,
				},
			},
		},
	}

	return controller
}

func NewObjectDeploymentController(
	c client.Client, log logr.Logger, scheme *runtime.Scheme,
) *GenericObjectDeploymentController {
	return NewGenericObjectDeploymentController(
		corev1alpha1.GroupVersion.WithKind("ObjectDeployment"),
		corev1alpha1.GroupVersion.WithKind("ObjectSet"),
		c,
		log,
		scheme,
	)
}

func NewClusterObjectDeploymentController(
	c client.Client, log logr.Logger, scheme *runtime.Scheme,
) *GenericObjectDeploymentController {
	return NewGenericObjectDeploymentController(
		corev1alpha1.GroupVersion.WithKind("ClusterObjectDeployment"),
		corev1alpha1.GroupVersion.WithKind("ClusterObjectSet"),
		c,
		log,
		scheme,
	)
}

func (od *GenericObjectDeploymentController) Reconcile(
	ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := od.log.WithValues("ObjectDeployment", req.String())
	defer log.Info("reconciled")
	ctx = logr.NewContext(ctx, log)
	objectDeployment := od.newOperand()
	if err := od.client.Get(ctx, req.NamespacedName, objectDeployment.ClientObject()); err != nil {
		// Ignore not found errors on delete
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	var (
		res ctrl.Result
		err error
	)
	for _, reconciler := range od.reconciler {
		res, err = reconciler.Reconcile(ctx, objectDeployment)
		if err != nil || !res.IsZero() {
			break
		}
	}
	if err != nil {
		return res, err
	}
	objectDeployment.UpdatePhase()
	objectDeployment.SetObservedGeneration(objectDeployment.GetGeneration())
	return res, od.client.Status().Update(ctx, objectDeployment.ClientObject())
}

func (od *GenericObjectDeploymentController) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).For(od.newOperand().ClientObject()).
		Owns(od.newOperandChild().ClientObject()).
		Complete(od)
}

func (od *GenericObjectDeploymentController) newOperand() genericObjectDeployment {
	genericObjectDeployment, err := od.scheme.New(od.gvk)
	if err != nil {
		panic(err)
	}
	switch object := genericObjectDeployment.(type) {
	case *corev1alpha1.ObjectDeployment:
		return &GenericObjectDeployment{
			*object,
		}
	case *corev1alpha1.ClusterObjectDeployment:
		return &GenericClusterObjectDeployment{
			*object,
		}
	}
	panic("Unsupported GVK")
}

func (od *GenericObjectDeploymentController) newOperandChild() genericObjectSet {
	object, err := od.scheme.New(od.childGvk)
	if err != nil {
		panic(err)
	}
	switch o := object.(type) {
	case *corev1alpha1.ObjectSet:
		return &GenericObjectSet{
			*o,
		}
	case *corev1alpha1.ClusterObjectSet:
		return &GenericClusterObjectSet{
			*o,
		}
	}
	panic("Unsupported child resource GVK")
}

func (c *GenericObjectDeploymentController) newOperandChildList() genericObjectSetList {
	childListGVK := c.childGvk.GroupVersion().
		WithKind(c.childGvk.Kind + "List")
	obj, err := c.scheme.New(childListGVK)
	if err != nil {
		panic(err)
	}

	switch o := obj.(type) {
	case *corev1alpha1.ObjectSetList:
		return &GenericObjectSetList{*o}
	case *corev1alpha1.ClusterObjectSetList:
		return &GenericClusterObjectSetList{*o}
	}
	panic("unsupported gvk")
}

func (od *GenericObjectDeploymentController) listObjectSetsByRevision(
	ctx context.Context,
	objectDeployment genericObjectDeployment,
) ([]genericObjectSet, error) {
	labelSelector := objectDeployment.GetSelector()
	objectSetSelector, err := metav1.LabelSelectorAsSelector(&labelSelector)
	if err != nil {
		return nil, fmt.Errorf("invalid selector: %w", err)
	}

	objectSetList := od.newOperandChildList()
	if err := od.client.List(
		ctx, objectSetList.ClientObjectList(),
		client.MatchingLabelsSelector{
			Selector: objectSetSelector,
		},
		client.InNamespace(objectDeployment.ClientObject().GetNamespace()),
	); err != nil {
		return nil, fmt.Errorf("listing ObjectSets: %w", err)
	}

	items := objectSetList.GetItems()

	// Ensure everything is sorted by revision.
	sort.Sort(objectSetsByRevision(items))
	return items, nil
}
