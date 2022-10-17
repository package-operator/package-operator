package objectdeployments

import (
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/package-operator/internal/utils"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type genericObjectDeployment interface {
	ClientObject() client.Object
	UpdatePhase()
	GetConditions() *[]metav1.Condition
	GetSelector() metav1.LabelSelector
	GetObjectSetTemplate() corev1alpha1.ObjectSetTemplate
	GetRevisionHistoryLimit() *int32
	SetStatusCollisionCount(*int32)
	GetStatusCollisionCount() *int32
	GetStatusTemplateHash() string
	SetStatusTemplateHash(templateHash string)
	SetObservedGeneration(generation int64)
	GetGeneration() int64
	GetObservedGeneration() int64
}

var (
	_ genericObjectDeployment = (*GenericObjectDeployment)(nil)
	_ genericObjectDeployment = (*GenericClusterObjectDeployment)(nil)
)

type GenericObjectDeployment struct {
	corev1alpha1.ObjectDeployment
}

func (a *GenericObjectDeployment) GetRevisionHistoryLimit() *int32 {
	return a.Spec.RevisionHistoryLimit
}

func (a *GenericObjectDeployment) SetObservedGeneration(generation int64) {
	a.Status.ObservedGeneration = generation
}

func (a *GenericObjectDeployment) GetGeneration() int64 {
	return a.ObjectMeta.Generation
}

func (a *GenericObjectDeployment) GetObservedGeneration() int64 {
	return a.Status.ObservedGeneration
}

func (a *GenericObjectDeployment) SetStatusCollisionCount(cc *int32) {
	a.Status.CollisionCount = cc
}

func (a *GenericObjectDeployment) GetStatusCollisionCount() *int32 {
	return a.Status.CollisionCount
}

func (a *GenericObjectDeployment) ClientObject() client.Object {
	return &a.ObjectDeployment
}

func (a *GenericObjectDeployment) UpdatePhase() {
	availableCond := meta.FindStatusCondition(
		a.Status.Conditions,
		corev1alpha1.ObjectDeploymentAvailable)

	if availableCond != nil {
		if availableCond.Status == metav1.ConditionTrue {
			a.Status.Phase = corev1alpha1.ObjectDeploymentPhaseAvailable
			return
		}
		if availableCond.Status == metav1.ConditionFalse {
			a.Status.Phase = corev1alpha1.ObjectDeploymentPhaseNotReady
			return
		}
	}
	a.Status.Phase = corev1alpha1.ObjectDeploymentPhaseProgressing
}

func (a *GenericObjectDeployment) GetConditions() *[]metav1.Condition {
	return &a.Status.Conditions
}

func (a *GenericObjectDeployment) GetSelector() metav1.LabelSelector {
	return a.Spec.Selector
}

func (a *GenericObjectDeployment) GetObjectSetTemplate() corev1alpha1.ObjectSetTemplate {
	return a.Spec.Template
}

func (a *GenericObjectDeployment) SetStatusTemplateHash(templateHash string) {
	a.Status.TemplateHash = templateHash
}

func (a *GenericObjectDeployment) GetStatusTemplateHash() string {
	return a.Status.TemplateHash
}

type GenericClusterObjectDeployment struct {
	corev1alpha1.ClusterObjectDeployment
}

func (a *GenericClusterObjectDeployment) SetObservedGeneration(generation int64) {
	a.Status.ObservedGeneration = generation
}

func (a *GenericClusterObjectDeployment) GetGeneration() int64 {
	return a.ObjectMeta.Generation
}

func (a *GenericClusterObjectDeployment) GetObservedGeneration() int64 {
	return a.Status.ObservedGeneration
}

func (a *GenericClusterObjectDeployment) GetRevisionHistoryLimit() *int32 {
	return a.Spec.RevisionHistoryLimit
}

func (a *GenericClusterObjectDeployment) SetStatusCollisionCount(cc *int32) {
	a.Status.CollisionCount = cc
}

func (a *GenericClusterObjectDeployment) ClientObject() client.Object {
	return &a.ClusterObjectDeployment
}

func (a *GenericClusterObjectDeployment) UpdatePhase() {
	availableCond := meta.FindStatusCondition(
		a.Status.Conditions,
		corev1alpha1.ObjectDeploymentAvailable)

	if availableCond != nil {
		if availableCond.Status == metav1.ConditionTrue {
			a.Status.Phase = corev1alpha1.ObjectDeploymentPhaseAvailable
			return
		}
		if availableCond.Status == metav1.ConditionFalse {
			a.Status.Phase = corev1alpha1.ObjectDeploymentPhaseNotReady
			return
		}
	}
	a.Status.Phase = corev1alpha1.ObjectDeploymentPhaseProgressing
}

