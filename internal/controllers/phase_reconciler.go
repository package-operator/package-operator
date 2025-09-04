package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	apimachineryerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/util/csaupgrade"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	"pkg.package-operator.run/boxcutter/managedcache"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
	"package-operator.run/internal/constants"
	"package-operator.run/internal/preflight"
	"package-operator.run/pkg/probing"
)

// PhaseReconciler reconciles objects within a ObjectSet phase.
type PhaseReconciler interface {
	ReconcilePhase(
		ctx context.Context, owner PhaseObjectOwner,
		phase corev1alpha1.ObjectSetTemplatePhase,
		probe probing.Prober, previous []PreviousObjectSet,
	) ([]client.Object, ProbingResult, error)

	TeardownPhase(
		ctx context.Context, owner PhaseObjectOwner,
		phase corev1alpha1.ObjectSetTemplatePhase,
	) (cleanupDone bool, err error)
}

type phaseReconciler struct {
	scheme   *runtime.Scheme
	accessor managedcache.Accessor
	// Dangerous: the uncached client here is not scoped by
	// the mapper passed to managedcache.ObjectBoundAccessManager!
	// This warning will be removed when the rest of PKO will be refactored
	// to use boxcutter's {Revision,Phase,Object}Engines.
	uncachedClient   client.Reader
	ownerStrategy    ownerStrategy
	adoptionChecker  adoptionChecker
	patcher          patcher
	preflightChecker preflightChecker
}

type ownerStrategy interface {
	GetController(obj metav1.Object) (metav1.OwnerReference, bool)
	IsController(owner, obj metav1.Object) bool
	IsOwner(owner, obj metav1.Object) bool
	ReleaseController(obj metav1.Object)
	RemoveOwner(owner, obj metav1.Object)
	SetOwnerReference(owner, obj metav1.Object) error
	SetControllerReference(owner, obj metav1.Object) error
}

