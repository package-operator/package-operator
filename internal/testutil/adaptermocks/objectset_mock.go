package adaptermocks

import (
	"github.com/stretchr/testify/mock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/internal/adapters"
)

var _ adapters.ObjectSetAccessor = (*ObjectSetMock)(nil)

type ObjectSetMock struct {
	metav1.ObjectMeta
	metav1.TypeMeta
	mock.Mock
}

func (o *ObjectSetMock) GetSpecPrevious() []corev1alpha1.PreviousRevisionReference {
	args := o.Called()
	return args.Get(0).([]corev1alpha1.PreviousRevisionReference)
}

func (o *ObjectSetMock) SetSpecPhases(phases []corev1alpha1.ObjectSetTemplatePhase) {
	o.Called(phases)
}

func (o *ObjectSetMock) GetAvailabilityProbes() []corev1alpha1.ObjectSetProbe {
	args := o.Called()
	return args.Get(0).([]corev1alpha1.ObjectSetProbe)
}

func (o *ObjectSetMock) GetSpecSuccessDelaySeconds() int32 {
	args := o.Called()
	return args.Get(0).(int32)
}

func (o *ObjectSetMock) SetStatusRevision(revision int64) {
	o.Called(revision)
}

func (o *ObjectSetMock) GetStatusRemotePhases() []corev1alpha1.RemotePhaseReference {
	args := o.Called()
	return args.Get(0).([]corev1alpha1.RemotePhaseReference)
}

func (o *ObjectSetMock) SetStatusRemotePhases(references []corev1alpha1.RemotePhaseReference) {
	o.Called(references)
}

func (o *ObjectSetMock) GetStatusControllerOf() []corev1alpha1.ControlledObjectReference {
	args := o.Called()
	return args.Get(0).([]corev1alpha1.ControlledObjectReference)
}

func (o *ObjectSetMock) SetStatusControllerOf(references []corev1alpha1.ControlledObjectReference) {
	o.Called(references)
}

func (o *ObjectSetMock) ClientObject() client.Object {
	args := o.Called()
	return args.Get(0).(client.Object)
}

func (o *ObjectSetMock) GetStatusRevision() int64 {
	args := o.Called()
	return args.Get(0).(int64)
}

func (o *ObjectSetMock) GetGeneration() int64 {
	args := o.Called()
	return args.Get(0).(int64)
}

func (o *ObjectSetMock) IsStatusPaused() bool {
	args := o.Called()
	return args.Bool(0)
}

func (o *ObjectSetMock) IsSpecPaused() bool {
	args := o.Called()
	return args.Bool(0)
}

func (o *ObjectSetMock) SetSpecPaused() {
	o.Called()
}

func (o *ObjectSetMock) IsSpecAvailable() bool {
	args := o.Called()
	return args.Bool(0)
}

func (o *ObjectSetMock) GetStatusConditions() *[]metav1.Condition {
	args := o.Called()
	return args.Get(0).(*[]metav1.Condition)
}

func (o *ObjectSetMock) IsSpecArchived() bool {
	args := o.Called()
	return args.Bool(0)
}

func (o *ObjectSetMock) SetSpecArchived() {
	o.Called()
}

func (o *ObjectSetMock) GetSpecPhases() []corev1alpha1.ObjectSetTemplatePhase {
	args := o.Called()
	return args.Get(0).([]corev1alpha1.ObjectSetTemplatePhase)
}

func (o *ObjectSetMock) SetSpecPreviousRevisions(prev []adapters.ObjectSetAccessor) {
	o.Called(prev)
}

func (o *ObjectSetMock) SetSpecTemplateSpec(templateSpec corev1alpha1.ObjectSetTemplateSpec) {
	o.Called(templateSpec)
}

func (o *ObjectSetMock) GetSpecTemplateSpec() corev1alpha1.ObjectSetTemplateSpec {
	args := o.Called()
	return args.Get(0).(corev1alpha1.ObjectSetTemplateSpec)
}

func (o *ObjectSetMock) SetSpecActiveByParent() {
	o.Called()
}

func (o *ObjectSetMock) SetSpecPausedByParent() {
	o.Called()
}

func (o *ObjectSetMock) GetSpecPausedByParent() bool {
	args := o.Called()
	return args.Get(0).(bool)
}

func (o *ObjectSetMock) GetSpecRevision() int64 {
	args := o.Called()
	return args.Get(0).(int64)
}

func (o *ObjectSetMock) SetSpecRevision(revision int64) {
	o.Called(revision)
}

func (o *ObjectSetMock) DeepCopyObject() runtime.Object {
	args := o.Called()
	return args.Get(0).(runtime.Object)
}
