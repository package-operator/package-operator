package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/yaml"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/package-operator/internal/probing"
)

// PhaseReconciler reconciles objects within a ObjectSet phase.
type PhaseReconciler struct {
	scheme *runtime.Scheme
	// just specify a writer, because we don't want to ever read from another source than
	// the dynamic cache that is managed to hold the objects we are reconciling.
	writer          client.Writer
	dynamicCache    dynamicCache
	ownerStrategy   ownerStrategy
	adoptionChecker adoptionChecker
	patcher         patcher
}

type ownerStrategy interface {
	IsController(owner, obj metav1.Object) bool
	ReleaseController(obj metav1.Object)
	RemoveOwner(owner, obj metav1.Object)
	SetControllerReference(owner, obj metav1.Object) error
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

type PhaseProbingFailedError struct {
	FailedProbes []string
	PhaseName    string
}

func (e *PhaseProbingFailedError) ErrorWithoutPhase() string {
	return strings.Join(e.FailedProbes, ", ")
}

func (e *PhaseProbingFailedError) Error() string {
	return fmt.Sprintf("Phase %q failed: %s",
		e.PhaseName, e.ErrorWithoutPhase())
}

func NewPhaseReconciler(
	scheme *runtime.Scheme,
	writer client.Writer,
	dynamicCache dynamicCache,
	ownerStrategy ownerStrategy,
) *PhaseReconciler {
	return &PhaseReconciler{
		scheme:          scheme,
		writer:          writer,
		dynamicCache:    dynamicCache,
		ownerStrategy:   ownerStrategy,
		adoptionChecker: &defaultAdoptionChecker{ownerStrategy: ownerStrategy, scheme: scheme},
		patcher:         &defaultPatcher{writer: writer},
	}
}

type PhaseObjectOwner interface {
	ClientObject() client.Object
	GetRevision() int64
	IsPaused() bool
}

func (r *PhaseReconciler) ReconcilePhase(
	ctx context.Context, owner PhaseObjectOwner,
	phase corev1alpha1.ObjectSetTemplatePhase,
	probe probing.Prober, previous []PreviousObjectSet,
) error {
	var failedProbes []string
	for _, phaseObject := range phase.Objects {
		actualObj, err := r.reconcilePhaseObject(ctx, owner, phaseObject, previous)
		if err != nil {
			return err
		}

		if success, message := probe.Probe(actualObj); !success {
			gvk := actualObj.GroupVersionKind()
			failedProbes = append(failedProbes,
				fmt.Sprintf("%s %s %s/%s: %s",
					gvk.Group, gvk.Kind, actualObj.GetNamespace(), actualObj.GetName(), message))
		}
	}

	if len(failedProbes) > 0 {
		return &PhaseProbingFailedError{
			FailedProbes: failedProbes,
			PhaseName:    phase.Name,
		}
	}
	return nil
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
		// but we still want to remove ourself as owner.
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

	return r.reconcileObject(ctx, owner, desiredObj, previous)
}

// Builds an object as specified in a phase.
// Includes system labels, namespace and owner reference.
func (r *PhaseReconciler) desiredObject(
	ctx context.Context, owner PhaseObjectOwner,
	phaseObject corev1alpha1.ObjectSetObject,
) (desiredObj *unstructured.Unstructured, err error) {
	desiredObj, err = unstructuredFromObjectSetObject(&phaseObject)
	if err != nil {
		return nil, err
	}

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
	updatedObj.SetLabels(mergeKeysFrom(updatedObj.GetLabels(), desiredObj.GetLabels()))
	updatedObj.SetAnnotations(mergeKeysFrom(updatedObj.GetAnnotations(), desiredObj.GetAnnotations()))

	// At this point metadata of updatedObj is how we want it to look like.
	// So we can persist our changes (if any) to the API server.
	updatedObjMeta, _, err := unstructured.NestedFieldNoCopy(updatedObj.Object, "metadata")
	if err != nil {
		panic(err) // this key MUST always be present at this point
	}
	currentObjMeta, _, err := unstructured.NestedFieldNoCopy(currentObj.Object, "metadata")
	if err != nil {
		panic(err) // this key MUST always be present at this point
	}

	// DeepEqual check to prevent unnecessary PATCH calls to the API.
	if !reflect.DeepEqual(updatedObjMeta, currentObjMeta) {
		// Patch with optimisticLocking to make sure ResourceVersion is checked.
		// OptimisticLocking is enabled by providing the resourceVersion property in the patch.
		// Just overriding would risk loosing labels and annotations added by other participants of the system.

		metadataPatch, err := json.Marshal(map[string]interface{}{
			"metadata": updatedObjMeta,
		})
		if err != nil {
			return fmt.Errorf("creating metadata patch: %w", err)
		}

		if err := p.writer.Patch(ctx, updatedObj, client.RawPatch(
			types.MergePatchType, metadataPatch)); err != nil {
			return fmt.Errorf("patching object metadata: %w", err)
		}
	}

	patch := desiredObj.DeepCopy()
	// metadata is already up-to-date and we don't want to patch it without optimistic locking.
	unstructured.RemoveNestedField(patch.Object, "metadata")
	// never patch status, even if specified
	// we would just start a fight with whatever controller is realizing this object.
	unstructured.RemoveNestedField(patch.Object, "status")

	base := updatedObj.DeepCopy()
	unstructured.RemoveNestedField(base.Object, "metadata")
	unstructured.RemoveNestedField(base.Object, "status")

	// Check for if an update is even needed.
	if !equality.Semantic.DeepDerivative(patch, base) {
		objectPatch, err := json.Marshal(patch)
		if err != nil {
			return fmt.Errorf("creating metadata patch: %w", err)
		}
		if err := p.writer.Patch(ctx, updatedObj, client.RawPatch(
			types.MergePatchType, objectPatch)); err != nil {
			return fmt.Errorf("patching object: %w", err)
		}
	}
	return nil
}

func unstructuredFromObjectSetObject(
	packageObject *corev1alpha1.ObjectSetObject,
) (*unstructured.Unstructured, error) {
	obj := &unstructured.Unstructured{}
	// Warning!
	// This MUST absolutely use sigs.k8s.io/yaml
	// Any other yaml parser, might yield unexpected results.
	if err := yaml.Unmarshal(packageObject.Object.Raw, obj); err != nil {
		return nil, fmt.Errorf("converting RawExtension into unstructured: %w", err)
	}
	return obj, nil
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
