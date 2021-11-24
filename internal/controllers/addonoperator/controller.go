package addonoperator

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"

	"github.com/go-logr/logr"
	configv1 "github.com/openshift/api/config/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	ctrl "sigs.k8s.io/controller-runtime"

	addonsv1alpha1 "github.com/openshift/addon-operator/apis/addons/v1alpha1"
	"github.com/openshift/addon-operator/internal/ocm"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	defaultAddonOperatorRequeueTime = time.Minute
)

type AddonOperatorReconciler struct {
	client.Client
	UncachedClient     client.Client
	Log                logr.Logger
	Scheme             *runtime.Scheme
	GlobalPauseManager globalPauseManager
	OCMClientManager   ocmClientManager
}

func (r *AddonOperatorReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&addonsv1alpha1.AddonOperator{}).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		Watches(source.Func(enqueueAddonOperator),
			&handler.EnqueueRequestForObject{}). // initial enqueue for creating the object
		Complete(r)
}

func enqueueAddonOperator(ctx context.Context, h handler.EventHandler,
	q workqueue.RateLimitingInterface, p ...predicate.Predicate) error {
	q.Add(reconcile.Request{NamespacedName: types.NamespacedName{
		Name: addonsv1alpha1.DefaultAddonOperatorName,
	}})
	return nil
}

func (r *AddonOperatorReconciler) Reconcile(
	ctx context.Context, req ctrl.Request) (ctrl.Result, error) {

	log := r.Log.WithValues("addon-operator", req.NamespacedName.String())

	addonOperator := &addonsv1alpha1.AddonOperator{}
	err := r.Get(ctx, client.ObjectKey{
		Name: addonsv1alpha1.DefaultAddonOperatorName,
	}, addonOperator)
	// Create default AddonOperator object if it doesn't exist
	if apierrors.IsNotFound(err) {
		log.Info("default AddonOperator not found")
		return ctrl.Result{}, r.handleAddonOperatorCreation(ctx, log)
	}
	if err != nil {
		return ctrl.Result{}, err
	}

	if err := r.handleGlobalPause(ctx, addonOperator); err != nil {
		return ctrl.Result{}, fmt.Errorf("handling global pause: %w", err)
	}

	if err := r.handleOCMClient(ctx, addonOperator); err != nil {
		return ctrl.Result{}, fmt.Errorf("handling OCM client: %w", err)
	}

	// TODO: This is where all the checking / validation happens
	// for "in-depth" status reporting

	err = r.reportAddonOperatorReadinessStatus(ctx, addonOperator)
	if err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{RequeueAfter: defaultAddonOperatorRequeueTime}, nil
}

// Creates an OCM API client and injects it into the OCM Client Manager for distribution.
func (r *AddonOperatorReconciler) handleOCMClient(
	ctx context.Context, addonOperator *addonsv1alpha1.AddonOperator) error {
	if addonOperator.Spec.OCM == nil {
		return nil
	}

	cv := &configv1.ClusterVersion{}
	if err := r.Get(ctx, client.ObjectKey{Name: "version"}, cv); err != nil {
		return fmt.Errorf("getting clusterversion: %w", err)
	}

	secret := &corev1.Secret{}
	if err := r.UncachedClient.Get(ctx, client.ObjectKey{
		Name:      addonOperator.Spec.OCM.Secret.Name,
		Namespace: addonOperator.Spec.OCM.Secret.Namespace,
	}, secret); err != nil {
		return fmt.Errorf("getting ocm secret: %w", err)
	}

	accessToken, err := accessTokenFromDockerConfig(secret.Data[corev1.DockerConfigJsonKey])
	if err != nil {
		return fmt.Errorf("extracting access token from .dockerconfigjson: %w", err)
	}

	c := ocm.NewClient(
		ocm.WithEndpoint(addonOperator.Spec.OCM.Endpoint),
		ocm.WithAccessToken(accessToken),
		ocm.WithClusterID(string(cv.Spec.ClusterID)),
	)
	if err := r.OCMClientManager.InjectOCMClient(ctx, c); err != nil {
		return fmt.Errorf("injecting ocm client: %w", err)
	}
	return nil
}

func (r *AddonOperatorReconciler) handleGlobalPause(
	ctx context.Context, addonOperator *addonsv1alpha1.AddonOperator) error {
	// Check if addonoperator.spec.paused == true
	if addonOperator.Spec.Paused {
		// Check if Paused condition has already been reported
		if meta.IsStatusConditionTrue(addonOperator.Status.Conditions,
			addonsv1alpha1.AddonOperatorPaused) {
			return nil
		}
		if err := r.GlobalPauseManager.EnableGlobalPause(ctx); err != nil {
			return fmt.Errorf("setting global pause: %w", err)
		}
		if err := r.reportAddonOperatorPauseStatus(ctx, addonOperator); err != nil {
			return fmt.Errorf("report AddonOperator paused: %w", err)
		}
		return nil
	}

	// Unpause only if the current reported condition is Paused
	if !meta.IsStatusConditionTrue(addonOperator.Status.Conditions,
		addonsv1alpha1.AddonOperatorPaused) {
		return nil
	}
	if err := r.GlobalPauseManager.DisableGlobalPause(ctx); err != nil {
		return fmt.Errorf("removing global pause: %w", err)
	}
	if err := r.removeAddonOperatorPauseCondition(ctx, addonOperator); err != nil {
		return fmt.Errorf("remove AddonOperator paused: %w", err)
	}
	return nil
}

func accessTokenFromDockerConfig(dockerConfigJson []byte) (string, error) {
	dockerConfig := map[string]interface{}{}
	if err := json.Unmarshal(dockerConfigJson, &dockerConfig); err != nil {
		return "", fmt.Errorf("unmarshalling docker config json: %w", err)
	}

	accessToken, ok, err := unstructured.NestedString(
		dockerConfig, "auths", "cloud.openshift.com", "auth")
	if err != nil {
		return "", fmt.Errorf("accessing cloud.openshift.com auth key: %w", err)
	}
	if !ok {
		return "", fmt.Errorf("missing token for cloud.openshift.com")
	}
	return accessToken, nil
}
