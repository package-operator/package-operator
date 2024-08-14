package objectdeployments

import (
	"context"

	"github.com/stretchr/testify/mock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

var (
	_ genericObjectSet         = (*genericObjectSetMock)(nil)
	_ objectDeploymentAccessor = (*genericObjectDeploymentMock)(nil)
	_ objectSetSubReconciler   = (*objectSetSubReconcilerMock)(nil)
)

type genericObjectSetMock struct {
	mock.Mock
}

func (o *genericObjectSetMock) ClientObject() client.Object {
	args := o.Called()
	return args.Get(0).(client.Object)
}

func (o *genericObjectSetMock) GetRevision() int64 {
	args := o.Called()
	return args.Get(0).(int64)
}

func (o *genericObjectSetMock) GetGeneration() int64 {
	args := o.Called()
	return args.Get(0).(int64)
}

func (o *genericObjectSetMock) IsStatusPaused() bool {
	args := o.Called()
	return args.Bool(0)
}

func (o *genericObjectSetMock) IsSpecPaused() bool {
	args := o.Called()
	return args.Bool(0)
}

func (o *genericObjectSetMock) SetPaused() {
	o.Called()
}

func (o *genericObjectSetMock) IsAvailable() bool {
	args := o.Called()
	return args.Bool(0)
}

func (o *genericObjectSetMock) GetConditions() []metav1.Condition {
	args := o.Called()
	return args.Get(0).([]metav1.Condition)
}

func (o *genericObjectSetMock) IsArchived() bool {
	args := o.Called()
	return args.Bool(0)
}

func (o *genericObjectSetMock) SetArchived() {
	o.Called()
}

func (o *genericObjectSetMock) GetPhases() []corev1alpha1.ObjectSetTemplatePhase {
	args := o.Called()
	return args.Get(0).([]corev1alpha1.ObjectSetTemplatePhase)
}

func (o *genericObjectSetMock) GetActivelyReconciledObjects() []objectIdentifier {
	args := o.Called()
	return args.Get(0).([]objectIdentifier)
}

func (o *genericObjectSetMock) GetObjects() ([]objectIdentifier, error) {
	args := o.Called()
	err, _ := args.Get(1).(error)
	return args.Get(0).([]objectIdentifier), err
}

func (o *genericObjectSetMock) SetPreviousRevisions(prev []genericObjectSet) {
	o.Called(prev)
}

func (o *genericObjectSetMock) SetTemplateSpec(templateSpec corev1alpha1.ObjectSetTemplateSpec) {
	o.Called(templateSpec)
}

func (o *genericObjectSetMock) GetTemplateSpec() corev1alpha1.ObjectSetTemplateSpec {
	args := o.Called()
	return args.Get(0).(corev1alpha1.ObjectSetTemplateSpec)
}

type genericObjectDeploymentMock struct {
	mock.Mock
}

func (o *genericObjectDeploymentMock) SetStatusRevision(r int64) {
	o.Called(r)
}

func (o *genericObjectDeploymentMock) ClientObject() client.Object {
	args := o.Called()
	return args.Get(0).(client.Object)
}

func (o *genericObjectDeploymentMock) GetStatusTemplateHash() string {
	args := o.Called()
	return args.Get(0).(string)
}

func (o *genericObjectDeploymentMock) SetStatusTemplateHash(templateHash string) {
	o.Called(templateHash)
}

func (o *genericObjectDeploymentMock) GetGeneration() int64 {
	args := o.Called()
	return args.Get(0).(int64)
}

func (o *genericObjectDeploymentMock) SetStatusConditions(conds ...metav1.Condition) {
	o.Called(conds)
}

func (o *genericObjectDeploymentMock) SetObservedGeneration(a int64) {
	o.Called(a)
}

func (o *genericObjectDeploymentMock) SetStatusCollisionCount(a *int32) {
	o.Called(a)
}

func (o *genericObjectDeploymentMock) GetObservedGeneration() int64 {
	args := o.Called()
	return args.Get(0).(int64)
}

func (o *genericObjectDeploymentMock) GetRevisionHistoryLimit() *int32 {
	args := o.Called()
	return args.Get(0).(*int32)
}

func (o *genericObjectDeploymentMock) GetStatusCollisionCount() *int32 {
	args := o.Called()
	res, _ := args.Get(0).(*int32)
	return res
}

func (o *genericObjectDeploymentMock) GetSelector() metav1.LabelSelector {
	args := o.Called()
	return args.Get(0).(metav1.LabelSelector)
}

func (o *genericObjectDeploymentMock) UpdatePhase() {
	o.Called()
}

func (o *genericObjectDeploymentMock) GetConditions() *[]metav1.Condition {
	args := o.Called()
	return args.Get(0).(*[]metav1.Condition)
}

func (o *genericObjectDeploymentMock) GetObjectSetTemplate() corev1alpha1.ObjectSetTemplate {
	args := o.Called()
	return args.Get(0).(corev1alpha1.ObjectSetTemplate)
}

func (o *genericObjectDeploymentMock) SetStatusControllerOf(a []corev1alpha1.ControlledObjectReference) {
	o.Called(a)
}

func (o *genericObjectDeploymentMock) GetStatusControllerOf() []corev1alpha1.ControlledObjectReference {
	args := o.Called()
	return args.Get(0).([]corev1alpha1.ControlledObjectReference)
}

type objectSetSubReconcilerMock struct {
	mock.Mock
}

func (o *objectSetSubReconcilerMock) Reconcile(ctx context.Context,
	currentObjectSet genericObjectSet, prevObjectSets []genericObjectSet, objectDeployment objectDeploymentAccessor,
) (ctrl.Result, error) {
	args := o.Called(ctx, currentObjectSet, prevObjectSets, objectDeployment)
	err, _ := args.Get(1).(error)
	return args.Get(0).(ctrl.Result), err
}
