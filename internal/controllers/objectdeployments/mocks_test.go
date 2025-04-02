package objectdeployments

import (
	"context"

	"github.com/stretchr/testify/mock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/internal/adapters"
)

var (
	_ objectDeploymentAccessor = (*genericObjectDeploymentMock)(nil)
	_ objectSetSubReconciler   = (*objectSetSubReconcilerMock)(nil)
	_ objectDeploymentAccessor = (*genericObjectSetDeploymentMock)(nil)
)

type genericObjectDeploymentMock struct {
	mock.Mock
}

func (o *genericObjectDeploymentMock) GetSpecPaused() bool {
	args := o.Called()
	return args.Get(0).(bool)
}

func (o *genericObjectDeploymentMock) SetSpecPaused(paused bool) {
	o.Called(paused)
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

func (o *genericObjectDeploymentMock) RemoveStatusConditions(condTypes ...string) {
	o.Called(condTypes)
}

type genericObjectSetDeploymentMock struct {
	mock.Mock
}

func (o *genericObjectSetDeploymentMock) RemoveStatusConditions(condTypes ...string) {
	o.Called(condTypes)
}

func (o *genericObjectSetDeploymentMock) GetSpecPaused() bool {
	args := o.Called()
	return args.Get(0).(bool)
}

func (o *genericObjectSetDeploymentMock) SetSpecPaused(paused bool) {
	o.Called(paused)
}

func (o *genericObjectSetDeploymentMock) SetStatusRevision(r int64) {
	o.Called(r)
}

func (o *genericObjectSetDeploymentMock) ClientObject() client.Object {
	args := o.Called()
	return args.Get(0).(client.Object)
}

func (o *genericObjectSetDeploymentMock) GetStatusTemplateHash() string {
	args := o.Called()
	return args.Get(0).(string)
}

func (o *genericObjectSetDeploymentMock) SetStatusTemplateHash(templateHash string) {
	o.Called(templateHash)
}

func (o *genericObjectSetDeploymentMock) GetGeneration() int64 {
	args := o.Called()
	return args.Get(0).(int64)
}

func (o *genericObjectSetDeploymentMock) SetStatusConditions(conds ...metav1.Condition) {
	o.Called(conds)
}

func (o *genericObjectSetDeploymentMock) SetObservedGeneration(a int64) {
	o.Called(a)
}

func (o *genericObjectSetDeploymentMock) SetStatusCollisionCount(a *int32) {
	o.Called(a)
}

func (o *genericObjectSetDeploymentMock) GetObservedGeneration() int64 {
	args := o.Called()
	return args.Get(0).(int64)
}

func (o *genericObjectSetDeploymentMock) GetRevisionHistoryLimit() *int32 {
	args := o.Called()
	return args.Get(0).(*int32)
}

func (o *genericObjectSetDeploymentMock) GetStatusCollisionCount() *int32 {
	args := o.Called()
	res, _ := args.Get(0).(*int32)
	return res
}

func (o *genericObjectSetDeploymentMock) GetSelector() metav1.LabelSelector {
	args := o.Called()
	return args.Get(0).(metav1.LabelSelector)
}

func (o *genericObjectSetDeploymentMock) GetConditions() *[]metav1.Condition {
	args := o.Called()
	return args.Get(0).(*[]metav1.Condition)
}

func (o *genericObjectSetDeploymentMock) GetObjectSetTemplate() corev1alpha1.ObjectSetTemplate {
	args := o.Called()
	return args.Get(0).(corev1alpha1.ObjectSetTemplate)
}

func (o *genericObjectSetDeploymentMock) SetStatusControllerOf(a []corev1alpha1.ControlledObjectReference) {
	o.Called(a)
}

func (o *genericObjectSetDeploymentMock) GetStatusControllerOf() []corev1alpha1.ControlledObjectReference {
	args := o.Called()
	return args.Get(0).([]corev1alpha1.ControlledObjectReference)
}

type objectSetSubReconcilerMock struct {
	mock.Mock
}

func (o *objectSetSubReconcilerMock) Reconcile(
	ctx context.Context, currentObjectSet adapters.ObjectSetAccessor,
	prevObjectSets []adapters.ObjectSetAccessor, objectDeployment objectDeploymentAccessor,
) (ctrl.Result, error) {
	args := o.Called(ctx, currentObjectSet, prevObjectSets, objectDeployment)
	err, _ := args.Get(1).(error)
	return args.Get(0).(ctrl.Result), err
}
