package utils

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func HasSameController(objA, objB metav1.Object) bool {
	controllerA := metav1.GetControllerOf(objA)
	controllerB := metav1.GetControllerOf(objB)
	if controllerA == nil || controllerB == nil {
		return false
	}
	return controllerA.UID == controllerB.UID
}
