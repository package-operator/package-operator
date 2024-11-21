package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	"pkg.package-operator.run/boxcutter/machinery"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
	"package-operator.run/internal/constants"
	"package-operator.run/internal/preflight"
	"package-operator.run/pkg/probing"
)

// PhaseReconciler reconciles objects within a ObjectSet phase.
type PhaseReconciler struct {
	scheme *runtime.Scheme
	// just specify a writer, because we don't want to ever read from another source than
	// the dynamic cache that is managed to hold the objects we are reconciling.
	writer         client.Writer
	dynamicCache   dynamicCache
	uncachedClient client.Reader
	ownerStrategy  ownerStrategy
	phaseEngine    *machinery.PhaseEngine
}

type ownerStrategy interface {
	IsController(owner, obj metav1.Object) bool
	IsOwner(owner, obj metav1.Object) bool
	ReleaseController(obj metav1.Object)
	RemoveOwner(owner, obj metav1.Object)
	SetOwnerReference(owner, obj metav1.Object) error
	SetControllerReference(owner, obj metav1.Object) error
}

type dynamicCache interface {
	client.Reader
	Watch(
		ctx context.Context, owner client.Object, obj runtime.Object,
	) error
}

func NewPhaseReconciler(
	scheme *runtime.Scheme,
	writer client.Writer,
	dynamicCache dynamicCache,
	uncachedClient client.Reader,
	ownerStrategy ownerStrategy,
	phaseEngine *machinery.PhaseEngine,
) *PhaseReconciler {
	return &PhaseReconciler{
		scheme:         scheme,
		writer:         writer,
		dynamicCache:   dynamicCache,
		uncachedClient: uncachedClient,
		ownerStrategy:  ownerStrategy,

		phaseEngine: phaseEngine,
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

type probeWrapper struct {
	probe probing.Prober
}

func (w *probeWrapper) Probe(obj *unstructured.Unstructured) (success bool, messages []string) {
	success, m := w.probe.Probe(obj)
	return success, []string{m}
}

func (r *PhaseReconciler) ReconcilePhase(
	ctx context.Context, owner PhaseObjectOwner,
	phase corev1alpha1.ObjectSetTemplatePhase,
	probe probing.Prober, previous []PreviousObjectSet,
) (actualObjects []client.Object, res ProbingResult, err error) {
	mphase := r.desiredPhase(owner, phase, probe, previous)

	pres, err := r.phaseEngine.Reconcile(ctx, owner.ClientObject(), owner.GetRevision(), mphase)
	if err != nil {
		return nil, ProbingResult{}, err
	}

	if pres.PreflightViolation != nil {
		perr := &preflight.Error{
			Violations: []preflight.Violation{{
				Error: pres.PreflightViolation.String(),
			}},
		}
		return nil, res, perr
	}
	for _, ores := range pres.Objects {
		if ores.Action() != machinery.ActionCollision {
			continue
		}
		return nil, res, &ObjectNotOwnedByPreviousRevisionError{
			CommonObjectPhaseError: CommonObjectPhaseError{
				OwnerKey:  client.ObjectKeyFromObject(owner.ClientObject()),
				OwnerGVK:  owner.ClientObject().GetObjectKind().GroupVersionKind(),
				ObjectKey: client.ObjectKeyFromObject(ores.Object()),
				ObjectGVK: ores.Object().GetObjectKind().GroupVersionKind(),
			},
		}
	}

	rec := newRecordingProbe(phase.Name, probe)
	actualObjects = make([]client.Object, 0, len(phase.Objects))
	for i, ores := range pres.Objects {
		actualObjects = append(actualObjects, ores.Object())
		if !ores.Probe().Success {
			rec.recordForObj(ores.Object(), strings.Join(ores.Probe().Messages, " and "))
		}
		if err := mapConditions(ctx, owner, phase.Objects[i].ConditionMappings, ores.Object()); err != nil {
			return nil, ProbingResult{}, fmt.Errorf("map conditions: %w", err)
		}
	}
	return actualObjects, rec.Result(), nil
}

func (r *PhaseReconciler) TeardownPhase(
	ctx context.Context, owner PhaseObjectOwner,
	phase corev1alpha1.ObjectSetTemplatePhase,
) (cleanupDone bool, err error) {
	mphase := r.desiredPhase(owner, phase, nil, nil)
	return r.phaseEngine.Teardown(ctx, owner.ClientObject(), owner.GetRevision(), mphase)
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

func (r *PhaseReconciler) desiredPhase(
	owner PhaseObjectOwner,
	phase corev1alpha1.ObjectSetTemplatePhase,
	probe probing.Prober, previous []PreviousObjectSet,
) machinery.Phase {
	mphase := machinery.Phase{
		Name: phase.Name,
	}
	prev := make([]client.Object, 0, len(previous))
	for _, p := range previous {
		prev = append(prev, p.ClientObject())

		// Add nested (Cluster)ObjectSetPhases as valid previous owners.
		remotePhases := p.GetRemotePhases()
		if len(remotePhases) == 0 {
			continue
		}
		prevGVK, err := apiutil.GVKForObject(p.ClientObject(), r.scheme)
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
			potentialRemoteOwner.SetNamespace(p.ClientObject().GetNamespace())
			prev = append(prev, potentialRemoteOwner)
		}
	}
	commonOpts := []machinery.ObjectOption{
		machinery.WithPreviousOwners(prev),
	}
	if probe != nil {
		commonOpts = append(commonOpts, machinery.WithProbe{Probe: &probeWrapper{probe: probe}})
	}
	if owner.IsPaused() {
		commonOpts = append(commonOpts, machinery.WithPaused{})
	}
	// TODO: refactor the hardcoded PKO package name (there's another hardcoded reference in the bootstrap/init job)
	var forceOwnership bool
	// if len(os.Getenv(constants.ForceAdoptionEnvironmentVariable)) > 0 {
	if owner.ClientObject().GetLabels()[manifestsv1alpha1.PackageLabel] == "package-operator" {
		forceOwnership = true
	}

	for _, o := range phase.Objects {
		mphase.Objects = append(
			mphase.Objects,
			r.desiredPhaseObject(owner, o, commonOpts, forceOwnership))
	}
	return mphase
}

func (r *PhaseReconciler) desiredPhaseObject(
	owner PhaseObjectOwner,
	phaseObject corev1alpha1.ObjectSetObject,
	commonOpts []machinery.ObjectOption,
	forceOwnership bool,
) machinery.PhaseObject {
	obj := phaseObject.Object.DeepCopy()

	// Set cache label
	labels := obj.GetLabels()
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
	obj.SetLabels(labels)

	if len(obj.GetNamespace()) == 0 {
		obj.SetNamespace(owner.ClientObject().GetNamespace())
	}

	pobj := machinery.PhaseObject{
		Object: obj,
		Opts:   commonOpts,
	}
	if forceOwnership {
		pobj.Opts = append(pobj.Opts, machinery.WithCollisionProtection(machinery.CollisionProtectionNone))
	} else {
		pobj.Opts = append(pobj.Opts, machinery.WithCollisionProtection(phaseObject.CollisionProtection))
	}
	return pobj
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
