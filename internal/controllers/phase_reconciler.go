package controllers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/go-logr/logr"
	apimachineryerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/openapi3"
	"k8s.io/client-go/util/csaupgrade"
	"k8s.io/kube-openapi/pkg/schemaconv"
	"k8s.io/kube-openapi/pkg/util"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/structured-merge-diff/v4/fieldpath"
	"sigs.k8s.io/structured-merge-diff/v4/typed"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
	"package-operator.run/internal/preflight"
	"package-operator.run/pkg/probing"
)

// PhaseReconciler reconciles objects within a ObjectSet phase.
type PhaseReconciler struct {
	scheme *runtime.Scheme
	// just specify a writer, because we don't want to ever read from another source than
	// the dynamic cache that is managed to hold the objects we are reconciling.
	writer           client.Writer
	dynamicCache     dynamicCache
	uncachedClient   client.Reader
	ownerStrategy    ownerStrategy
	adoptionChecker  adoptionChecker
	patcher          patcher
	preflightChecker preflightChecker
	discoveryClient  discovery.DiscoveryInterface
}

type ownerStrategy interface {
	IsController(owner, obj metav1.Object) bool
	IsOwner(owner, obj metav1.Object) bool
	ReleaseController(obj metav1.Object)
	RemoveOwner(owner, obj metav1.Object)
	SetOwnerReference(owner, obj metav1.Object) error
	SetControllerReference(owner, obj metav1.Object) error
	OwnerPatch(owner metav1.Object) ([]byte, error)
	HasController(obj metav1.Object) bool
}

type adoptionChecker interface {
	Check(
		ctx context.Context, owner PhaseObjectOwner, obj client.Object,
		previous []PreviousObjectSet,
		collisionProtection corev1alpha1.CollisionProtection,
	) (needsAdoption bool, err error)
}

type patcher interface {
	Patch(
		ctx context.Context,
		desiredObj, currentObj, updatedObj *unstructured.Unstructured,
	) error
}

type dynamicCache interface {
	client.Reader
	Watch(
		ctx context.Context, owner client.Object, obj runtime.Object,
	) error
}

type preflightChecker interface {
	Check(
		ctx context.Context, owner, obj client.Object,
	) (violations []preflight.Violation, err error)
}

func NewPhaseReconciler(
	scheme *runtime.Scheme,
	writer client.Writer,
	dynamicCache dynamicCache,
	uncachedClient client.Reader,
	ownerStrategy ownerStrategy,
	preflightChecker preflightChecker,
	discoveryClient discovery.DiscoveryInterface,
) *PhaseReconciler {
	return &PhaseReconciler{
		scheme:           scheme,
		writer:           writer,
		dynamicCache:     dynamicCache,
		uncachedClient:   uncachedClient,
		ownerStrategy:    ownerStrategy,
		adoptionChecker:  &defaultAdoptionChecker{ownerStrategy: ownerStrategy, scheme: scheme},
		patcher:          &defaultPatcher{writer: writer},
		preflightChecker: preflightChecker,
		discoveryClient:  discoveryClient,
	}
}

type PhaseObjectOwner interface {
	ClientObject() client.Object
	GetRevision() int64
	GetConditions() *[]metav1.Condition
	IsPaused() bool
}

func newRecordingProbe(name string, probe probing.Prober) recordingProbe {
	return recordingProbe{
		name:  name,
		probe: probe,
	}
}

type recordingProbe struct {
	name     string
	probe    probing.Prober
	failures []string
}

func (p *recordingProbe) Probe(obj *unstructured.Unstructured) {
	ok, msg := p.probe.Probe(obj)
	if ok {
		return
	}
	p.recordForObj(obj, msg)
}

func (p *recordingProbe) RecordMissingObject(obj *unstructured.Unstructured) {
	p.recordForObj(obj, "not found")
}

func (p *recordingProbe) recordForObj(obj *unstructured.Unstructured, msg string) {
	gvk := obj.GroupVersionKind()
	msg = fmt.Sprintf("%s %s %s/%s: %s", gvk.Group, gvk.Kind, obj.GetNamespace(), obj.GetName(), msg)

	p.failures = append(p.failures, msg)
}

func (p *recordingProbe) Result() ProbingResult {
	if len(p.failures) == 0 {
		return ProbingResult{}
	}

	return ProbingResult{
		PhaseName:    p.name,
		FailedProbes: p.failures,
	}
}

type ProbingResult struct {
	PhaseName    string
	FailedProbes []string
}

func (e *ProbingResult) IsZero() bool {
	if e == nil || len(e.PhaseName) == 0 && len(e.FailedProbes) == 0 {
		return true
	}
	return false
}

