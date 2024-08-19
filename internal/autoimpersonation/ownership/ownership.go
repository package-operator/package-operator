package ownership

import (
	"errors"
	"fmt"
	"reflect"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"package-operator.run/apis/core/v1alpha1"
)

var errUnsupportedOwnerKind = errors.New("unsupported owner kind")

func VerifyOwnership(obj, owner client.Object) (bool, error) {
	ver, ok := verifiers[owner.GetObjectKind().GroupVersionKind().Kind]
	if !ok {
		return false, fmt.Errorf("%w: %s", errUnsupportedOwnerKind, owner.GetObjectKind().GroupVersionKind().Kind)
	}

	return ver(obj, owner), nil
}

type verifier func(obj, owner client.Object) bool

// constant map.
var verifiers = map[string]verifier{
	"Package":                 verifyPackage,
	"ClusterPackage":          verifyClusterPackage,
	"ObjectDeployment":        verifyObjectDeployment,
	"ClusterObjectDeployment": verifyClusterObjectDeployment,
	"ObjectSet":               verifyObjectSet,
	"ClusterObjectSet":        verifyClusterObjectSet,
	"ObjectTemplate":          verifyObjectTemplate,
	"ClusterObjectTemplate":   verifyClusterObjectTemplate,
}

func verifyPackage(obj, owner client.Object) bool {
	return obj.GetObjectKind().GroupVersionKind().Kind == "ObjectDeployment" &&
		owner.GetName() == obj.GetName() &&
		owner.GetNamespace() == obj.GetNamespace()
}

func verifyClusterPackage(obj, owner client.Object) bool {
	return obj.GetObjectKind().GroupVersionKind().Kind == "ClusterObjectDeployment" &&
		owner.GetName() == obj.GetName()
}

func verifyObjectDeployment(obj, owner client.Object) bool {
	objectDeployment := owner.(*v1alpha1.ObjectDeployment)
	return verifyTwoWayOwnership(obj, owner, objectDeployment.Status.ControllerOf)
}

func verifyClusterObjectDeployment(obj, owner client.Object) bool {
	objectDeployment := owner.(*v1alpha1.ObjectDeployment)
	return verifyTwoWayOwnership(obj, owner, objectDeployment.Status.ControllerOf)
}

func verifyObjectSet(obj, owner client.Object) bool {
	objectSet := owner.(*v1alpha1.ObjectSet)
	return verifyTwoWayOwnership(obj, owner, objectSet.Status.ControllerOf)
}

func verifyClusterObjectSet(obj, owner client.Object) bool {
	objectSet := owner.(*v1alpha1.ClusterObjectSet)
	return verifyTwoWayOwnership(obj, owner, objectSet.Status.ControllerOf)
}

func verifyObjectTemplate(obj, owner client.Object) bool {
	objectTemplate := owner.(*v1alpha1.ObjectTemplate)
	return verifyTwoWayOwnership(obj, owner, []v1alpha1.ControlledObjectReference{objectTemplate.Status.ControllerOf})
}

func verifyClusterObjectTemplate(obj, owner client.Object) bool {
	objectTemplate := owner.(*v1alpha1.ClusterObjectTemplate)
	return verifyTwoWayOwnership(obj, owner, []v1alpha1.ControlledObjectReference{objectTemplate.Status.ControllerOf})
}

func verifyTwoWayOwnership(
	obj, owner client.Object,
	controllerOf []v1alpha1.ControlledObjectReference,
) bool {
	return findOwnerReference(obj, owner) && findControllerOf(obj, controllerOf)
}

func findOwnerReference(obj, owner client.Object) bool {
	for _, ref := range obj.GetOwnerReferences() {
		if ref.UID == owner.GetUID() && ref.Controller != nil && *ref.Controller {
			return true
		}
	}
	return false
}

func findControllerOf(obj client.Object, controllerOf []v1alpha1.ControlledObjectReference) bool {
	gvk := obj.GetObjectKind().GroupVersionKind()
	objRef := v1alpha1.ControlledObjectReference{
		Kind:      gvk.Kind,
		Group:     gvk.Group,
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
	}

	for _, ref := range controllerOf {
		if reflect.DeepEqual(objRef, ref) {
			return true
		}
	}
	return false
}
