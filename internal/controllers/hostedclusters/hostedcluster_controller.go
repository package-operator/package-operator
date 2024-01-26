package hostedclusters

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/internal/controllers/hostedclusters/hypershift/v1beta1"
	"package-operator.run/internal/ownerhandling"
)

type HostedClusterController struct {
	client                  client.Client
	log                     logr.Logger
	scheme                  *runtime.Scheme
	remotePhasePackageImage string
	ownerStrategy           ownerStrategy

	remotePhaseAffinity    *corev1.Affinity
	remotePhaseTolerations []corev1.Toleration
}

type ownerStrategy interface {
	SetControllerReference(owner, obj metav1.Object) error
	EnqueueRequestForOwner(
		ownerType client.Object, mapper meta.RESTMapper, isController bool,
	) handler.EventHandler
}

func NewHostedClusterController(
	c client.Client, log logr.Logger, scheme *runtime.Scheme,
	remotePhasePackageImage string,
	remotePhaseAffinity *corev1.Affinity,
	remotePhaseTolerations []corev1.Toleration,
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

		remotePhaseAffinity:    remotePhaseAffinity,
		remotePhaseTolerations: remotePhaseTolerations,
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

	if !hostedCluster.DeletionTimestamp.IsZero() {
		log.Info("HostedCluster is deleting")
		return ctrl.Result{}, nil
	}

	if !meta.IsStatusConditionTrue(hostedCluster.Status.Conditions, v1beta1.HostedClusterAvailable) {
		log.Info("waiting for HostedCluster to become ready")
		return ctrl.Result{}, nil
	}

	desiredPkg, err := c.desiredPackage(hostedCluster)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("building desired package: %w", err)
	}
	if err = c.ownerStrategy.SetControllerReference(hostedCluster, desiredPkg); err != nil {
		return ctrl.Result{}, fmt.Errorf("setting controller reference: %w", err)
	}

	existingPkg := &corev1alpha1.Package{}
	err = c.client.Get(ctx, client.ObjectKeyFromObject(desiredPkg), existingPkg)
	if err != nil && errors.IsNotFound(err) {
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

func (c *HostedClusterController) desiredPackage(cluster *v1beta1.HostedCluster) (*corev1alpha1.Package, error) {
	pkg := &corev1alpha1.Package{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "remote-phase",
			Namespace: v1beta1.HostedClusterNamespace(*cluster),
		},
		Spec: corev1alpha1.PackageSpec{
			Image:     c.remotePhasePackageImage,
			Component: "remote-phase",
		},
	}

	config := map[string]any{}
	if c.remotePhaseAffinity != nil {
		config["affinity"] = c.remotePhaseAffinity
	}
	if c.remotePhaseTolerations != nil {
		config["tolerations"] = c.remotePhaseTolerations
	}
	if len(config) > 0 {
		configJSON, err := json.Marshal(config)
		if err != nil {
			return nil, fmt.Errorf("marshalling config: %w", err)
		}

		pkg.Spec.Config = &runtime.RawExtension{Raw: configJSON}
	}

	return pkg, nil
}

func (c *HostedClusterController) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1beta1.HostedCluster{}).
		WatchesRawSource(source.Kind(mgr.GetCache(), &corev1alpha1.Package{}), c.ownerStrategy.EnqueueRequestForOwner(
			&v1beta1.HostedCluster{}, mgr.GetRESTMapper(), true,
		)).
		Complete(c)
}
