package objectsetphases

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"pkg.package-operator.run/boxcutter"
	"pkg.package-operator.run/boxcutter/machinery"
	"pkg.package-operator.run/boxcutter/machinery/types"
	"pkg.package-operator.run/boxcutter/managedcache"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/flowcontrol"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/internal/adapters"
	"package-operator.run/internal/constants"
	"package-operator.run/internal/controllers"
	"package-operator.run/internal/controllers/boxcutterutil"
	internalprobing "package-operator.run/internal/probing"
)

// objectSetPhaseReconciler reconciles objects within a phase.
type objectSetPhaseReconciler struct {
	scheme                  *runtime.Scheme
	accessManager           managedcache.ObjectBoundAccessManager[client.Object]
	uncachedclient          client.Client
	phaseEngineFactory      boxcutterutil.PhaseEngineFactory
	lookupPreviousRevisions lookupPreviousRevisions
	ownerStrategy           boxcutterutil.OwnerStrategy
	backoff                 *flowcontrol.Backoff
}

func newObjectSetPhaseReconciler(
	scheme *runtime.Scheme,
	accessManager managedcache.ObjectBoundAccessManager[client.Object],
	uncachedClient client.Client,
	phaseEngineFactory boxcutterutil.PhaseEngineFactory,
	lookupPreviousRevisions lookupPreviousRevisions,
	ownerStrategy boxcutterutil.OwnerStrategy,
) *objectSetPhaseReconciler {
	var cfg objectSetPhaseReconcilerConfig

	cfg.Default()

	return &objectSetPhaseReconciler{
		scheme:                  scheme,
		accessManager:           accessManager,
		uncachedclient:          uncachedClient,
		phaseEngineFactory:      phaseEngineFactory,
		lookupPreviousRevisions: lookupPreviousRevisions,
		ownerStrategy:           ownerStrategy,
		backoff:                 cfg.GetBackoff(),
	}
}

type lookupPreviousRevisions func(
	ctx context.Context, owner controllers.PreviousOwner,
) ([]client.Object, error)

func (r *objectSetPhaseReconciler) Reconcile(
	ctx context.Context, objectSetPhase adapters.ObjectSetPhaseAccessor,
) (res ctrl.Result, err error) {
	defer r.backoff.GC()
	controllers.DeleteMappedConditions(ctx, objectSetPhase.GetStatusConditions())
	previous, err := r.lookupPreviousRevisions(ctx, objectSetPhase)
	if err != nil {
		return res, fmt.Errorf("lookup previous revisions: %w", err)
	}

	probe, err := internalprobing.Parse(
		ctx, objectSetPhase.GetAvailabilityProbes())
	if err != nil {
		return res, fmt.Errorf("parsing probes: %w", err)
	}

	objectsInPhase := []client.Object{}
	for _, object := range objectSetPhase.GetPhase().Objects {
		objectsInPhase = append(objectsInPhase, &object.Object)
	}
	cache, err := r.accessManager.GetWithUser(
		ctx,
		constants.StaticCacheOwner(),
		objectSetPhase.ClientObject(),
		objectsInPhase,
	)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("preparing cache: %w", err)
	}

	phaseEngine, err := r.phaseEngineFactory.New(cache)
	if err != nil {
		return res, err
	}

	apiPhase := objectSetPhase.GetPhase()
	// TODO Fix this!!!
	phaseObjects := make([]unstructured.Unstructured, 0)
	phaseReconcileOptions := make([]types.PhaseReconcileOption, 0)
	for i := range apiPhase.Objects {
		phaseObjects = append(phaseObjects, apiPhase.Objects[i].Object)
		labels := phaseObjects[i].GetLabels()
		if labels == nil {
			labels = map[string]string{}
		}
		labels[constants.DynamicCacheLabel] = "True"
		phaseObjects[i].SetLabels(labels)
		phaseReconcileOptions = append(phaseReconcileOptions, types.WithObjectReconcileOptions(
			&apiPhase.Objects[i].Object,
			boxcutterutil.TranslateCollisionProtection(apiPhase.Objects[i].CollisionProtection),
			types.WithProbe(types.ProgressProbeType, probe),
			boxcutter.WithPreviousOwners(previous),
		))
		if objectSetPhase.IsSpecPaused() {
			phaseReconcileOptions = append(phaseReconcileOptions, types.WithPaused{})
		}
	}

	result, err := phaseEngine.Reconcile(ctx,
		objectSetPhase.ClientObject(),
		objectSetPhase.GetStatusRevision(),
		types.Phase{
			Name:    apiPhase.Name,
			Objects: phaseObjects,
		},
		phaseReconcileOptions...,
	)

	target := &machinery.CreateCollisionError{}
	if errors.As(err, &target) {
		_, err := controllers.AddDynamicCacheLabel(ctx, r.uncachedclient, convertToUnstructured(target.Object()))
		if err != nil {
			return res, err
		}
		id := string(objectSetPhase.ClientObject().GetUID())

		r.backoff.Next(id, r.backoff.Clock.Now())

		return ctrl.Result{
			RequeueAfter: r.backoff.Get(id),
		}, nil
	}

	if controllers.IsExternalResourceNotFound(err) {
		id := string(objectSetPhase.ClientObject().GetUID())

		r.backoff.Next(id, r.backoff.Clock.Now())

		return ctrl.Result{
			RequeueAfter: r.backoff.Get(id),
		}, nil
	} else if err != nil {
		return res, err
	}

	actualObjects := make([]machinery.Object, 0, len(result.GetObjects()))
	for _, obj := range result.GetObjects() {
		actualObjects = append(actualObjects, obj.Object())
	}

	if err = mapConditions(actualObjects, objectSetPhase); err != nil {
		return res, err
	}

	objectSetPhase.SetStatusControllerOf(
		boxcutterutil.GetControllerOf(r.ownerStrategy, objectSetPhase.ClientObject(), result))

	if !result.IsComplete() {
		meta.SetStatusCondition(
			objectSetPhase.GetStatusConditions(), metav1.Condition{
				Type:               corev1alpha1.ObjectSetAvailable,
				Status:             metav1.ConditionFalse,
				Reason:             "ProbeFailure",
				Message:            result.String(),
				ObservedGeneration: objectSetPhase.ClientObject().GetGeneration(),
			})
		return res, nil
	}

	meta.SetStatusCondition(objectSetPhase.GetStatusConditions(), metav1.Condition{
		Type:               corev1alpha1.ObjectSetPhaseAvailable,
		Status:             metav1.ConditionTrue,
		Reason:             "Available",
		Message:            "Object is available and passes all probes.",
		ObservedGeneration: objectSetPhase.ClientObject().GetGeneration(),
	})

	return ctrl.Result{}, nil
}

