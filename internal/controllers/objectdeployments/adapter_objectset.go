package objectdeployments

import (
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/internal/utils"
)

const (
	pausedByParentAnnotation = "package-operator.run/paused-by-parent"
	pausedByParentTrue       = "true"
	pausedByParentFalse      = "false"
)

type genericObjectSet interface {
	ClientObject() client.Object
	GetTemplateSpec() corev1alpha1.ObjectSetTemplateSpec
	SetTemplateSpec(templateSpec corev1alpha1.ObjectSetTemplateSpec)
	SetPreviousRevisions(prev []genericObjectSet)
	GetObjects() ([]objectIdentifier, error)
	GetActivelyReconciledObjects() []objectIdentifier
	GetPhases() []corev1alpha1.ObjectSetTemplatePhase
	GetConditions() []metav1.Condition
	SetArchived()
	IsArchived() bool
	GetRevision() int64
	GetGeneration() int64
	IsStatusPaused() bool
	SetPaused()
	IsSpecPaused() bool
	IsAvailable() bool
	SetPausedByParent()
	SetActiveByParent()
	GetPausedByParent() bool
}

type genericObjectSetFactory func(
	scheme *runtime.Scheme) genericObjectSet

var (
	objectSetGVK        = corev1alpha1.GroupVersion.WithKind("ObjectSet")
	clusterObjectSetGVK = corev1alpha1.GroupVersion.WithKind("ClusterObjectSet")
)

func newGenericObjectSet(scheme *runtime.Scheme) genericObjectSet {
	obj, err := scheme.New(objectSetGVK)
	if err != nil {
		panic(err)
	}

	return &GenericObjectSet{
		ObjectSet: *obj.(*corev1alpha1.ObjectSet),
	}
}

func newGenericClusterObjectSet(scheme *runtime.Scheme) genericObjectSet {
	obj, err := scheme.New(clusterObjectSetGVK)
	if err != nil {
		panic(err)
	}

	return &GenericClusterObjectSet{
		ClusterObjectSet: *obj.(*corev1alpha1.ClusterObjectSet),
	}
}

var (
	_ genericObjectSet = (*GenericObjectSet)(nil)
	_ genericObjectSet = (*GenericClusterObjectSet)(nil)
)

type objectIdentifier interface {
	UniqueIdentifier() string
}

type objectSetObjectIdentifier struct {
	kind      string
	name      string
	namespace string
	group     string
}

func (o objectSetObjectIdentifier) UniqueIdentifier() string {
	return fmt.Sprintf("%s/%s/%s/%s", o.group, o.kind, o.namespace, o.name)
}

type GenericObjectSet struct {
	corev1alpha1.ObjectSet
}

func (a *GenericObjectSet) GetTemplateSpec() corev1alpha1.ObjectSetTemplateSpec {
	return a.Spec.ObjectSetTemplateSpec
}

func (a *GenericObjectSet) GetActivelyReconciledObjects() []objectIdentifier {
	res := make([]objectIdentifier, 0)
	if a.IsArchived() {
		// If an objectset is archived, it doesnt actively
		// reconcile anything, we just return an empty list
		return []objectIdentifier{}
	}

	if a.Status.ControllerOf == nil {
		// ActivelyReconciledObjects status is not reported yet
		return nil
	}

	for _, reconciledObj := range a.Status.ControllerOf {
		currentObj := objectSetObjectIdentifier{
			kind:      reconciledObj.Kind,
			group:     reconciledObj.Group,
			name:      reconciledObj.Name,
			namespace: reconciledObj.Namespace,
		}
		res = append(res, currentObj)
	}
	return res
}

func (a *GenericObjectSet) IsStatusPaused() bool {
	return meta.IsStatusConditionTrue(
		a.Status.Conditions,
		corev1alpha1.ObjectSetPaused,
	)
}

func (a *GenericObjectSet) IsAvailable() bool {
	return meta.IsStatusConditionTrue(
		a.Status.Conditions,
		corev1alpha1.ObjectSetAvailable,
	)
}

func (a *GenericObjectSet) SetPaused() {
	a.Spec.LifecycleState = corev1alpha1.ObjectSetLifecycleStatePaused
}

func (a *GenericObjectSet) IsSpecPaused() bool {
	return a.Spec.LifecycleState == corev1alpha1.ObjectSetLifecycleStatePaused
}

