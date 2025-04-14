package adaptermocks

import (
	"github.com/stretchr/testify/mock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/internal/adapters"
)

var _ adapters.ObjectSetAccessor = (*ObjectSetMock)(nil)

type ObjectSetMock struct {
	mock.Mock
}

func (o *ObjectSetMock) GetPrevious() []corev1alpha1.PreviousRevisionReference {
	args := o.Called()
	return args.Get(0).([]corev1alpha1.PreviousRevisionReference)
}

func (o *ObjectSetMock) SetPhases(phases []corev1alpha1.ObjectSetTemplatePhase) {
	o.Called(phases)
}

func (o *ObjectSetMock) GetAvailabilityProbes() []corev1alpha1.ObjectSetProbe {
	args := o.Called()
	return args.Get(0).([]corev1alpha1.ObjectSetProbe)
}

func (o *ObjectSetMock) GetSuccessDelaySeconds() int32 {
	args := o.Called()
	return args.Get(0).(int32)
}

func (o *ObjectSetMock) SetRevision(revision int64) {
	o.Called(revision)
}

func (o *ObjectSetMock) GetRemotePhases() []corev1alpha1.RemotePhaseReference {
	args := o.Called()
	return args.Get(0).([]corev1alpha1.RemotePhaseReference)
}

func (o *ObjectSetMock) SetRemotePhases(references []corev1alpha1.RemotePhaseReference) {
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

func (o *ObjectSetMock) GetRevision() int64 {
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

func (o *ObjectSetMock) SetPaused() {
	o.Called()
}

func (o *ObjectSetMock) IsAvailable() bool {
	args := o.Called()
	return args.Bool(0)
}

func (o *ObjectSetMock) GetConditions() *[]metav1.Condition {
	args := o.Called()
	return args.Get(0).(*[]metav1.Condition)
}

func (o *ObjectSetMock) IsArchived() bool {
	args := o.Called()
	return args.Bool(0)
}

func (o *ObjectSetMock) SetArchived() {
	o.Called()
}

func (o *ObjectSetMock) GetPhases() []corev1alpha1.ObjectSetTemplatePhase {
	args := o.Called()
	return args.Get(0).([]corev1alpha1.ObjectSetTemplatePhase)
}

func (o *ObjectSetMock) SetPreviousRevisions(prev []adapters.ObjectSetAccessor) {
	o.Called(prev)
}

func (o *ObjectSetMock) SetTemplateSpec(templateSpec corev1alpha1.ObjectSetTemplateSpec) {
	o.Called(templateSpec)
}

func (o *ObjectSetMock) GetTemplateSpec() corev1alpha1.ObjectSetTemplateSpec {
	args := o.Called()
	return args.Get(0).(corev1alpha1.ObjectSetTemplateSpec)
}

func (o *ObjectSetMock) SetActiveByParent() {
	o.Called()
}

func (o *ObjectSetMock) SetPausedByParent() {
	o.Called()
}

func (o *ObjectSetMock) GetPausedByParent() bool {
	args := o.Called()
	return args.Get(0).(bool)
}