func (r *objectSetPhaseReconciler) Teardown(
	ctx context.Context, objectSetPhase adapters.ObjectSetPhaseAccessor,
) (cleanupDone bool, err error) {
	// objectSetPhase is deleted with the `orphan` cascade option, so we don't need to delete the owned objects.
	if controllerutil.ContainsFinalizer(objectSetPhase.ClientObject(), "orphan") {
		return true, nil
	}

	objectsInPhase := []client.Object{}
	for _, object := range objectSetPhase.GetPhase().Objects {
		objectsInPhase = append(objectsInPhase, &object.Object)
	}
	cache, err := r.accessManager.GetWithUser(
		ctx,
		constants.StaticCacheOwner(),
		objectSetPhase.ClientObject(),
		objectsInPhase,
	)
	if err != nil {
		return false, fmt.Errorf("preparing cache: %w", err)
	}

	phaseEngine, err := r.phaseEngineFactory.New(cache)
	if err != nil {
		return false, err
	}
	apiPhase := objectSetPhase.GetPhase()
	phaseObjects := make([]unstructured.Unstructured, len(apiPhase.Objects))
	objectTearDownOptions := make([]types.PhaseTeardownOption, len(apiPhase.Objects))

	for i := range apiPhase.Objects {
		phaseObjects[i] = apiPhase.Objects[i].Object
		objectTearDownOptions[i] = types.WithObjectTeardownOptions(
			&apiPhase.Objects[i].Object,
		)
	}

	result, err := phaseEngine.Teardown(ctx,
		objectSetPhase.ClientObject(),
		objectSetPhase.GetStatusRevision(),
		types.Phase{
			Name:    apiPhase.Name,
			Objects: phaseObjects,
		},
		objectTearDownOptions...,
	)
	if err != nil {
		return false, err
	}
	if !result.IsComplete() {
		return false, nil
	}

	if err := r.accessManager.FreeWithUser(
		ctx,
		constants.StaticCacheOwner(),
		objectSetPhase.ClientObject(),
	); err != nil {
		return false, fmt.Errorf("freewithuser: %w", err)
	}

	return true, nil
}

type objectSetPhaseReconcilerConfig struct {
	controllers.BackoffConfig
}

func (c *objectSetPhaseReconcilerConfig) Option(opts ...objectSetPhaseReconcilerOption) {
	for _, opt := range opts {
		opt.ConfigureObjectSetPhaseReconciler(c)
	}
}

func (c *objectSetPhaseReconcilerConfig) Default() {
	c.BackoffConfig.Default()
}

type objectSetPhaseReconcilerOption interface {
	ConfigureObjectSetPhaseReconciler(*objectSetPhaseReconcilerConfig)
}

// Convert a  kubernetes object to an unstructured object.
func convertToUnstructured(obj machinery.Object) *unstructured.Unstructured {
	return obj.(*unstructured.Unstructured)
}

func mapConditions(actualObjects []machinery.Object, owner adapters.ObjectSetPhaseAccessor) error {
	for _, obj := range actualObjects {
		unstructuredObj := convertToUnstructured(obj)

		rawConditions, exist, err := unstructured.NestedFieldNoCopy(
			unstructuredObj.Object, "status", "conditions")
		if err != nil {
			return err
		}
		if !exist {
			return nil
		}

		j, err := json.Marshal(rawConditions)
		if err != nil {
			return err
		}
		var objectConditions []metav1.Condition
		if err := json.Unmarshal(j, &objectConditions); err != nil {
			return err
		}

		var conditionMappings []corev1alpha1.ConditionMapping
		objectsettemplatephase := owner.GetPhase()
		for _, objectsetobject := range objectsettemplatephase.Objects {
			if objectsetobject.Object.GetName() == obj.GetName() {
				conditionMappings = objectsetobject.ConditionMappings
			}
		}

		// Maps from object condition type to PKO condition type.
		conditionTypeMap := map[string]string{}
		for _, m := range conditionMappings {
			conditionTypeMap[m.SourceType] = m.DestinationType
		}
		for _, condition := range objectConditions {
			if condition.ObservedGeneration != 0 &&
				condition.ObservedGeneration != obj.GetGeneration() {
				// condition outdated
				continue
			}

			destType, ok := conditionTypeMap[condition.Type]
			if !ok {
				// condition not mapped
				continue
			}

			meta.SetStatusCondition(owner.GetStatusConditions(), metav1.Condition{
				Type:               destType,
				Status:             condition.Status,
				Reason:             condition.Reason,
				Message:            condition.Message,
				ObservedGeneration: owner.ClientObject().GetGeneration(),
			})
		}
	}

	return nil
}