func (e *ProbingResult) StringWithoutPhase() string {
	return strings.Join(e.FailedProbes, ", ")
}

func (e *ProbingResult) String() string {
	return fmt.Sprintf("Phase %q failed: %s",
		e.PhaseName, e.StringWithoutPhase())
}

func (r *PhaseReconciler) ReconcilePhase(
	ctx context.Context, owner PhaseObjectOwner,
	phase corev1alpha1.ObjectSetTemplatePhase,
	probe probing.Prober, previous []PreviousObjectSet,
) (actualObjects []client.Object, res ProbingResult, err error) {
	desiredObjects := make([]unstructured.Unstructured, len(phase.Objects))
	for i, phaseObject := range phase.Objects {
		desired, err := r.desiredObject(ctx, owner, phaseObject)
		if err != nil {
			return nil, res, fmt.Errorf("%s: %w", phaseObject, err)
		}
		desiredObjects[i] = *desired
	}

	violations, err := preflight.CheckAllInPhase(
		ctx, r.preflightChecker, owner.ClientObject(), phase, desiredObjects)
	if err != nil {
		return nil, res, err
	}
	if len(violations) > 0 {
		return nil, res, &preflight.Error{
			Violations: violations,
		}
	}

	meta.RemoveStatusCondition(owner.GetConditions(), corev1alpha1.ObjectSetDiverged)
	rec := newRecordingProbe(phase.Name, probe)
	for i, phaseObject := range phase.Objects {
		desiredObj := &desiredObjects[i]
		actualObj, err := r.reconcilePhaseObject(ctx, owner, phaseObject, desiredObj, previous)
		if apimachineryerrors.IsNotFound(err) {
			// Don't error, just observe.
			rec.RecordMissingObject(desiredObj)
			continue
		}
		if err != nil {
			return nil, res, fmt.Errorf("%s: %w", phaseObject, err)
		}
		actualObjects = append(actualObjects, actualObj)

		rec.Probe(actualObj)
	}

	for _, obj := range phase.ExternalObjects {
		observedObj, err := r.observeExternalObject(ctx, owner, obj)
		if err != nil {
			return nil, res, fmt.Errorf("%s: %w", obj, err)
		}

		rec.Probe(observedObj)
	}

	return actualObjects, rec.Result(), nil
}

func (r *PhaseReconciler) observeExternalObject(
	ctx context.Context,
	owner PhaseObjectOwner,
	extObj corev1alpha1.ObjectSetObject,
) (*unstructured.Unstructured, error) {
	var (
		obj      = &extObj.Object
		ownerObj = owner.ClientObject()
	)

	if len(obj.GetNamespace()) == 0 {
		obj.SetNamespace(ownerObj.GetNamespace())
	}

	// Watch this external object while the owner of the phase is active
	if err := r.dynamicCache.Watch(ctx, ownerObj, obj); err != nil {
		return nil, fmt.Errorf("watching external object: %w", err)
	}

	var (
		key      = client.ObjectKeyFromObject(obj)
		observed = obj.DeepCopy()
	)

	if err := r.dynamicCache.Get(ctx, key, observed); apimachineryerrors.IsNotFound(err) {
		if err := r.uncachedClient.Get(ctx, key, obj); apimachineryerrors.IsNotFound(err) {
			return nil, NewExternalResourceNotFoundError(obj)
		} else if err != nil {
			return nil, fmt.Errorf("retrieving external object: %w", err)
		}

		// Update object to ensure it is part of our cache and we get events to reconcile.
		if observed, err = AddDynamicCacheLabel(ctx, r.writer, observed); err != nil {
			return nil, fmt.Errorf("adding cache label: %w", err)
		}
	} else if err != nil {
		return nil, fmt.Errorf("retrieving external object: %w", err)
	}

	if err := r.ownerStrategy.SetOwnerReference(ownerObj, observed); err != nil {
		return nil, fmt.Errorf("setting owner reference: %w", err)
	}
	ownerPatch, err := r.ownerStrategy.OwnerPatch(observed)
	if err != nil {
		return nil, fmt.Errorf("determining owner patch: %w", err)
	}
	if err := r.writer.Patch(ctx, observed, client.RawPatch(
		types.MergePatchType, ownerPatch,
	)); err != nil {
		return nil, fmt.Errorf("patching object ownership: %w", err)
	}

	if err := mapConditions(ctx, owner, extObj.ConditionMappings, observed); err != nil {
		return nil, err
	}

	return observed, nil
}

