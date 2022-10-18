package objectdeployments

import (
	"context"
	"fmt"
	"sort"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

const (
	ObjectSetHashAnnotation = "package-operator.run/hash"
)

type reconciler interface {
	Reconcile(ctx context.Context, objectSetDeployment genericObjectDeployment) (ctrl.Result, error)
}

type GenericObjectDeploymentController struct {
	gvk                 schema.GroupVersionKind
	childGvk            schema.GroupVersionKind
	client              client.Client
	log                 logr.Logger
	scheme              *runtime.Scheme
	newObjectDeployment genericObjectDeploymentFactory
	newObjectSet        genericObjectSetFactory
	newObjectSetList    genericObjectSetListFactory
	reconciler          []reconciler
}

func newGenericObjectDeploymentController(
	gvk schema.GroupVersionKind,
	childGVK schema.GroupVersionKind,
	c client.Client, log logr.Logger, scheme *runtime.Scheme,
	newObjectDeployment genericObjectDeploymentFactory,
	newObjectSet genericObjectSetFactory,
	newObjectSetList genericObjectSetListFactory,
) *GenericObjectDeploymentController {
	controller := &GenericObjectDeploymentController{
		gvk:                 gvk,
		childGvk:            childGVK,
		client:              c,
		log:                 log,
		scheme:              scheme,
		newObjectDeployment: newObjectDeployment,
		newObjectSet:        newObjectSet,
		newObjectSetList:    newObjectSetList,
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
					newObjectSet: newObjectSet,
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
	return newGenericObjectDeploymentController(
		corev1alpha1.GroupVersion.WithKind("ObjectDeployment"),
		corev1alpha1.GroupVersion.WithKind("ObjectSet"),
		c,
		log,
		scheme,
		newGenericObjectDeployment,
		newGenericObjectSet,
		newGenericObjectSetList,
	)
}

func NewClusterObjectDeploymentController(
	c client.Client, log logr.Logger, scheme *runtime.Scheme,
) *GenericObjectDeploymentController {
	return newGenericObjectDeploymentController(
		corev1alpha1.GroupVersion.WithKind("ClusterObjectDeployment"),
		corev1alpha1.GroupVersion.WithKind("ClusterObjectSet"),
		c,
		log,
		scheme,
		newGenericClusterObjectDeployment,
		newGenericClusterObjectSet,
		newGenericClusterObjectSetList,
	)
}

func (od *GenericObjectDeploymentController) Reconcile(
	ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := od.log.WithValues("ObjectDeployment", req.String())
	defer log.Info("reconciled")
	ctx = logr.NewContext(ctx, log)
	objectDeployment := od.newObjectDeployment(od.scheme)
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
	return res, od.client.Status().Update(ctx, objectDeployment.ClientObject())
}

func (od *GenericObjectDeploymentController) SetupWithManager(mgr ctrl.Manager) error {
	objectDeployment := od.newObjectDeployment(od.scheme).ClientObject()
	objectSet := od.newObjectSet(od.scheme).ClientObject()

	return ctrl.NewControllerManagedBy(mgr).
		For(objectDeployment).
		Owns(objectSet).
		Complete(od)
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

	objectSetList := od.newObjectSetList(od.scheme)
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
	sort.Sort(objectSetsByRevisionAscending(items))
	return items, nil
}
