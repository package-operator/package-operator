package integration

import (
	"context"
	"fmt"
	"testing"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

func TestObjectTemplate_creationDeletion(t *testing.T) {
	cmKey := "database"
	cmDestination := "database"
	cmValue := "big-database"
	cmName := "config-map"
	cm := v1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      cmName,
			Namespace: "default",
		},
		Data: map[string]string{
			cmKey: cmValue,
		},
	}
	cmSource := corev1alpha1.ObjectTemplateSource{
		APIVersion: "v1",
		Kind:       "ConfigMap",
		Name:       cmName,
		Items: []corev1alpha1.ObjectTemplateSourceItem{
			{
				Key:         "data." + cmKey,
				Destination: cmDestination,
			},
		},
	}
	cmGVK, err := apiutil.GVKForObject(&cm, Scheme)
	require.NoError(t, err)
	cm.SetGroupVersionKind(cmGVK)
	secretName := "secret"
	secretKey := "password"
	secretDestination := "password"
	secretValue := "super-secret-password"
	secret := v1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: "default",
		},
		Type: v1.SecretTypeOpaque,
		StringData: map[string]string{
			secretKey: secretValue,
		},
	}
	secretGVK, err := apiutil.GVKForObject(&secret, Scheme)
	require.NoError(t, err)
	secret.SetGroupVersionKind(secretGVK)
	secretSource := corev1alpha1.ObjectTemplateSource{
		APIVersion: "v1",
		Kind:       "Secret",
		Name:       secretName,
		Items: []corev1alpha1.ObjectTemplateSourceItem{
			{
				Key:         "data." + secretKey,
				Destination: secretDestination,
			},
		},
	}

	template := fmt.Sprintf(`apiVersion: package-operator.run/v1alpha1
kind: Package
metadata:
  name: test-stub
  namespace: default
spec:
  image: %s
  config:
    {{ toJson .config }}`, SuccessTestPackageImage)

	objectTemplateName := "object-template"
	objectTemplate := corev1alpha1.ObjectTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      objectTemplateName,
			Namespace: "default",
		},
		Spec: corev1alpha1.ObjectTemplateSpec{
			Template: template,
			Sources: []corev1alpha1.ObjectTemplateSource{
				cmSource,
				secretSource,
			},
		},
	}
	objectTemplateGVK, err := apiutil.GVKForObject(&objectTemplate, Scheme)
	require.NoError(t, err)
	objectTemplate.SetGroupVersionKind(objectTemplateGVK)
	tests := []struct {
		name string
	}{
		{
			name: "toJSON",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := logr.NewContext(context.Background(), testr.New(t))
			err := Client.Create(ctx, &cm)
			require.NoError(t, err)
			err = Client.Create(ctx, &secret)
			require.NoError(t, err)
			err = Client.Create(ctx, &objectTemplate)
			require.NoError(t, err)

			pkg := &corev1alpha1.Package{}
			require.NoError(t, Client.Get(ctx, client.ObjectKey{
				Name: "test-stub", Namespace: "default",
			}, pkg))
		})
	}
}