func (a *GenericClusterObjectDeployment) GetConditions() *[]metav1.Condition {
	return &a.Status.Conditions
}

func (a *GenericClusterObjectDeployment) GetSelector() metav1.LabelSelector {
	return a.Spec.Selector
}

func (a *GenericClusterObjectDeployment) GetObjectSetTemplate() corev1alpha1.ObjectSetTemplate {
	return a.Spec.Template
}

func (a *GenericClusterObjectDeployment) GetStatusCollisionCount() *int32 {
	return a.Status.CollisionCount
}

func (a *GenericClusterObjectDeployment) SetStatusTemplateHash(templateHash string) {
	a.Status.TemplateHash = templateHash
}

func (a *GenericClusterObjectDeployment) GetStatusTemplateHash() string {
	return a.Status.TemplateHash
}

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
	IsStatusPaused() bool
	SetPaused()
	IsSpecPaused() bool
	IsAvailable() bool
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
	var result []objectIdentifier
	objects := utils.GetObjectsFromPhases(a.Spec.Phases)
	for i := range objects {
		unstructuredObj := objects[i].Object.DeepCopy()
		if len(unstructuredObj.GetNamespace()) == 0 {
			unstructuredObj.SetNamespace(a.Namespace)
		}
		result = append(result, objectSetObjectIdentifier{
			name:      unstructuredObj.GetName(),
			namespace: unstructuredObj.GetNamespace(),
			group:     unstructuredObj.GroupVersionKind().Group,
			kind:      unstructuredObj.GroupVersionKind().Kind,
		})
	}
	return result, nil
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
	var res []objectIdentifier
	if a.IsArchived() {
		// If an objectset is archived, it doesnt actively
		// reconcile anything, we just return an empty list
		return res
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

func (a *GenericClusterObjectSet) GetObjects() ([]objectIdentifier, error) {
	var result []objectIdentifier
	objects := utils.GetObjectsFromPhases(a.Spec.Phases)
	for i := range objects {
		unstructuredObj := objects[i].Object.DeepCopy()
		if len(unstructuredObj.GetNamespace()) == 0 {
			unstructuredObj.SetNamespace(a.Namespace)
		}

		result = append(result, objectSetObjectIdentifier{
			name:      unstructuredObj.GetName(),
			namespace: unstructuredObj.GetNamespace(),
			group:     unstructuredObj.GroupVersionKind().Group,
			kind:      unstructuredObj.GroupVersionKind().Kind,
		})
	}
	return result, nil
}

type genericObjectSetList interface {
	ClientObjectList() client.ObjectList
	GetItems() []genericObjectSet
}

var (
	_ genericObjectSetList = (*GenericObjectSetList)(nil)
	_ genericObjectSetList = (*GenericClusterObjectSetList)(nil)
)

type GenericObjectSetList struct {
	corev1alpha1.ObjectSetList
}

func (a *GenericObjectSetList) ClientObjectList() client.ObjectList {
	return &a.ObjectSetList
}

func (a *GenericObjectSetList) GetItems() []genericObjectSet {
	out := make([]genericObjectSet, len(a.Items))
	for i := range a.Items {
		out[i] = &GenericObjectSet{
			ObjectSet: a.Items[i],
		}
	}
	return out
}

type GenericClusterObjectSetList struct {
	corev1alpha1.ClusterObjectSetList
}

func (a *GenericClusterObjectSetList) ClientObjectList() client.ObjectList {
	return &a.ClusterObjectSetList
}

func (a *GenericClusterObjectSetList) GetItems() []genericObjectSet {
	out := make([]genericObjectSet, len(a.Items))
	for i := range a.Items {
		out[i] = &GenericClusterObjectSet{
			ClusterObjectSet: a.Items[i],
		}
	}
	return out
}

type objectSetsByRevision []genericObjectSet

func (a objectSetsByRevision) Len() int      { return len(a) }
func (a objectSetsByRevision) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a objectSetsByRevision) Less(i, j int) bool {
	iClientObj := a[i].ClientObject()
	jClientObj := a[j].ClientObject()
	iObj := a[i]
	jObj := a[j]

	if iObj.GetRevision() == 0 ||
		jObj.GetRevision() == 0 {
		return iClientObj.GetCreationTimestamp().UTC().Before(
			jClientObj.GetCreationTimestamp().UTC())
	}

	return iObj.GetRevision() < jObj.GetRevision()
}