func (a *GenericObjectSet) SetTemplateSpec(templateSpec corev1alpha1.ObjectSetTemplateSpec) {
	a.Spec.ObjectSetTemplateSpec = templateSpec
}

func (a *GenericObjectSet) ClientObject() client.Object {
	return &a.ObjectSet
}

func (a *GenericObjectSet) GetConditions() []metav1.Condition {
	return a.Status.Conditions
}

func (a *GenericObjectSet) GetRevision() int64 {
	return a.Status.Revision
}

func (a *GenericObjectSet) GetGeneration() int64 {
	return a.Generation
}

func (a *GenericObjectSet) GetPhases() []corev1alpha1.ObjectSetTemplatePhase {
	return a.Spec.ObjectSetTemplateSpec.Phases
}

func (a *GenericObjectSet) SetArchived() {
	a.Spec.LifecycleState = corev1alpha1.ObjectSetLifecycleStateArchived
}

func (a *GenericObjectSet) IsArchived() bool {
	return a.Spec.LifecycleState == corev1alpha1.ObjectSetLifecycleStateArchived
}

func (a *GenericObjectSet) SetPreviousRevisions(prevObjectSets []genericObjectSet) {
	prevRefs := make([]corev1alpha1.PreviousRevisionReference, len(prevObjectSets))
	for i := range prevObjectSets {
		prevObjSet := prevObjectSets[i]
		currPrevRef := corev1alpha1.PreviousRevisionReference{
			Name: prevObjSet.ClientObject().GetName(),
		}
		prevRefs[i] = currPrevRef
	}
	a.Spec.Previous = prevRefs
}

func (a *GenericObjectSet) GetObjects() ([]objectIdentifier, error) {
	objects := utils.GetObjectsFromPhases(a.Spec.Phases)
	result := make([]objectIdentifier, len(objects))
	for i := range objects {
		unstructuredObj := objects[i].Object
		var objNamespace string
		if len(unstructuredObj.GetNamespace()) == 0 {
			objNamespace = a.Namespace
		} else {
			objNamespace = unstructuredObj.GetNamespace()
		}
		result[i] = objectSetObjectIdentifier{
			name:      unstructuredObj.GetName(),
			namespace: objNamespace,
			group:     unstructuredObj.GroupVersionKind().Group,
			kind:      unstructuredObj.GroupVersionKind().Kind,
		}
	}
	return result, nil
}

func (a *GenericObjectSet) SetPausedByParent() {
	a.Annotations[pausedByParentAnnotation] = pausedByParentTrue
	a.Spec.LifecycleState = corev1alpha1.ObjectSetLifecycleStatePaused
}

func (a *GenericObjectSet) SetActiveByParent() {
	a.Annotations[pausedByParentAnnotation] = pausedByParentFalse
	a.Spec.LifecycleState = corev1alpha1.ObjectSetLifecycleStateActive
}

func (a *GenericObjectSet) GetPausedByParent() bool {
	return a.Spec.LifecycleState == corev1alpha1.ObjectSetLifecycleStatePaused &&
		a.Annotations[pausedByParentAnnotation] == pausedByParentTrue
}

type GenericClusterObjectSet struct {
	corev1alpha1.ClusterObjectSet
}

func (a *GenericClusterObjectSet) SetArchived() {
	a.Spec.LifecycleState = corev1alpha1.ObjectSetLifecycleStateArchived
}

func (a *GenericClusterObjectSet) SetPreviousRevisions(prevObjectSets []genericObjectSet) {
	prevRefs := make([]corev1alpha1.PreviousRevisionReference, len(prevObjectSets))
	for i := range prevObjectSets {
		prevObjSet := prevObjectSets[i]
		currPrevRef := corev1alpha1.PreviousRevisionReference{
			Name: prevObjSet.ClientObject().GetName(),
		}
		prevRefs[i] = currPrevRef
	}
	a.Spec.Previous = prevRefs
}

func (a *GenericClusterObjectSet) GetTemplateSpec() corev1alpha1.ObjectSetTemplateSpec {
	return a.Spec.ObjectSetTemplateSpec
}

func (a *GenericClusterObjectSet) SetTemplateSpec(templateSpec corev1alpha1.ObjectSetTemplateSpec) {
	a.Spec.ObjectSetTemplateSpec = templateSpec
}

