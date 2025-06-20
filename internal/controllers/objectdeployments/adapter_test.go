package objectdeployments

import (
	"testing"

	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	apis "package-operator.run/apis"
	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/internal/adapters"
)

var testScheme *runtime.Scheme

func init() {
	requiredSchemes := runtime.SchemeBuilder{
		scheme.AddToScheme,
		apis.AddToScheme,
	}
	testScheme = runtime.NewScheme()
	if err := requiredSchemes.AddToScheme(testScheme); err != nil {
		panic(err)
	}
}

func genObjectSet(
	name string,
	namespace string,
	phaseAndObjectMap map[string][]client.Object,
	controllerOf []corev1alpha1.ControlledObjectReference,
) objectSetGetter {
	objectSet := &corev1alpha1.ObjectSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: corev1alpha1.ObjectSetSpec{
			ObjectSetTemplateSpec: corev1alpha1.ObjectSetTemplateSpec{},
		},
		Status: corev1alpha1.ObjectSetStatus{
			ControllerOf: controllerOf,
		},
	}

	phases := make([]corev1alpha1.ObjectSetTemplatePhase, 0, len(phaseAndObjectMap))
	for phaseName, objects := range phaseAndObjectMap {
		currPhase := corev1alpha1.ObjectSetTemplatePhase{
			Name:    phaseName,
			Objects: clientObjs2ObjectSetObjects(objects),
		}
		phases = append(phases, currPhase)
	}
	objectSet.Spec.Phases = phases
	return newObjectSetGetter(&adapters.ObjectSetAdapter{
		ObjectSet: *objectSet,
	})
}

func clientObjs2ObjectSetObjects(runtimeObjs []client.Object) []corev1alpha1.ObjectSetObject {
	res := make([]corev1alpha1.ObjectSetObject, len(runtimeObjs))
	for i := range runtimeObjs {
		obj := runtimeObjs[i]
		unstructuredObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
		if err != nil {
			panic(err)
		}
		res[i] = corev1alpha1.ObjectSetObject{
			Object: unstructured.Unstructured{Object: unstructuredObj},
		}
	}
	return res
}

func TestGetObjectSetObjects(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		objectSet         objectSetGetter
		expectedObjectIDs []string
	}{
		{
			objectSet: genObjectSet("test-1", "default", map[string][]client.Object{
				"phase1": {
					cmTemplate("cm1", "namespace1", t),
					cmTemplate("cm2", "namespace2", t),
				},
				"phase2": {
					deploymentTemplate("deployment1", t),
					secretTemplate("secret1", t),
				},
			},
				nil,
			),
			expectedObjectIDs: []string{
				"/ConfigMap/namespace1/cm1",
				"/ConfigMap/namespace2/cm2",
				"apps/Deployment/default/deployment1",
				"/Secret/default/secret1",
			},
		},
		{
			objectSet: genObjectSet("test-2", "default-1", map[string][]client.Object{
				"phase1": {
					cmTemplate("cm1", "", t),
					cmTemplate("cm2", "", t),
				},
				"phase2": {
					deploymentTemplate("deployment1", t),
					secretTemplate("secret1", t),
				},
			},
				nil,
			),
			expectedObjectIDs: []string{
				"/ConfigMap/default-1/cm1",
				"/ConfigMap/default-1/cm2",
				"apps/Deployment/default-1/deployment1",
				"/Secret/default-1/secret1",
			},
		},
	}
	for _, testCase := range testCases {
		objects, err := testCase.objectSet.getObjects()
		require.NoError(t, err)
		resObjectIDs := make([]string, len(objects))
		for i := range objects {
			receivedObj := objects[i]
			resObjectIDs[i] = receivedObj.UniqueIdentifier()
		}
		require.ElementsMatch(t, testCase.expectedObjectIDs, resObjectIDs)
	}
}

func TestGetActivelyReconciledObjects(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		objectSet               objectSetGetter
		expectedControllerOfRef []string
	}{
		{
			objectSet: genObjectSet("test-1", "default", nil, []corev1alpha1.ControlledObjectReference{
				{
					Name:      "a",
					Namespace: "b",
					Kind:      "c",
					Group:     "d",
				},
				{
					Name:      "d",
					Namespace: "",
					Kind:      "f",
					Group:     "g",
				},
			}),
			expectedControllerOfRef: []string{"d/c/b/a", "g/f//d"},
		},
	}
	for _, testCase := range testCases {
		currentControllerOfRefs := testCase.objectSet.getActivelyReconciledObjects()
		res := make([]string, 0)
		for _, ref := range currentControllerOfRefs {
			res = append(res, ref.UniqueIdentifier())
		}
		require.ElementsMatch(t, res, testCase.expectedControllerOfRef)
	}
}

func cmTemplate(name string, namespace string, t require.TestingT) client.Object {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    map[string]string{"test.package-operator.run/test-1": "True"},
		},
	}
	GVK, err := apiutil.GVKForObject(cm, testScheme)
	require.NoError(t, err)
	cm.SetGroupVersionKind(GVK)
	return cm
}

func secretTemplate(name string, t require.TestingT) client.Object {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: map[string]string{"test.package-operator.run/test-1": "True"},
		},
		Type: corev1.SecretTypeOpaque,
		StringData: map[string]string{
			"hello": "world",
		},
	}
	GVK, err := apiutil.GVKForObject(secret, testScheme)
	require.NoError(t, err)
	secret.SetGroupVersionKind(GVK)
	return secret
}

func deploymentTemplate(deploymentName string, t require.TestingT) client.Object {
	obj := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:   deploymentName,
			Labels: map[string]string{"test.package-operator.run/test-1": "True"},
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"test.package-operator.run/test-1": "True"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "nginx",
					Labels: map[string]string{"test.package-operator.run/test-1": "True"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "nginx",
							Image: "nginx",
						},
					},
				},
			},
		},
	}

	GVK, err := apiutil.GVKForObject(obj, testScheme)
	require.NoError(t, err)
	obj.SetGroupVersionKind(GVK)
	return obj
}