func (r *PhaseReconciler) TeardownPhase(
	ctx context.Context, owner PhaseObjectOwner,
	phase corev1alpha1.ObjectSetTemplatePhase,
) (cleanupDone bool, err error) {
	var cleanupCounter int
	objectsToCleanup := len(phase.Objects) + len(phase.ExternalObjects)
	for _, phaseObject := range phase.Objects {
		done, err := r.teardownPhaseObject(ctx, owner, phaseObject)
		if err != nil {
			return false, err
		}

		if done {
			cleanupCounter++
		}
	}

	for _, extObj := range phase.ExternalObjects {
		done, err := r.teardownExternalObject(ctx, owner, extObj)
		if err != nil {
			return false, fmt.Errorf("tearing down external object: %w", err)
		}

		if done {
			cleanupCounter++
		}
	}

	return cleanupCounter == objectsToCleanup, nil
}

func (r *PhaseReconciler) teardownPhaseObject(
	ctx context.Context, owner PhaseObjectOwner,
	phaseObject corev1alpha1.ObjectSetObject,
) (cleanupDone bool, err error) {
	log := logr.FromContextOrDiscard(ctx)

	desiredObj, err := r.desiredObject(ctx, owner, phaseObject)
	if err != nil {
		return false, fmt.Errorf("building desired object: %w", err)
	}

	// Preflight checker during teardown prevents the deletion of resources in different namespaces and
	// unblocks teardown when APIs have been removed.
	if v, err := r.preflightChecker.Check(ctx, owner.ClientObject(), desiredObj); err != nil {
		return false, fmt.Errorf("running preflight validation: %w", err)
	} else if len(v) > 0 {
		return true, nil
	}

	// Ensure to watch this type of object, also during teardown!
	// If the controller was restarted or crashed during deletion, we might not have a cache in memory anymore.
	if err := r.dynamicCache.Watch(
		ctx, owner.ClientObject(), desiredObj); err != nil {
		return false, fmt.Errorf("watching new resource: %w", err)
	}

	currentObj := desiredObj.DeepCopy()
	err = r.uncachedClient.Get(
		ctx, client.ObjectKeyFromObject(desiredObj), currentObj)
	if err != nil && apimachineryerrors.IsNotFound(err) {
		// No matter who the owner of this object is,
		// it's already gone.
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("getting object for teardown: %w", err)
	}

	if !r.ownerStrategy.IsController(owner.ClientObject(), currentObj) {
		if !r.ownerStrategy.IsOwner(owner.ClientObject(), currentObj) {
			return true, nil
		}

		// This object is controlled by someone else
		// so we don't have to delete it for cleanup.
		// But we still want to remove ourselves as potential owner.
		r.ownerStrategy.RemoveOwner(owner.ClientObject(), currentObj)
		if err := r.writer.Update(ctx, currentObj); err != nil {
			return false, fmt.Errorf("removing owner reference: %w", err)
		}
		return true, nil
	}

	log.Info("deleting managed object",
		"apiVersion", currentObj.GetAPIVersion(),
		"kind", currentObj.GroupVersionKind().Kind,
		"namespace", currentObj.GetNamespace(),
		"name", currentObj.GetName())

	err = r.writer.Delete(ctx, currentObj)
	if err != nil && apimachineryerrors.IsNotFound(err) {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("deleting object for teardown: %w", err)
	}

	return false, nil
}

func (r *PhaseReconciler) teardownExternalObject(
	ctx context.Context, owner PhaseObjectOwner,
	extObj corev1alpha1.ObjectSetObject,
) (cleanupDone bool, err error) {
	var (
		obj      = &extObj.Object
		ownerObj = owner.ClientObject()
	)

	if len(obj.GetNamespace()) == 0 {
		obj.SetNamespace(ownerObj.GetNamespace())
	}

	// handles the case where cache must be repaired after objectset
	// was initially reconciled
	if err := r.dynamicCache.Watch(ctx, ownerObj, obj); err != nil {
		return false, fmt.Errorf("watching external object: %w", err)
	}

	var (
		key      = client.ObjectKeyFromObject(obj)
		observed = obj.DeepCopy()
	)
	if err := r.dynamicCache.Get(ctx, key, observed); apimachineryerrors.IsNotFound(err) {
		if err := r.uncachedClient.Get(ctx, key, obj); apimachineryerrors.IsNotFound(err) {
			// external object does not exist therefore no action is needed
			return true, nil
		} else if err != nil {
			return false, fmt.Errorf("retrieving external object: %w", err)
		}

		if _, err = RemoveDynamicCacheLabel(ctx, r.writer, observed); err != nil {
			return false, fmt.Errorf("removing cache label: %w", err)
		}
	} else if err != nil {
		return false, fmt.Errorf("retrieving external object: %w", err)
	}

	r.ownerStrategy.RemoveOwner(ownerObj, obj)
	if err := r.writer.Update(ctx, obj); err != nil {
		return false, fmt.Errorf("removing owner reference: %w", err)
	}

	return true, nil
}

