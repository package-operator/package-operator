package webhooks

import (
	"testing"

	"github.com/stretchr/testify/assert"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

func TestValidateUpdate_ObjectSet(t *testing.T) {
	wh := new(GenericObjectSetWebhookHandler[corev1alpha1.ObjectSet])

	t.Run("previous immutable", func(t *testing.T) {
		oldObj := wh.newObjectSet()
		obj := wh.newObjectSet()
		oldObj.Spec.Previous = []corev1alpha1.PreviousRevisionReference{{Name: "previous-revision"}}
		obj.Spec.Previous = []corev1alpha1.PreviousRevisionReference{{Name: "different-revision"}}
		r := wh.validateUpdate(obj, oldObj)
		assert.False(t, r.Allowed)
		assert.Equal(t, string(r.Result.Reason), errPreviousImmutable.Error())
	})

	t.Run("ObjectSetTemplatePhase immutable", func(t *testing.T) {
		oldObj := wh.newObjectSet()
		obj := wh.newObjectSet()
		p1 := []corev1alpha1.ObjectSetTemplatePhase{{Name: "first-phase"}}
		oldObj.Spec.ObjectSetTemplateSpec = corev1alpha1.ObjectSetTemplateSpec{Phases: p1}
		p2 := []corev1alpha1.ObjectSetTemplatePhase{{Name: "second-phase"}}
		obj.Spec.ObjectSetTemplateSpec = corev1alpha1.ObjectSetTemplateSpec{Phases: p2}
		r := wh.validateUpdate(obj, oldObj)
		assert.False(t, r.Allowed)
		assert.Equal(t, string(r.Result.Reason), errObjectSetTemplateSpecImmutable.Error())
	})
}
