package webhooks

import (
	"testing"

	"github.com/stretchr/testify/assert"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

func TestValidateUpdate_ObjectSetPhase(t *testing.T) {
	wh := new(GenericObjectSetPhaseWebhookHandler[corev1alpha1.ObjectSetPhase])

	// test Previous immutable
	oldObj := wh.newObjectSetPhase()
	obj := wh.newObjectSetPhase()
	oldObj.Spec.Previous = []corev1alpha1.PreviousRevisionReference{{Name: "previous-revision"}}
	obj.Spec.Previous = []corev1alpha1.PreviousRevisionReference{{Name: "different-revision"}}
	r := wh.validateUpdate(obj, oldObj)
	assert.False(t, r.Allowed)
	assert.Equal(t, string(r.Result.Reason), errPreviousImmutable.Error())

	// test ObjectSetTemplatePhase immutable
	oldObj = wh.newObjectSetPhase()
	obj = wh.newObjectSetPhase()
	oldObj.Spec.ObjectSetTemplatePhase = corev1alpha1.ObjectSetTemplatePhase{Name: "first-phase"}
	obj.Spec.ObjectSetTemplatePhase = corev1alpha1.ObjectSetTemplatePhase{Name: "second-phase"}
	r = wh.validateUpdate(obj, oldObj)
	assert.False(t, r.Allowed)
	assert.Equal(t, string(r.Result.Reason), errObjectSetTemplatePhaseImmutable.Error())

	// test Revision immutable
	oldObj = wh.newObjectSetPhase()
	obj = wh.newObjectSetPhase()
	oldObj.Spec.Revision = 1
	obj.Spec.Revision = 2
	r = wh.validateUpdate(obj, oldObj)
	assert.False(t, r.Allowed)
	assert.Equal(t, string(r.Result.Reason), errRevisionImmutable.Error())

	// test AvailabilityProbes immutable
	oldObj = wh.newObjectSetPhase()
	obj = wh.newObjectSetPhase()

	cp1 := &corev1alpha1.ProbeConditionSpec{Status: "True"}
	oldObj.Spec.AvailabilityProbes = []corev1alpha1.ObjectSetProbe{{Probes: []corev1alpha1.Probe{{Condition: cp1}}}}

	cp2 := &corev1alpha1.ProbeConditionSpec{Status: "False"}
	obj.Spec.AvailabilityProbes = []corev1alpha1.ObjectSetProbe{{Probes: []corev1alpha1.Probe{{Condition: cp2}}}}

	r = wh.validateUpdate(obj, oldObj)
	assert.False(t, r.Allowed)
	assert.Equal(t, string(r.Result.Reason), errAvailabilityProbesImmutable.Error())
}

func TestValidateUpdate_ClusterObjectSetPhase(t *testing.T) {
	wh := new(GenericObjectSetPhaseWebhookHandler[corev1alpha1.ClusterObjectSetPhase])

	// test Previous immutable
	oldObj := wh.newObjectSetPhase()
	obj := wh.newObjectSetPhase()
	oldObj.Spec.Previous = []corev1alpha1.PreviousRevisionReference{{Name: "previous-revision"}}
	obj.Spec.Previous = []corev1alpha1.PreviousRevisionReference{{Name: "different-revision"}}
	r := wh.validateUpdate(obj, oldObj)
	assert.False(t, r.Allowed)
	assert.Equal(t, string(r.Result.Reason), errPreviousImmutable.Error())

	// test ObjectSetTemplatePhase immutable
	oldObj = wh.newObjectSetPhase()
	obj = wh.newObjectSetPhase()
	oldObj.Spec.ObjectSetTemplatePhase = corev1alpha1.ObjectSetTemplatePhase{Name: "first-phase"}
	obj.Spec.ObjectSetTemplatePhase = corev1alpha1.ObjectSetTemplatePhase{Name: "second-phase"}
	r = wh.validateUpdate(obj, oldObj)
	assert.False(t, r.Allowed)
	assert.Equal(t, string(r.Result.Reason), errObjectSetTemplatePhaseImmutable.Error())

	// test Revision immutable
	oldObj = wh.newObjectSetPhase()
	obj = wh.newObjectSetPhase()
	oldObj.Spec.Revision = 1
	obj.Spec.Revision = 2
	r = wh.validateUpdate(obj, oldObj)
	assert.False(t, r.Allowed)
	assert.Equal(t, string(r.Result.Reason), errRevisionImmutable.Error())

	// test AvailabilityProbes immutable
	oldObj = wh.newObjectSetPhase()
	obj = wh.newObjectSetPhase()

	cp1 := &corev1alpha1.ProbeConditionSpec{Status: "True"}
	oldObj.Spec.AvailabilityProbes = []corev1alpha1.ObjectSetProbe{{Probes: []corev1alpha1.Probe{{Condition: cp1}}}}

	cp2 := &corev1alpha1.ProbeConditionSpec{Status: "False"}
	obj.Spec.AvailabilityProbes = []corev1alpha1.ObjectSetProbe{{Probes: []corev1alpha1.Probe{{Condition: cp2}}}}

	r = wh.validateUpdate(obj, oldObj)
	assert.False(t, r.Allowed)
	assert.Equal(t, string(r.Result.Reason), errAvailabilityProbesImmutable.Error())

}