func (r *PhaseReconciler) reconcilePhaseObject(
	ctx context.Context, owner PhaseObjectOwner,
	phaseObject corev1alpha1.ObjectSetObject,
	desiredObj *unstructured.Unstructured,
	previous []PreviousObjectSet,
) (actualObj *unstructured.Unstructured, err error) {
	// Set owner reference
	if err := r.ownerStrategy.SetControllerReference(owner.ClientObject(), desiredObj); err != nil {
		return nil, fmt.Errorf("set controller reference: %w", err)
	}

	// Ensure to watch this type of object.
	if err := r.dynamicCache.Watch(
		ctx, owner.ClientObject(), desiredObj); err != nil {
		return nil, fmt.Errorf("watching new resource: %w", err)
	}

	if owner.IsPaused() {
		actualObj = desiredObj.DeepCopy()
		if err := r.dynamicCache.Get(ctx, client.ObjectKeyFromObject(desiredObj), actualObj); err != nil {
			return nil, fmt.Errorf("looking up object while paused: %w", err)
		}

		diverged, mfields, err := hasDiverged(owner, r.ownerStrategy, r.discoveryClient, desiredObj, actualObj)
		if err != nil {
			return nil, fmt.Errorf("checking for divergence: %w", err)
		}
		if diverged {
			msg := fmt.Sprintf(
				"Object %s %s/%s has been modified by:",
				desiredObj.GroupVersionKind().Kind,
				desiredObj.GetNamespace(),
				desiredObj.GetName(),
			)
			for m, fields := range mfields {
				msg += fmt.Sprintf("\n- %q: %s", m, strings.ReplaceAll(fields.String(), "\n", ", "))
			}

			meta.SetStatusCondition(owner.GetConditions(), metav1.Condition{
				Type:               corev1alpha1.ObjectSetDiverged,
				Status:             "True",
				Reason:             "Diverged",
				Message:            msg,
				ObservedGeneration: owner.ClientObject().GetGeneration(),
			})
		}

		return actualObj, nil
	}

	if actualObj, err = r.reconcileObject(ctx, owner, desiredObj, previous, phaseObject.CollisionProtection); err != nil {
		return nil, err
	}

	if err = mapConditions(ctx, owner, phaseObject.ConditionMappings, actualObj); err != nil {
		return nil, err
	}

	return actualObj, nil
}

func mapConditions(
	_ context.Context, owner PhaseObjectOwner,
	conditionMappings []corev1alpha1.ConditionMapping,
	actualObject *unstructured.Unstructured,
) error {
	if len(conditionMappings) == 0 {
		return nil
	}

	rawConditions, exist, err := unstructured.NestedFieldNoCopy(
		actualObject.Object, "status", "conditions")
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

	// Maps from object condition type to PKO condition type.
	conditionTypeMap := map[string]string{}
	for _, m := range conditionMappings {
		conditionTypeMap[m.SourceType] = m.DestinationType
	}
	for _, condition := range objectConditions {
		if condition.ObservedGeneration != 0 &&
			condition.ObservedGeneration != actualObject.GetGeneration() {
			// condition outdated
			continue
		}

		destType, ok := conditionTypeMap[condition.Type]
		if !ok {
			// condition not mapped
			continue
		}

		meta.SetStatusCondition(owner.GetConditions(), metav1.Condition{
			Type:               destType,
			Status:             condition.Status,
			Reason:             condition.Reason,
			Message:            condition.Message,
			ObservedGeneration: owner.ClientObject().GetGeneration(),
		})
	}
	return nil
}

// Builds an object as specified in a phase.
// Includes system labels, namespace and owner reference.
func (r *PhaseReconciler) desiredObject(
	_ context.Context, owner PhaseObjectOwner,
	phaseObject corev1alpha1.ObjectSetObject,
) (desiredObj *unstructured.Unstructured, err error) {
	desiredObj = phaseObject.Object.DeepCopy()

	// Default namespace to the owners namespace
	if len(desiredObj.GetNamespace()) == 0 {
		desiredObj.SetNamespace(
			owner.ClientObject().GetNamespace())
	}

	// Set cache label
	labels := desiredObj.GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}
	labels[DynamicCacheLabel] = "True"

	if ownerLabels := owner.ClientObject().GetLabels(); ownerLabels != nil {
		if pkgLabel, ok := ownerLabels[manifestsv1alpha1.PackageLabel]; ok {
			labels[manifestsv1alpha1.PackageLabel] = pkgLabel
		}
		if pkgInstanceLabel, ok := ownerLabels[manifestsv1alpha1.PackageInstanceLabel]; ok {
			labels[manifestsv1alpha1.PackageInstanceLabel] = pkgInstanceLabel
		}
	}

	desiredObj.SetLabels(labels)

	setObjectRevision(desiredObj, owner.GetRevision())

	return desiredObj, nil
}

