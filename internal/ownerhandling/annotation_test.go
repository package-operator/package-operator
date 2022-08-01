package ownerhandling

import (
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	"testing"
)

func TestGetControllerReference(t *testing.T) {
	ownerRef := annotationOwnerRef{
		APIVersion: "v1",
		Kind:       "ConfigMap",
		UID:        types.UID("1234"),
		Name:       "cmtest",
		Namespace:  "cmtestns",
		Controller: pointer.BoolPtr(true),
	}
	s := &OwnerStrategyAnnotation{}
	obj := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "testns",
		},
	}
	s.setOwnerReferences(obj, []annotationOwnerRef{ownerRef})
	gottenOwnerRefs := s.getOwnerReferences(obj)
	assert.Equal(t, len(gottenOwnerRefs), 1)
	assert.Equal(t, gottenOwnerRefs[0], ownerRef)
}
