package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/package-operator/internal/preflight"
	"package-operator.run/package-operator/internal/probing"
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
}

type ownerStrategy interface {
	IsController(owner, obj metav1.Object) bool
	ReleaseController(obj metav1.Object)
	RemoveOwner(owner, obj metav1.Object)
	SetControllerReference(owner, obj metav1.Object) error
	OwnerPatch(owner metav1.Object) ([]byte, error)
}

type adoptionChecker interface {
	Check(
		ctx context.Context, owner PhaseObjectOwner, obj client.Object,
		previous []PreviousObjectSet,
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
	violations, err := preflight.CheckAllInPhase(
		ctx, r.preflightChecker, owner.ClientObject(), phase)
	if err != nil {
		return nil, res, err
	}
	if len(violations) > 0 {
		return nil, res, &preflight.Error{Violations: violations}
	}

	rec := newRecordingProbe(phase.Name, probe)

	for _, phaseObject := range phase.Objects {
		actualObj, err := r.reconcilePhaseObject(ctx, owner, phaseObject, previous)
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

	if err := r.dynamicCache.Get(ctx, key, observed); errors.IsNotFound(err) {
		if err := r.uncachedClient.Get(ctx, key, obj); err != nil {
			return nil, fmt.Errorf("retrieving external object: %w", err)
		}

		// Update object to ensure it is part of our cache and we get events to reconcile.
		if observed, err = AddDynamicCacheLabel(ctx, r.writer, observed); err != nil {
			return nil, fmt.Errorf("adding cache label: %w", err)
		}
	} else if err != nil {
		return nil, fmt.Errorf("retrieving external object: %w", err)
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

func (r *PhaseReconciler) teardownPhaseObject(
	ctx context.Context, owner PhaseObjectOwner,
	phaseObject corev1alpha1.ObjectSetObject,
) (cleanupDone bool, err error) {
	desiredObj, err := r.desiredObject(ctx, owner, phaseObject)
	if err != nil {
		return false, fmt.Errorf("building desired object: %w", err)
	}

	// Ensure to watch this type of object, also during teardown!
	// If the controller was restarted or crashed during deletion, we might not have a cache in memory anymore.
	if err := r.dynamicCache.Watch(
		ctx, owner.ClientObject(), desiredObj); err != nil {
		return false, fmt.Errorf("watching new resource: %w", err)
	}

	currentObj := desiredObj.DeepCopy()
	err = r.dynamicCache.Get(
		ctx, client.ObjectKeyFromObject(desiredObj), currentObj)
	if err != nil && errors.IsNotFound(err) {
		// No matter who the owner of this object is,
		// it's already gone.
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("getting object for teardown: %w", err)
	}

	if !r.ownerStrategy.IsController(owner.ClientObject(), currentObj) {
		// this object is owned by someone else
		// so we don't have to delete it for cleanup,
		// but we still want to remove ourselves as owner.
		r.ownerStrategy.RemoveOwner(owner.ClientObject(), currentObj)
		if err := r.writer.Update(ctx, currentObj); err != nil {
			return false, fmt.Errorf("removing owner reference: %w", err)
		}
		return true, nil
	}

	err = r.writer.Delete(ctx, currentObj)
	if err != nil && errors.IsNotFound(err) {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("deleting object for teardown: %w", err)
	}

	return false, nil
}

func (r *PhaseReconciler) reconcilePhaseObject(
	ctx context.Context, owner PhaseObjectOwner,
	phaseObject corev1alpha1.ObjectSetObject,
	previous []PreviousObjectSet,
) (actualObj *unstructured.Unstructured, err error) {
	desiredObj, err := r.desiredObject(
		ctx, owner, phaseObject)
	if err != nil {
		return nil, fmt.Errorf("building desired object: %w", err)
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
		return actualObj, nil
	}

	if actualObj, err = r.reconcileObject(ctx, owner, desiredObj, previous); err != nil {
		return nil, err
	}

	if err = mapConditions(ctx, owner, phaseObject.ConditionMappings, actualObj); err != nil {
		return nil, err
	}

	return actualObj, nil
}

func mapConditions(
	ctx context.Context, owner PhaseObjectOwner,
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
	ctx context.Context, owner PhaseObjectOwner,
	phaseObject corev1alpha1.ObjectSetObject,
) (desiredObj *unstructured.Unstructured, err error) {
	desiredObj = &phaseObject.Object

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
	desiredObj.SetLabels(labels)

	setObjectRevision(desiredObj, owner.GetRevision())

	// Set owner reference
	if err := r.ownerStrategy.SetControllerReference(owner.ClientObject(), desiredObj); err != nil {
		return nil, err
	}
	return desiredObj, nil
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

func (e ObjectNotOwnedByPreviousRevisionError) Error() string {
	return fmt.Sprintf("refusing adoption, object %s %s not owned by previous revision", e.ObjectGVK, e.ObjectKey)
}

// This error is returned when a Phase tries to adopt an object
// where the revision number is not increasing.
type RevisionCollisionError struct {
	CommonObjectPhaseError
}

func (e RevisionCollisionError) Error() string {
	return fmt.Sprintf("refusing adoption, revision collision on %s %s", e.ObjectGVK, e.ObjectKey)
}

func (r *PhaseReconciler) reconcileObject(
	ctx context.Context, owner PhaseObjectOwner,
	desiredObj *unstructured.Unstructured, previous []PreviousObjectSet,
) (actualObj *unstructured.Unstructured, err error) {
	objKey := client.ObjectKeyFromObject(desiredObj)
	currentObj := desiredObj.DeepCopy()
	err = r.dynamicCache.Get(ctx, objKey, currentObj)
	if err != nil && !errors.IsNotFound(err) {
		return nil, fmt.Errorf("getting %s: %w", desiredObj.GroupVersionKind(), err)
	}
	if errors.IsNotFound(err) {
		// The object is not yet present on the cluster,
		// just create it using desired state!
		err := r.writer.Create(ctx, desiredObj)
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
	needsAdoption, err := r.adoptionChecker.Check(ctx, owner, currentObj, previous)
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

		ownerPatch, err := r.ownerStrategy.OwnerPatch(updatedObj)
		if err != nil {
			return nil, fmt.Errorf("ownership patch: %w", err)
		}
		if err := r.writer.Patch(ctx, updatedObj, client.RawPatch(
			types.MergePatchType, ownerPatch,
		)); err != nil {
			return nil, fmt.Errorf("patching object ownership: %w", err)
		}
	}

	// Only issue updates when this instance is already or will be controlled by this instance.
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
	// Ensure desired labels and annotations are present
	desiredObj.SetLabels(mergeKeysFrom(updatedObj.GetLabels(), desiredObj.GetLabels()))
	desiredObj.SetAnnotations(mergeKeysFrom(updatedObj.GetAnnotations(), desiredObj.GetAnnotations()))

	patch := desiredObj.DeepCopy()
	// never patch status, even if specified
	// we would just start a fight with whatever controller is realizing this object.
	unstructured.RemoveNestedField(patch.Object, "status")
	// don't strategic merge ownerReferences - we already take care about that with its own patch.
	unstructured.RemoveNestedField(patch.Object, "metadata", "ownerReferences")

	base := updatedObj.DeepCopy()
	unstructured.RemoveNestedField(base.Object, "status")

	// Check for if an update is even needed.
	if !equality.Semantic.DeepDerivative(patch, base) {
		patch.SetResourceVersion(currentObj.GetResourceVersion())
		objectPatch, err := json.Marshal(patch)
		if err != nil {
			return fmt.Errorf("creating patch: %w", err)
		}
		if err := p.writer.Patch(ctx, updatedObj, client.RawPatch(
			types.ApplyPatchType, objectPatch),
			client.FieldOwner("package-operator"),
			client.ForceOwnership,
		); err != nil {
			return fmt.Errorf("patching object: %w", err)
		}
	}
	return nil
}

func mergeKeysFrom(base, additional map[string]string) map[string]string {
	if base == nil {
		base = map[string]string{}
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
	ctx context.Context, owner PhaseObjectOwner, obj client.Object,
	previous []PreviousObjectSet,
) (needsAdoption bool, err error) {
	if len(os.Getenv("PKO_FORCE_ADOPTION")) > 0 {
		return true, nil
	}

	if c.ownerStrategy.IsController(owner.ClientObject(), obj) {
		// already owner, nothing to do.
		return false, nil
	}

	currentRevision, err := getObjectRevision(obj)
	if err != nil {
		return false, fmt.Errorf("getting revision of object: %w", err)
	}
	if currentRevision > owner.GetRevision() {
		// owned by newer revision.
		return false, nil
	}

	if !c.isControlledByPreviousRevision(obj, previous) {
		return false, ObjectNotOwnedByPreviousRevisionError{
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
		return false, RevisionCollisionError{
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

const (
	// Revision annotations holds a revision generation number to order ObjectSets.
	revisionAnnotation = "package-operator.run/revision"
)

// Retrieves the revision number from a well-known annotation on the given object.
func getObjectRevision(obj client.Object) (int64, error) {
	a := obj.GetAnnotations()
	if a == nil {
		return 0, nil
	}

	if len(a[revisionAnnotation]) == 0 {
		return 0, nil
	}

	return strconv.ParseInt(a[revisionAnnotation], 10, 64)
}

// Stores the revision number in a well-known annotation on the given object.
func setObjectRevision(obj client.Object, revision int64) {
	a := obj.GetAnnotations()
	if a == nil {
		a = map[string]string{}
	}
	a[revisionAnnotation] = fmt.Sprintf("%d", revision)
	obj.SetAnnotations(a)
}