type ObjectSetOrPhase interface {
	ClientObject() client.Object
	GetConditions() *[]metav1.Condition
	UpdateStatusPhase()
}

func UpdateObjectSetOrPhaseStatusFromError(
	ctx context.Context, objectSetOrPhase ObjectSetOrPhase,
	reconcileErr error, updateStatus func(ctx context.Context) error,
) (res ctrl.Result, err error) {
	var preflightError *preflight.Error
	if errors.As(reconcileErr, &preflightError) {
		meta.SetStatusCondition(objectSetOrPhase.GetConditions(), metav1.Condition{
			Type:               corev1alpha1.ObjectSetAvailable,
			Status:             metav1.ConditionFalse,
			ObservedGeneration: objectSetOrPhase.ClientObject().GetGeneration(),
			Reason:             "PreflightError",
			Message:            preflightError.Error(),
		})
		// Retry every once and a while to automatically unblock, if the preflight check issue has been cleared.
		res.RequeueAfter = DefaultGlobalMissConfigurationRetry
		return res, updateStatus(ctx)
	}

	if IsAdoptionRefusedError(reconcileErr) {
		meta.SetStatusCondition(objectSetOrPhase.GetConditions(), metav1.Condition{
			Type:               corev1alpha1.ObjectSetAvailable,
			Status:             metav1.ConditionFalse,
			ObservedGeneration: objectSetOrPhase.ClientObject().GetGeneration(),
			Reason:             "CollisionDetected",
			Message:            reconcileErr.Error(),
		})
		// Retry every once and a while to automatically unblock, if the conflicting resource has been deleted.
		res.RequeueAfter = DefaultGlobalMissConfigurationRetry
		return res, updateStatus(ctx)
	}

	// if we don't handle the error in any special way above,
	// just return it unchanged.
	return res, reconcileErr
}

type CommonObjectPhaseError struct {
	OwnerKey, ObjectKey client.ObjectKey
	OwnerGVK, ObjectGVK schema.GroupVersionKind
}

// This error is returned when a Phase contains objects
// that are not owned by a previous revision.
// Previous revisions of an Phase have to be declared in .spec.previousRevisions.
type ObjectNotOwnedByPreviousRevisionError struct {
	CommonObjectPhaseError
}

func (e *ObjectNotOwnedByPreviousRevisionError) Error() string {
	return fmt.Sprintf("refusing adoption, object %s %s not owned by previous revision", e.ObjectGVK, e.ObjectKey)
}

// This error is returned when a Phase tries to adopt an object
// where the revision number is not increasing.
type RevisionCollisionError struct {
	CommonObjectPhaseError
}

func (e *RevisionCollisionError) Error() string {
	return fmt.Sprintf("refusing adoption, revision collision on %s %s", e.ObjectGVK, e.ObjectKey)
}

func (r *PhaseReconciler) reconcileObject(
	ctx context.Context, owner PhaseObjectOwner,
	desiredObj *unstructured.Unstructured, previous []PreviousObjectSet,
	collisionProtection corev1alpha1.CollisionProtection,
) (actualObj *unstructured.Unstructured, err error) {
	objKey := client.ObjectKeyFromObject(desiredObj)
	currentObj := desiredObj.DeepCopy()
	err = r.dynamicCache.Get(ctx, objKey, currentObj)
	if err != nil && !apimachineryerrors.IsNotFound(err) {
		return nil, fmt.Errorf("getting %s: %w", desiredObj.GroupVersionKind(), err)
	}
	if apimachineryerrors.IsNotFound(err) {
		err = r.uncachedClient.Get(ctx, objKey, currentObj)
		if err != nil && !apimachineryerrors.IsNotFound(err) {
			return nil, fmt.Errorf("getting %s: %w", desiredObj.GroupVersionKind(), err)
		}
	}
	if apimachineryerrors.IsNotFound(err) {
		// The object is not yet present on the cluster,
		// just create it using desired state!
		err := r.writer.Patch(ctx, desiredObj, client.Apply, client.FieldOwner(FieldOwner))
		if apimachineryerrors.IsAlreadyExists(err) {
			// object already exists, but was not in our cache.
			// get object via uncached client directly from the API server.
			if err := r.uncachedClient.Get(ctx, objKey, currentObj); err != nil {
				return nil, fmt.Errorf("getting %s from uncached client: %w", desiredObj.GroupVersionKind(), err)
			}
		}
		if err != nil {
			return nil, fmt.Errorf("creating: %w", err)
		}
		return desiredObj, nil
	}

	// An object already exists - this is the complicated part.

	// Keep a copy of the object on the cluster for comparison.
	// UpdatedObj will be changed according to desiredObj.
	updatedObj := currentObj.DeepCopy()

	// Check if we can even work on this object or need to adopt it.
	needsAdoption, err := r.adoptionChecker.Check(ctx, owner, currentObj, previous, collisionProtection)
	if err != nil {
		return nil, err
	}

	// Take over object ownership by patching metadata.
	if needsAdoption {
		log := logr.FromContextOrDiscard(ctx)
		log.Info("adopting object",
			"OwnerKey", client.ObjectKeyFromObject(owner.ClientObject()),
			"OwnerGVK", owner.ClientObject().GetObjectKind().GroupVersionKind(),
			"ObjectKey", client.ObjectKeyFromObject(desiredObj),
			"ObjectGVK", desiredObj.GetObjectKind().GroupVersionKind())
		setObjectRevision(updatedObj, owner.GetRevision())
		r.ownerStrategy.ReleaseController(updatedObj)
		if err := r.ownerStrategy.SetControllerReference(owner.ClientObject(), updatedObj); err != nil {
			return nil, err
		}
	}

	// Only issue updates when this instance is already controlled by this instance.
	if r.ownerStrategy.IsController(owner.ClientObject(), updatedObj) {
		if err := r.patcher.Patch(ctx, desiredObj, currentObj, updatedObj); err != nil {
			return nil, err
		}
	}

	return updatedObj, nil
}