type adoptionChecker interface {
	Check(
		owner PhaseObjectOwner, obj client.Object,
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

type preflightChecker interface {
	Check(
		ctx context.Context, owner, obj client.Object,
	) (violations []preflight.Violation, err error)
}

type PhaseObjectOwner interface {
	ClientObject() client.Object
	GetStatusRevision() int64
	GetStatusConditions() *[]metav1.Condition
	IsSpecPaused() bool
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
	p.recordForObj(obj, []string{"not found"})
}

func (p *recordingProbe) recordForObj(obj *unstructured.Unstructured, msgs []string) {
	gvk := obj.GroupVersionKind()
	msg := fmt.Sprintf("%s %s %s/%s: %s", gvk.Group, gvk.Kind, obj.GetNamespace(), obj.GetName(), strings.Join(msgs, ", "))

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

func (r *phaseReconciler) ReconcilePhase(
	ctx context.Context, owner PhaseObjectOwner,
	phase corev1alpha1.ObjectSetTemplatePhase,
	probe probing.Prober, previous []PreviousObjectSet,
) (actualObjects []client.Object, res ProbingResult, err error) {
	desiredObjects := make([]unstructured.Unstructured, len(phase.Objects))
	for i, phaseObject := range phase.Objects {
		desiredObjects[i] = *r.desiredObject(ctx, owner, phaseObject)
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

	return actualObjects, rec.Result(), nil
}

func (r *phaseReconciler) TeardownPhase(
	ctx context.Context, owner PhaseObjectOwner,
	phase corev1alpha1.ObjectSetTemplatePhase,
) (cleanupDone bool, err error) {
	var cleanupCounter int
	objectsToCleanup := len(phase.Objects)
	for _, phaseObject := range phase.Objects {
		done, err := r.teardownPhaseObject(ctx, owner, phaseObject)
		if err != nil {
			return false, err
		}

		if done {
			cleanupCounter++
		}
	}

	return cleanupCounter == objectsToCleanup, nil
}

func (r *phaseReconciler) teardownPhaseObject(
	ctx context.Context, owner PhaseObjectOwner,
	phaseObject corev1alpha1.ObjectSetObject,
) (cleanupDone bool, err error) {
	log := ctrl.LoggerFrom(ctx)

	desiredObj := r.desiredObject(ctx, owner, phaseObject)

	// Preflight checker during teardown prevents the deletion of resources in different namespaces and
	// unblocks teardown when APIs have been removed.
	if v, err := r.preflightChecker.Check(ctx, owner.ClientObject(), desiredObj); err != nil {
		return false, fmt.Errorf("running preflight validation: %w", err)
	} else if len(v) > 0 {
		return true, nil
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
		object := &unstructured.Unstructured{}
		object.SetOwnerReferences(currentObj.GetOwnerReferences())
		r.ownerStrategy.RemoveOwner(owner.ClientObject(), object)
		objectPatch := map[string]interface{}{
			"metadata": map[string]interface{}{
				"labels": map[string]interface{}{
					constants.DynamicCacheLabel: nil,
				},
				"ownerReferences": object.GetOwnerReferences(),
			},
		}
		objectPatchJSON, err := json.Marshal(objectPatch)
		if err != nil {
			return false, fmt.Errorf("creating patch: %w", err)
		}
		if err = r.accessor.Patch(ctx, currentObj, client.RawPatch(
			types.MergePatchType, objectPatchJSON,
		)); err != nil {
			return false, fmt.Errorf("removing external object owner reference: %w", err)
		}

		return true, nil
	}

	log.Info("deleting managed object",
		"apiVersion", currentObj.GetAPIVersion(),
		"kind", currentObj.GroupVersionKind().Kind,
		"namespace", currentObj.GetNamespace(),
		"name", currentObj.GetName())

	// Delete object with preconditions, enforcing that `currentObj` corresponds
	// to the latest api revision of this object.
	// This should make it impossible to accidentally delete orphaned children
	// in case we missed the orphan finalizer.
	err = r.accessor.Delete(ctx, currentObj, client.Preconditions{
		UID:             ptr.To(currentObj.GetUID()),
		ResourceVersion: ptr.To(currentObj.GetResourceVersion()),
	})
	// TODO - not found with uid does not return IsNotFound but preconditionfailed
	if err != nil && apimachineryerrors.IsNotFound(err) {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("deleting object for teardown: %w", err)
	}

	return false, nil
}

func (r *phaseReconciler) reconcilePhaseObject(
	ctx context.Context, owner PhaseObjectOwner,
	phaseObject corev1alpha1.ObjectSetObject,
	desiredObj *unstructured.Unstructured,
	previous []PreviousObjectSet,
) (actualObj *unstructured.Unstructured, err error) {
	// Set owner reference
	if err := r.ownerStrategy.SetControllerReference(owner.ClientObject(), desiredObj); err != nil {
		return nil, fmt.Errorf("set controller reference: %w", err)
	}

	if owner.IsSpecPaused() {
		actualObj = desiredObj.DeepCopy()
		if err := r.accessor.Get(ctx, client.ObjectKeyFromObject(desiredObj), actualObj); err != nil {
			return nil, fmt.Errorf("looking up object while paused: %w", err)
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

		meta.SetStatusCondition(owner.GetStatusConditions(), metav1.Condition{
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
func (r *phaseReconciler) desiredObject(
	_ context.Context, owner PhaseObjectOwner,
	phaseObject corev1alpha1.ObjectSetObject,
) (desiredObj *unstructured.Unstructured) {
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
	labels[constants.DynamicCacheLabel] = "True"

	if ownerLabels := owner.ClientObject().GetLabels(); ownerLabels != nil {
		if pkgLabel, ok := ownerLabels[manifestsv1alpha1.PackageLabel]; ok {
			labels[manifestsv1alpha1.PackageLabel] = pkgLabel
		}
		if pkgInstanceLabel, ok := ownerLabels[manifestsv1alpha1.PackageInstanceLabel]; ok {
			labels[manifestsv1alpha1.PackageInstanceLabel] = pkgInstanceLabel
		}
	}

	desiredObj.SetLabels(labels)

	setObjectRevision(desiredObj, owner.GetStatusRevision())

	return desiredObj
}

// updateStatusError(ctx context.Context, objectSet genericObjectSet,
// 	reconcileErr error,
// ) (res ctrl.Result, err error)

type ObjectSetOrPhase interface {
	ClientObject() client.Object
	GetStatusConditions() *[]metav1.Condition
}

func UpdateObjectSetOrPhaseStatusFromError(
	ctx context.Context, objectSetOrPhase ObjectSetOrPhase,
	reconcileErr error, updateStatus func(ctx context.Context) error,
) (res ctrl.Result, err error) {
	var preflightError *preflight.Error
	if errors.As(reconcileErr, &preflightError) {
		meta.SetStatusCondition(objectSetOrPhase.GetStatusConditions(), metav1.Condition{
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
		meta.SetStatusCondition(objectSetOrPhase.GetStatusConditions(), metav1.Condition{
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

func (r *phaseReconciler) reconcileObject(
	ctx context.Context, owner PhaseObjectOwner,
	desiredObj *unstructured.Unstructured, previous []PreviousObjectSet,
	collisionProtection corev1alpha1.CollisionProtection,
) (actualObj *unstructured.Unstructured, err error) {
	objKey := client.ObjectKeyFromObject(desiredObj)
	currentObj := desiredObj.DeepCopy()
	err = r.accessor.Get(ctx, objKey, currentObj)
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
		err := r.accessor.Patch(ctx, desiredObj, client.Apply, client.FieldOwner(constants.FieldOwner))
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
	needsAdoption, err := r.adoptionChecker.Check(owner, currentObj, previous, collisionProtection)
	if err != nil {
		return nil, err
	}

	// Take over object ownership by patching metadata.
	if needsAdoption {
		log := ctrl.LoggerFrom(ctx)
		log.Info("adopting object",
			"OwnerKey", client.ObjectKeyFromObject(owner.ClientObject()),
			"OwnerGVK", owner.ClientObject().GetObjectKind().GroupVersionKind(),
			"ObjectKey", client.ObjectKeyFromObject(desiredObj),
			"ObjectGVK", desiredObj.GetObjectKind().GroupVersionKind())
		setObjectRevision(updatedObj, owner.GetStatusRevision())
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
		client.FieldOwner(constants.FieldOwner),
		client.ForceOwnership,
	); err != nil {
		return fmt.Errorf("patching object: %w", err)
	}
	return nil
}

// Autogenerated field owner names that we used previously.
// We need the list replace all of them with the value of `FieldOwner`.
var oldFieldOwners = sets.New(constants.FieldOwner, "package-operator-manager", "remote-phase-manger")

// Migrate field ownerships to be compatible with server-side apply.
// SSA really is complicated: https://github.com/kubernetes/kubernetes/issues/99003
func (p *defaultPatcher) fixFieldManagers(
	ctx context.Context,
	currentObj *unstructured.Unstructured,
) error {
	patch, err := csaupgrade.UpgradeManagedFieldsPatch(currentObj, oldFieldOwners, constants.FieldOwner)
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
func (c *defaultAdoptionChecker) Check(owner PhaseObjectOwner, obj client.Object,
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
	if currentRevision > owner.GetStatusRevision() {
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
	if len(os.Getenv(constants.ForceAdoptionEnvironmentVariable)) > 0 ||
		labels[manifestsv1alpha1.PackageLabel] == "package-operator" {
		collisionProtection = corev1alpha1.CollisionProtectionNone
	}

	switch collisionProtection {
	case corev1alpha1.CollisionProtectionNone:
		// I hope the user knows what he is doing ;)
		return true, nil
	case corev1alpha1.CollisionProtectionIfNoController:
		if _, hasController := c.ownerStrategy.GetController(obj); !hasController {
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

	if currentRevision == owner.GetStatusRevision() {
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

		remotePhases := prev.GetStatusRemotePhases()
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
