package hostedclusters

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/package-operator/internal/controllers/hostedclusters/hypershift/v1alpha1"
)

type HostedClusterController struct {
	client client.Client
	log    logr.Logger
	scheme *runtime.Scheme
	image  string
}

func NewHostedClusterController(
	c client.Client, log logr.Logger, scheme *runtime.Scheme, image string,
) *HostedClusterController {
	controller := &HostedClusterController{
		client: c,
		log:    log,
		scheme: scheme,
		image:  image,
	}
	return controller
}

func (c *HostedClusterController) Reconcile(
	ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := c.log.WithValues("HostedCluster", req.String())
	defer log.Info("reconciled")
	ctx = logr.NewContext(ctx, log)
	hostedCluster := &v1alpha1.HostedCluster{}
	if err := c.client.Get(ctx, req.NamespacedName, hostedCluster); err != nil {
		// Ignore not found errors on delete
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	ok := isHostedClusterReady(hostedCluster)
	if !ok {
		return ctrl.Result{}, nil
	}

	desiredPkg := c.desiredPackage(hostedCluster)
	err := controllerutil.SetControllerReference(hostedCluster, desiredPkg, c.scheme)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("setting controller reference: %w", err)
	}

	existingPkg := &corev1alpha1.Package{}
	if err := c.client.Get(ctx, client.ObjectKeyFromObject(desiredPkg), existingPkg); err != nil && errors.IsNotFound(err) {
		if err := c.client.Create(ctx, desiredPkg); err != nil {
			return ctrl.Result{}, fmt.Errorf("creating Package: %w", err)
		}
		return ctrl.Result{}, nil
	} else if err != nil {
		return ctrl.Result{}, fmt.Errorf("getting Package: %w", err)
	}

	if existingPkg.Spec.Image != desiredPkg.Spec.Image {
		// re-create job
		if err := c.client.Delete(ctx, existingPkg); err != nil {
			return ctrl.Result{}, fmt.Errorf("deleting outdated Package: %w", err)
		}
		if err := c.client.Create(ctx, desiredPkg); err != nil {
			return ctrl.Result{}, fmt.Errorf("creating Package: %w", err)
		}
	}

	return ctrl.Result{}, nil
}

func isHostedClusterReady(hc *v1alpha1.HostedCluster) bool {
	ready := false

	conds := hc.Status.Conditions
	for _, cond := range conds {
		if cond.Type == v1alpha1.HostedClusterAvailable {
			if cond.Status == metav1.ConditionTrue {
				ready = true
			}
			break
		}
	}
	return ready
}

func (c *HostedClusterController) desiredPackage(cluster *v1alpha1.HostedCluster) *corev1alpha1.Package {
	pkg := &corev1alpha1.Package{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cluster.Name + "_remote_phase_manager",
			Namespace: cluster.Namespace,
		},
		Spec: corev1alpha1.PackageSpec{
			Image: c.image,
		},
	}
	return pkg
}

func (c *HostedClusterController) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.HostedCluster{}).
		Owns(&corev1alpha1.Package{}).
		Complete(c)
}