type defaultPatcher struct {
	writer client.Writer
}

func (p *defaultPatcher) Patch(
	ctx context.Context,
	desiredObj, // object as specified by users
	currentObj, // object as currently present on the cluster
	// deepCopy of currentObj, already updated for owner handling
	updatedObj *unstructured.Unstructured,
) error {
	// Ensure owners are present
	desiredObj.SetOwnerReferences(updatedObj.GetOwnerReferences())

	patch := desiredObj.DeepCopy()
	// never patch status, even if specified
	// we would just start a fight with whatever controller is realizing this object.
	unstructured.RemoveNestedField(patch.Object, "status")

	if err := p.fixFieldManagers(ctx, currentObj); err != nil {
		return fmt.Errorf("fix field managers for SSA: %w", err)
	}

	objectPatch, err := json.Marshal(patch)
	if err != nil {
		return fmt.Errorf("creating patch: %w", err)
	}
	if err := p.writer.Patch(ctx, updatedObj, client.RawPatch(
		types.ApplyPatchType, objectPatch),
		client.FieldOwner(FieldOwner),
		client.ForceOwnership,
	); err != nil {
		return fmt.Errorf("patching object: %w", err)
	}
	return nil
}

// Autogenerated field owner names that we used previously.
// We need the list replace all of them with the value of `FieldOwner`.
var oldFieldOwners = sets.New(FieldOwner, "package-operator-manager", "remote-phase-manger")

// Migrate field ownerships to be compatible with server-side apply.
// SSA really is complicated: https://github.com/kubernetes/kubernetes/issues/99003
func (p *defaultPatcher) fixFieldManagers(
	ctx context.Context,
	currentObj *unstructured.Unstructured,
) error {
	patch, err := csaupgrade.UpgradeManagedFieldsPatch(currentObj, oldFieldOwners, FieldOwner)
	switch {
	case err != nil:
		return err
	case len(patch) == 0:
		// csaupgrade.UpgradeManagedFieldsPatch return nil, nil when no work is to be done. Empty patch cannot be applied so
		// exit early.
		return nil
	}

	if err := p.writer.Patch(ctx, currentObj, client.RawPatch(types.JSONPatchType, patch)); err != nil {
		return fmt.Errorf("update field managers: %w", err)
	}
	return nil
}

func mergeKeysFrom[K comparable, V any](base, additional map[K]V) map[K]V {
	if base == nil {
		base = map[K]V{}
	}
	for k, v := range additional {
		base[k] = v
	}
	if len(base) == 0 {
		return nil
	}
	return base
}

type defaultAdoptionChecker struct {
	scheme        *runtime.Scheme
	ownerStrategy ownerStrategy
}

