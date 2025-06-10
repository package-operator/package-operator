package adaptermocks

import (
	"github.com/stretchr/testify/mock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/internal/adapters"
)

var (
	_ adapters.ObjectDeploymentAccessor = (*ObjectDeploymentMock)(nil)
	_ adapters.ObjectDeploymentAccessor = (*ObjectSetDeploymentMock)(nil)
)

type ObjectDeploymentMock struct {
	mock.Mock
}

func (o *ObjectDeploymentMock) GetSpecPaused() bool {
	args := o.Called()
	return args.Get(0).(bool)
}

func (o *ObjectDeploymentMock) SetSpecPaused(paused bool) {
	o.Called(paused)
}

func (o *ObjectDeploymentMock) SetStatusRevision(r int64) {
	o.Called(r)
}

func (o *ObjectDeploymentMock) GetStatusRevision() int64 {
	args := o.Called()
	return args.Get(0).(int64)
}

func (o *ObjectDeploymentMock) ClientObject() client.Object {
	args := o.Called()
	return args.Get(0).(client.Object)
}

func (o *ObjectDeploymentMock) GetStatusTemplateHash() string {
	args := o.Called()
	return args.Get(0).(string)
}

func (o *ObjectDeploymentMock) SetStatusTemplateHash(templateHash string) {
	o.Called(templateHash)
}

func (o *ObjectDeploymentMock) GetGeneration() int64 {
	args := o.Called()
	return args.Get(0).(int64)
}

func (o *ObjectDeploymentMock) SetStatusConditions(conds ...metav1.Condition) {
	o.Called(conds)
}

func (o *ObjectDeploymentMock) SetObservedGeneration(a int64) {
	o.Called(a)
}

func (o *ObjectDeploymentMock) SetStatusCollisionCount(a *int32) {
	o.Called(a)
}

func (o *ObjectDeploymentMock) GetObservedGeneration() int64 {
	args := o.Called()
	return args.Get(0).(int64)
}

func (o *ObjectDeploymentMock) GetSpecRevisionHistoryLimit() *int32 {
	args := o.Called()
	return args.Get(0).(*int32)
}

func (o *ObjectDeploymentMock) GetStatusCollisionCount() *int32 {
	args := o.Called()
	res, _ := args.Get(0).(*int32)
	return res
}

func (o *ObjectDeploymentMock) GetSpecSelector() metav1.LabelSelector {
	args := o.Called()
	return args.Get(0).(metav1.LabelSelector)
}

func (o *ObjectDeploymentMock) SetSpecSelector(labels map[string]string) {
	o.Called(labels)
}

func (o *ObjectDeploymentMock) GetStatusConditions() *[]metav1.Condition {
	args := o.Called()
	return args.Get(0).(*[]metav1.Condition)
}

func (o *ObjectDeploymentMock) GetSpecObjectSetTemplate() corev1alpha1.ObjectSetTemplate {
	args := o.Called()
	return args.Get(0).(corev1alpha1.ObjectSetTemplate)
}

func (o *ObjectDeploymentMock) SetStatusControllerOf(a []corev1alpha1.ControlledObjectReference) {
	o.Called(a)
}

func (o *ObjectDeploymentMock) GetStatusControllerOf() []corev1alpha1.ControlledObjectReference {
	args := o.Called()
	return args.Get(0).([]corev1alpha1.ControlledObjectReference)
}

func (o *ObjectDeploymentMock) RemoveStatusConditions(condTypes ...string) {
	o.Called(condTypes)
}

func (o *ObjectDeploymentMock) GetSpecTemplateSpec() corev1alpha1.ObjectSetTemplateSpec {
	args := o.Called()
	return args.Get(0).(corev1alpha1.ObjectSetTemplateSpec)
}

func (o *ObjectDeploymentMock) SetSpecTemplateSpec(a corev1alpha1.ObjectSetTemplateSpec) {
	o.Called(a)
}

type ObjectSetDeploymentMock struct {
	mock.Mock
}

func (o *ObjectSetDeploymentMock) RemoveStatusConditions(condTypes ...string) {
	o.Called(condTypes)
}

func (o *ObjectSetDeploymentMock) GetSpecPaused() bool {
	args := o.Called()
	return args.Get(0).(bool)
}

func (o *ObjectSetDeploymentMock) SetSpecPaused(paused bool) {
	o.Called(paused)
}

func (o *ObjectSetDeploymentMock) SetStatusRevision(r int64) {
	o.Called(r)
}

func (o *ObjectSetDeploymentMock) GetStatusRevision() int64 {
	args := o.Called()
	return args.Get(0).(int64)
}

func (o *ObjectSetDeploymentMock) ClientObject() client.Object {
	args := o.Called()
	return args.Get(0).(client.Object)
}

func (o *ObjectSetDeploymentMock) GetStatusTemplateHash() string {
	args := o.Called()
	return args.Get(0).(string)
}

func (o *ObjectSetDeploymentMock) SetStatusTemplateHash(templateHash string) {
	o.Called(templateHash)
}

func (o *ObjectSetDeploymentMock) GetGeneration() int64 {
	args := o.Called()
	return args.Get(0).(int64)
}

func (o *ObjectSetDeploymentMock) SetStatusConditions(conds ...metav1.Condition) {
	o.Called(conds)
}

func (o *ObjectSetDeploymentMock) SetObservedGeneration(a int64) {
	o.Called(a)
}

func (o *ObjectSetDeploymentMock) SetStatusCollisionCount(a *int32) {
	o.Called(a)
}

func (o *ObjectSetDeploymentMock) GetObservedGeneration() int64 {
	args := o.Called()
	return args.Get(0).(int64)
}

func (o *ObjectSetDeploymentMock) GetSpecRevisionHistoryLimit() *int32 {
	args := o.Called()
	return args.Get(0).(*int32)
}

func (o *ObjectSetDeploymentMock) GetStatusCollisionCount() *int32 {
	args := o.Called()
	res, _ := args.Get(0).(*int32)
	return res
}

func (o *ObjectSetDeploymentMock) GetSpecSelector() metav1.LabelSelector {
	args := o.Called()
	return args.Get(0).(metav1.LabelSelector)
}

func (o *ObjectSetDeploymentMock) SetSpecSelector(labels map[string]string) {
	o.Called(labels)
}

func (o *ObjectSetDeploymentMock) GetStatusConditions() *[]metav1.Condition {
	args := o.Called()
	return args.Get(0).(*[]metav1.Condition)
}

func (o *ObjectSetDeploymentMock) GetSpecObjectSetTemplate() corev1alpha1.ObjectSetTemplate {
	args := o.Called()
	return args.Get(0).(corev1alpha1.ObjectSetTemplate)
}

func (o *ObjectSetDeploymentMock) SetStatusControllerOf(a []corev1alpha1.ControlledObjectReference) {
	o.Called(a)
}

func (o *ObjectSetDeploymentMock) GetStatusControllerOf() []corev1alpha1.ControlledObjectReference {
	args := o.Called()
	return args.Get(0).([]corev1alpha1.ControlledObjectReference)
}

func (o *ObjectSetDeploymentMock) GetSpecTemplateSpec() corev1alpha1.ObjectSetTemplateSpec {
	args := o.Called()
	return args.Get(0).(corev1alpha1.ObjectSetTemplateSpec)
}

func (o *ObjectSetDeploymentMock) SetSpecTemplateSpec(a corev1alpha1.ObjectSetTemplateSpec) {
	o.Called(a)
}