func (a *GenericClusterObjectSet) ClientObject() client.Object {
	return &a.ClusterObjectSet
}

func (a *GenericClusterObjectSet) GetConditions() []metav1.Condition {
	return a.Status.Conditions
}

func (a *GenericClusterObjectSet) IsAvailable() bool {
	return meta.IsStatusConditionTrue(
		a.Status.Conditions,
		corev1alpha1.ObjectSetAvailable,
	)
}

func (a *GenericClusterObjectSet) GetPhases() []corev1alpha1.ObjectSetTemplatePhase {
	return a.Spec.Phases
}

func (a *GenericClusterObjectSet) GetRevision() int64 {
	return a.Status.Revision
}

func (a *GenericClusterObjectSet) GetGeneration() int64 {
	return a.Generation
}

func (a *GenericClusterObjectSet) IsArchived() bool {
	return a.Spec.LifecycleState == corev1alpha1.ObjectSetLifecycleStateArchived
}

func (a *GenericClusterObjectSet) IsStatusPaused() bool {
	return meta.IsStatusConditionTrue(
		a.Status.Conditions,
		corev1alpha1.ObjectSetPaused,
	)
}

func (a *GenericClusterObjectSet) SetPaused() {
	a.Spec.LifecycleState = corev1alpha1.ObjectSetLifecycleStatePaused
}

func (a *GenericClusterObjectSet) IsSpecPaused() bool {
	return a.Spec.LifecycleState == corev1alpha1.ObjectSetLifecycleStatePaused
}

func (a *GenericClusterObjectSet) GetActivelyReconciledObjects() []objectIdentifier {
	if a.IsArchived() {
		// If an objectset is archived, it doesnt actively
		// reconcile anything, we just return an empty list
		return nil
	}

	if a.Status.ControllerOf == nil {
		// ActivelyReconciledObjects status is not reported yet
		return nil
	}

	res := make([]objectIdentifier, len(a.Status.ControllerOf))
	for i, reconciledObj := range a.Status.ControllerOf {
		currentObj := objectSetObjectIdentifier{
			kind:      reconciledObj.Kind,
			group:     reconciledObj.Group,
			name:      reconciledObj.Name,
			namespace: reconciledObj.Namespace,
		}
		res[i] = currentObj
	}
	return res
}

func (a *GenericClusterObjectSet) GetObjects() ([]objectIdentifier, error) {
	objects := utils.GetObjectsFromPhases(a.Spec.Phases)
	result := make([]objectIdentifier, len(objects))
	for i := range objects {
		unstructuredObj := objects[i].Object
		var objNamespace string
		if len(unstructuredObj.GetNamespace()) == 0 {
			objNamespace = a.Namespace
		} else {
			objNamespace = unstructuredObj.GetNamespace()
		}

		result[i] = objectSetObjectIdentifier{
			name:      unstructuredObj.GetName(),
			namespace: objNamespace,
			group:     unstructuredObj.GroupVersionKind().Group,
			kind:      unstructuredObj.GroupVersionKind().Kind,
		}
	}
	return result, nil
}

func (a *GenericClusterObjectSet) SetPausedByParent() {
	a.Annotations[pausedByParentAnnotation] = pausedByParentTrue
	a.Spec.LifecycleState = corev1alpha1.ObjectSetLifecycleStatePaused
}

func (a *GenericClusterObjectSet) SetActiveByParent() {
	a.Annotations[pausedByParentAnnotation] = pausedByParentFalse
	a.Spec.LifecycleState = corev1alpha1.ObjectSetLifecycleStateActive
}

func (a *GenericClusterObjectSet) GetPausedByParent() bool {
	return a.Spec.LifecycleState == corev1alpha1.ObjectSetLifecycleStatePaused &&
		a.Annotations[pausedByParentAnnotation] == pausedByParentTrue
}

type objectSetsByRevisionAscending []genericObjectSet

func (a objectSetsByRevisionAscending) Len() int      { return len(a) }
func (a objectSetsByRevisionAscending) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a objectSetsByRevisionAscending) Less(i, j int) bool {
	iObj := a[i]
	jObj := a[j]

	return iObj.GetRevision() < jObj.GetRevision()
}