// Check detects whether an ownership change is needed.
func (c *defaultAdoptionChecker) Check(
	_ context.Context, owner PhaseObjectOwner, obj client.Object,
	previous []PreviousObjectSet,
	collisionProtection corev1alpha1.CollisionProtection,
) (needsAdoption bool, err error) {
	if c.ownerStrategy.IsController(owner.ClientObject(), obj) {
		// already owner, nothing to do.
		return false, nil
	}

	currentRevision, err := getObjectRevision(obj)
	if err != nil {
		return false, fmt.Errorf("getting revision of object: %w", err)
	}
	// Never ever adopt objects of newer revisions.
	if currentRevision > owner.GetRevision() {
		// owned by newer revision.
		return false, nil
	}

	// Forced adoption is enabled:
	// - for all objects via the envvar in `ForceAdoptionEnvironmentVariable`
	// - for all objects in the package-operator (Cluster)Package

	// TODO: check if `obj.GetLabels()` actually CAN return a non-initialized map (aka `nil`)
	labels := obj.GetLabels()
	if labels == nil {
		labels = make(map[string]string)
	}
	// TODO: refactor the hardcoded PKO package name (there's another hardcoded reference in the bootstrap/init job)
	if len(os.Getenv(ForceAdoptionEnvironmentVariable)) > 0 ||
		labels[manifestsv1alpha1.PackageLabel] == "package-operator" {
		collisionProtection = corev1alpha1.CollisionProtectionNone
	}

	switch collisionProtection {
	case corev1alpha1.CollisionProtectionNone:
		// I hope the user knows what he is doing ;)
		return true, nil
	case corev1alpha1.CollisionProtectionIfNoController:
		if !c.ownerStrategy.HasController(obj) {
			return true, nil
		}
	}

	if !c.isControlledByPreviousRevision(obj, previous) {
		return false, &ObjectNotOwnedByPreviousRevisionError{
			CommonObjectPhaseError: CommonObjectPhaseError{
				OwnerKey:  client.ObjectKeyFromObject(owner.ClientObject()),
				OwnerGVK:  owner.ClientObject().GetObjectKind().GroupVersionKind(),
				ObjectKey: client.ObjectKeyFromObject(obj),
				ObjectGVK: obj.GetObjectKind().GroupVersionKind(),
			},
		}
	}

	if currentRevision == owner.GetRevision() {
		// This should not have happened.
		// Revision is same as owner,
		// but the object is not already owned by this object.
		return false, &RevisionCollisionError{
			CommonObjectPhaseError: CommonObjectPhaseError{
				OwnerKey:  client.ObjectKeyFromObject(owner.ClientObject()),
				OwnerGVK:  owner.ClientObject().GetObjectKind().GroupVersionKind(),
				ObjectKey: client.ObjectKeyFromObject(obj),
				ObjectGVK: obj.GetObjectKind().GroupVersionKind(),
			},
		}
	}

	// Object belongs to an older/lesser revision,
	// is not already owned by us and also belongs to a previous revision.
	return true, nil
}

func (c *defaultAdoptionChecker) isControlledByPreviousRevision(
	obj client.Object, previous []PreviousObjectSet,
) bool {
	for _, prev := range previous {
		if c.ownerStrategy.IsController(prev.ClientObject(), obj) {
			return true
		}

		remotePhases := prev.GetRemotePhases()
		if len(remotePhases) == 0 {
			continue
		}

		prevGVK, err := apiutil.GVKForObject(prev.ClientObject(), c.scheme)
		if err != nil {
			panic(err)
		}

		var remoteGVK schema.GroupVersionKind
		if strings.HasPrefix(prevGVK.Kind, "Cluster") {
			// ClusterObjectSet
			remoteGVK = corev1alpha1.GroupVersion.WithKind("ClusterObjectSetPhase")
		} else {
			// ObjectSet
			remoteGVK = corev1alpha1.GroupVersion.WithKind("ObjectSetPhase")
		}
		for _, remote := range remotePhases {
			potentialRemoteOwner := &unstructured.Unstructured{}
			potentialRemoteOwner.SetGroupVersionKind(remoteGVK)
			potentialRemoteOwner.SetName(remote.Name)
			potentialRemoteOwner.SetUID(remote.UID)
			potentialRemoteOwner.SetNamespace(
				prev.ClientObject().GetNamespace())

			if c.ownerStrategy.IsController(potentialRemoteOwner, obj) {
				return true
			}
		}
	}
	return false
}

// Retrieves the revision number from a well-known annotation on the given object.
func getObjectRevision(obj client.Object) (int64, error) {
	a := obj.GetAnnotations()
	if a == nil {
		return 0, nil
	}

	if len(a[corev1alpha1.ObjectSetRevisionAnnotation]) == 0 {
		return 0, nil
	}

	return strconv.ParseInt(a[corev1alpha1.ObjectSetRevisionAnnotation], 10, 64)
}

// Stores the revision number in a well-known annotation on the given object.
func setObjectRevision(obj client.Object, revision int64) {
	a := obj.GetAnnotations()
	if a == nil {
		a = map[string]string{}
	}
	a[corev1alpha1.ObjectSetRevisionAnnotation] = strconv.FormatInt(revision, 10)
	obj.SetAnnotations(a)
}

