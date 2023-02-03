package hostedclusters

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/package-operator/internal/controllers/hostedclusters/hypershift/v1beta1"
	"package-operator.run/package-operator/internal/ownerhandling"
)

type HostedClusterController struct {
	client                  client.Client
	log                     logr.Logger
	scheme                  *runtime.Scheme
	remotePhasePackageImage string
	ownerStrategy           ownerStrategy
}

type ownerStrategy interface {
	SetControllerReference(owner, obj metav1.Object) error
	EnqueueRequestForOwner(
		ownerType client.Object, isController bool,
	) handler.EventHandler
}

func NewHostedClusterController(
	c client.Client, log logr.Logger, scheme *runtime.Scheme,
	remotePhasePackageImage string,
) *HostedClusterController {
	controller := &HostedClusterController{
		client:                  c,
		log:                     log,
		scheme:                  scheme,
		remotePhasePackageImage: remotePhasePackageImage,
		// Using Annotation Owner-Handling,
		// because Package objects will live in the hosted-clusters "execution" namespace.
		// e.g. clusters-my-cluster and not in the same Namespace as the HostedCluster object
		ownerStrategy: ownerhandling.NewAnnotation(scheme),
	}
	return controller
}

func (c *HostedClusterController) Reconcile(
	ctx context.Context, req ctrl.Request,
) (ctrl.Result, error) {
	log := c.log.WithValues("HostedCluster", req.String())
	defer log.Info("reconciled")

	ctx = logr.NewContext(ctx, log)
	hostedCluster := &v1beta1.HostedCluster{}
	if err := c.client.Get(ctx, req.NamespacedName, hostedCluster); err != nil {
		// Ignore not found errors on delete
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !meta.IsStatusConditionTrue(hostedCluster.Status.Conditions, v1beta1.HostedClusterAvailable) {
		log.Info("waiting for HostedCluster to become ready")
		return ctrl.Result{}, nil
	}

	desiredPkg := c.desiredPackage(hostedCluster)
	err := c.ownerStrategy.SetControllerReference(hostedCluster, desiredPkg)
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
		// update Job
		existingPkg.Spec.Image = desiredPkg.Spec.Image
		if err := c.client.Update(ctx, existingPkg); err != nil {
			return ctrl.Result{}, fmt.Errorf("deleting outdated Package: %w", err)
		}
	}

	return ctrl.Result{}, nil
}

func (c *HostedClusterController) desiredPackage(cluster *v1beta1.HostedCluster) *corev1alpha1.Package {
	pkg := &corev1alpha1.Package{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "remote-phase",
			Namespace: hostedClusterNamespace(cluster),
		},
		Spec: corev1alpha1.PackageSpec{
			Image: c.remotePhasePackageImage,
		},
	}
	return pkg
}

// From
// https://github.com/openshift/hypershift/blob/9c3e998b0b37bedce07163a197e0bf30339e627e/hypershift-operator/controllers/manifests/manifests.go#L13
func hostedClusterNamespace(cluster *v1beta1.HostedCluster) string {
	return fmt.Sprintf("%s-%s", cluster.Namespace, strings.ReplaceAll(cluster.Name, ".", "-"))
}

func (c *HostedClusterController) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1beta1.HostedCluster{}).
		Watches(&source.Kind{
			Type: &corev1alpha1.Package{},
		}, c.ownerStrategy.EnqueueRequestForOwner(
			&v1beta1.HostedCluster{}, true,
		)).
		Complete(c)
}
