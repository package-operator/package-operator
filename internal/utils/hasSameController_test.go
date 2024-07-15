package utils

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestHasSameController(t *testing.T) {
	t.Parallel()
	boolPtr := func(b bool) *bool { return &b }
	// Helper function to create an object with a specified controller UID
	createObjectWithController := func(uid types.UID) metav1.Object {
		return &metav1.ObjectMeta{
			OwnerReferences: []metav1.OwnerReference{
				{
					UID:        uid,
					Controller: boolPtr(true),
				},
			},
		}
	}

	// Helper function to create an object without a controller
	createObjectWithoutController := func() metav1.Object {
		return &metav1.ObjectMeta{}
	}

	tests := []struct {
		name   string
		objA   metav1.Object
		objB   metav1.Object
		result bool
	}{
		{
			name:   "Both objects have the same controller",
			objA:   createObjectWithController("controller-1"),
			objB:   createObjectWithController("controller-1"),
			result: true,
		},
		{
			name:   "Objects have different controllers",
			objA:   createObjectWithController("controller-1"),
			objB:   createObjectWithController("controller-2"),
			result: false,
		},
		{
			name:   "First object does not have a controller",
			objA:   createObjectWithoutController(),
			objB:   createObjectWithController("controller-1"),
			result: false,
		},
		{
			name:   "Second object does not have a controller",
			objA:   createObjectWithController("controller-1"),
			objB:   createObjectWithoutController(),
			result: false,
		},
		{
			name:   "Neither object has a controller",
			objA:   createObjectWithoutController(),
			objB:   createObjectWithoutController(),
			result: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := HasSameController(tt.objA, tt.objB); got != tt.result {
				t.Errorf("HasSameController() = %v, want %v", got, tt.result)
			}
		})
	}
}