func hasDiverged(
	owner PhaseObjectOwner,
	ownerStrategy ownerStrategy,
	discoveryClient discovery.DiscoveryInterface,
	desiredObject, actualObject *unstructured.Unstructured,
) (diverged bool, managerPaths map[string]*fieldpath.Set, err error) {
	gvk := desiredObject.GroupVersionKind()

	r := openapi3.NewRoot(discoveryClient.OpenAPIV3())
	s, err := r.GVSpec(gvk.GroupVersion())
	if err != nil {
		return false, nil, err
	}
	ss, err := schemaconv.ToSchemaFromOpenAPI(s.Components.Schemas, false)
	if err != nil {
		return false, nil, err
	}

	var parser typed.Parser
	ss.CopyInto(&parser.Schema)

	mf, ok := findManagedFields(actualObject)
	if !ok {
		// no PKO managed fields -> diverged for sure
		// diverged on EVERYTHING.
		return true, nil, nil
	}
	actualFieldSet := &fieldpath.Set{}
	if err := actualFieldSet.FromJSON(bytes.NewReader(mf.FieldsV1.Raw)); err != nil {
		return false, nil, fmt.Errorf("field set for actual: %w", err)
	}

	desiredObject = desiredObject.DeepCopy()
	if err := ownerStrategy.SetControllerReference(owner.ClientObject(), desiredObject); err != nil {
		return false, nil, err
	}

	desiredObject = desiredObject.DeepCopy()

	tName, err := openAPICanonicalName(*desiredObject)
	if err != nil {
		return false, nil, err
	}
	typedDesired, err := parser.Type(tName).FromUnstructured(desiredObject.Object)
	if err != nil {
		return false, nil, fmt.Errorf("struct merge type conversion: %w", err)
	}
	desiredFieldSet, err := typedDesired.ToFieldSet()
	if err != nil {
		return false, nil, fmt.Errorf("desired to field set: %w", err)
	}

	diff := desiredFieldSet.Difference(actualFieldSet).Difference(stripSet).Leaves()

	managerPaths = map[string]*fieldpath.Set{}
	for _, mf := range actualObject.GetManagedFields() {
		fs := &fieldpath.Set{}
		if err := fs.FromJSON(bytes.NewReader(mf.FieldsV1.Raw)); err != nil {
			return false, nil, fmt.Errorf("field set for actual: %w", err)
		}
		diff.Leaves().Iterate(func(p fieldpath.Path) {
			if !fs.Has(p) {
				return
			}
			if _, ok := managerPaths[mf.Manager]; !ok {
				managerPaths[mf.Manager] = &fieldpath.Set{}
			}
			managerPaths[mf.Manager].Insert(p)
		})
	}
	return !diff.Empty(), managerPaths, nil
}

func findManagedFields(accessor metav1.Object) (metav1.ManagedFieldsEntry, bool) {
	objManagedFields := accessor.GetManagedFields()
	for _, mf := range objManagedFields {
		if mf.Manager == FieldOwner && mf.Operation == metav1.ManagedFieldsOperationApply && mf.Subresource == "" {
			return mf, true
		}
	}
	return metav1.ManagedFieldsEntry{}, false
}

// taken from:
// https://github.com/kubernetes/apimachinery/blob/v0.32.0-alpha.0/pkg/util/managedfields/internal/stripmeta.go#L39-L52
var stripSet = fieldpath.NewSet(
	fieldpath.MakePathOrDie("apiVersion"),
	fieldpath.MakePathOrDie("kind"),
	fieldpath.MakePathOrDie("metadata"),
	fieldpath.MakePathOrDie("metadata", "name"),
	fieldpath.MakePathOrDie("metadata", "namespace"),
	fieldpath.MakePathOrDie("metadata", "creationTimestamp"),
	fieldpath.MakePathOrDie("metadata", "selfLink"),
	fieldpath.MakePathOrDie("metadata", "uid"),
	fieldpath.MakePathOrDie("metadata", "clusterName"),
	fieldpath.MakePathOrDie("metadata", "generation"),
	fieldpath.MakePathOrDie("metadata", "managedFields"),
	fieldpath.MakePathOrDie("metadata", "resourceVersion"),
)

func openAPICanonicalName(obj unstructured.Unstructured) (string, error) {
	gvk := obj.GroupVersionKind()

	var schemaTypeName string
	o, err := scheme.Scheme.New(gvk)
	switch {
	case err != nil && runtime.IsNotRegisteredError(err):
		// Assume CRD
		schemaTypeName = fmt.Sprintf("%s/%s.%s", gvk.Group, gvk.Version, gvk.Kind)
	case err != nil:
		return "", err
	default:
		schemaTypeName = util.GetCanonicalTypeName(o)
	}
	return util.ToRESTFriendlyName(schemaTypeName), nil
}
