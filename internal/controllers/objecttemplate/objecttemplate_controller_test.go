package objecttemplate

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	ctrl "sigs.k8s.io/controller-runtime"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/package-operator/internal/controllers"
	"package-operator.run/package-operator/internal/testutil"
	"package-operator.run/package-operator/internal/testutil/dynamiccachemocks"
)

func TestGenericObjectTemplateController_Reconcile(t *testing.T) {
	tests := []struct {
		name              string
		deletionTimestamp *metav1.Time
	}{
		{
			name: "Runs through",
		},
		{
			name:              "already deleted",
			deletionTimestamp: &metav1.Time{Time: time.Now()},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			controller, c, dc := newControllerAndMocks()

			c.On("Update", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Return(nil).Maybe()
			c.On("Patch", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Return(nil).Maybe()
			dc.On("Free", mock.Anything, mock.Anything).Return(nil).Maybe()

			template, err := os.ReadFile("test_files/package_template_to_json.yaml")
			require.NoError(t, err)
			ObjectTemplate := GenericObjectTemplate{
				ObjectTemplate: corev1alpha1.ObjectTemplate{
					ObjectMeta: metav1.ObjectMeta{
						Finalizers: []string{
							controllers.CachedFinalizer,
						},
					},
					Spec: corev1alpha1.ObjectTemplateSpec{
						Template: string(template),
					},
				},
			}
			ObjectTemplate.ClientObject().SetDeletionTimestamp(test.deletionTimestamp)

			// getting ObjectTemplate
			c.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Run(func(args mock.Arguments) {
					arg := args.Get(2).(*corev1alpha1.ObjectTemplate)
					ObjectTemplate.DeepCopyInto(arg)
				}).
				Return(nil).Once()

			// getting unstructured package
			c.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Return(nil).Once().Maybe()

			res, err := controller.Reconcile(context.Background(), ctrl.Request{})
			assert.Empty(t, res)
			assert.NoError(t, err)

			if test.deletionTimestamp != nil {
				dc.AssertCalled(t, "Free", mock.Anything, mock.Anything)
				return
			}

			dc.AssertNotCalled(t, "Free", mock.Anything, mock.Anything)
		})
	}
}

func TestGenericObjectTemplateController_GetValuesFromSources(t *testing.T) {
	cmKey := "database"
	cmDestination := "database"
	cmValue := "big-database"
	cmSource := corev1alpha1.ObjectTemplateSource{
		ApiVersion: "v1",
		Kind:       "ConfigMap",
		Items: []corev1alpha1.ObjectTemplateSourceItem{
			{
				Key:         cmKey,
				Destination: cmDestination,
			},
		},
	}
	secretKey := "password"
	secretDestination := "password"
	secretValue := "super-secret-password"
	secretSource := corev1alpha1.ObjectTemplateSource{
		ApiVersion: "v1",
		Kind:       "Secret",
		Items: []corev1alpha1.ObjectTemplateSourceItem{
			{
				Key:         secretKey,
				Destination: secretDestination,
			},
		},
	}

	rawObjectTemplate := corev1alpha1.ObjectTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "right-namespace",
		},
		Spec: corev1alpha1.ObjectTemplateSpec{
			Sources: []corev1alpha1.ObjectTemplateSource{
				cmSource,
				secretSource,
			},
		},
	}

	duplicateDestinationRawObjectTemplate := corev1alpha1.ObjectTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "right-namespace",
		},
		Spec: corev1alpha1.ObjectTemplateSpec{
			Sources: []corev1alpha1.ObjectTemplateSource{
				cmSource,
				cmSource,
			},
		},
	}

	rawClusterObjectTemplate := corev1alpha1.ClusterObjectTemplate{
		Spec: corev1alpha1.ObjectTemplateSpec{
			Sources: []corev1alpha1.ObjectTemplateSource{
				cmSource,
				secretSource,
			},
		},
	}

	tests := []struct {
		name                  string
		objectTemplate        corev1alpha1.ObjectTemplate
		clusterObjectTemplate corev1alpha1.ClusterObjectTemplate
		sourceNamespace       string
		duplicateDestination  bool
	}{
		{
			name:           "ObjectTemplate no namespace",
			objectTemplate: rawObjectTemplate,
		},
		{
			name:                 "ObjectTemplate duplicate destination",
			objectTemplate:       duplicateDestinationRawObjectTemplate,
			duplicateDestination: true,
		},
		{
			name:            "ObjectTemplate matching namespace",
			objectTemplate:  rawObjectTemplate,
			sourceNamespace: "right-namespace",
		},
		{
			name:            "ObjectTemplate not matching namespace",
			objectTemplate:  rawObjectTemplate,
			sourceNamespace: "wrong-namespace",
		},
		{
			name:                  "ClusterObjectTemplate no namespace",
			clusterObjectTemplate: rawClusterObjectTemplate,
		},
		{
			name:                  "ClusterObjectTemplate namespace",
			clusterObjectTemplate: rawClusterObjectTemplate,
			sourceNamespace:       "random-namespace",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var genericObjectTemplate genericObjectTemplate
			if len(test.objectTemplate.Spec.Sources) > 0 {
				for i := 0; i < len(test.objectTemplate.Spec.Sources); i++ {
					test.objectTemplate.Spec.Sources[i].Namespace = test.sourceNamespace
				}
				genericObjectTemplate = &GenericObjectTemplate{test.objectTemplate}
			} else if len(test.clusterObjectTemplate.Spec.Sources) > 0 {
				for i := 0; i < len(test.clusterObjectTemplate.Spec.Sources); i++ {
					test.clusterObjectTemplate.Spec.Sources[i].Namespace = test.sourceNamespace
				}
				genericObjectTemplate = &GenericClusterObjectTemplate{test.clusterObjectTemplate}
			}

			controller, _, dc := newControllerAndMocks()
			dc.On("Watch", mock.Anything, mock.Anything, mock.Anything).Return(nil)

			// getting the configMap
			dc.On("Get",
				mock.Anything,
				mock.Anything,
				mock.Anything,
				mock.Anything,
			).Run(func(args mock.Arguments) {
				obj := args.Get(2).(*unstructured.Unstructured)
				err := unstructured.SetNestedField(obj.Object, cmValue, cmKey)
				require.NoError(t, err)
			}).Return(nil).Once().Maybe()

			// Getting the secret
			dc.On("Get",
				mock.Anything,
				mock.Anything,
				mock.Anything,
				mock.Anything,
			).Run(func(args mock.Arguments) {
				obj := args.Get(2).(*unstructured.Unstructured)
				err := unstructured.SetNestedField(obj.Object, secretValue, secretKey)
				require.NoError(t, err)
			}).Return(nil).Once().Maybe()

			sources := &unstructured.Unstructured{
				Object: map[string]interface{}{},
			}
			err := controller.GetValuesFromSources(context.TODO(), genericObjectTemplate, sources)
			if test.duplicateDestination {
				assert.Error(t, err)
				return
			}
			if test.sourceNamespace == "wrong-namespace" {
				assert.Error(t, err)
				return
			}
			if len(test.clusterObjectTemplate.Spec.Sources) > 0 && test.sourceNamespace == "" {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			dc.AssertNumberOfCalls(t, "Watch", 2)
			assert.Equal(t, sources.Object[cmDestination], cmValue)
			assert.Equal(t, sources.Object[secretDestination], secretValue)
		})
	}
}

func TestGenericObjectTemplateController_TemplatePackage(t *testing.T) {
	tests := []struct {
		name        string
		packageFile string
	}{
		{
			name:        "template by key",
			packageFile: "package_template_by_key.yaml",
		},
		{
			name:        "template with toJson",
			packageFile: "package_template_to_json.yaml",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			controller, _, _ := newControllerAndMocks()
			pkg := &unstructured.Unstructured{}
			sources := &unstructured.Unstructured{
				Object: map[string]interface{}{
					"Database":      "asdf",
					"username1":     "user",
					"auth_password": "hunter2",
				},
			}
			template, err := os.ReadFile(filepath.Join("test_files", test.packageFile))
			require.NoError(t, err)
			err = controller.TemplatePackage(context.TODO(), string(template), sources, pkg)
			require.NoError(t, err)

			for key, value := range sources.Object {
				renderedValue, found, err := unstructured.NestedFieldCopy(pkg.Object, "spec", "config", key)
				require.True(t, found)
				require.NoError(t, err)
				assert.Equal(t, renderedValue, value)
			}
		})
	}
}

func newControllerAndMocks() (*GenericObjectTemplateController, *testutil.CtrlClient, *dynamiccachemocks.DynamicCacheMock) {
	scheme := testutil.NewTestSchemeWithCoreV1Alpha1()
	c := testutil.NewClient()
	dc := &dynamiccachemocks.DynamicCacheMock{}

	controller := &GenericObjectTemplateController{
		newObjectTemplate: newGenericObjectTemplate,
		client:            c,
		log:               ctrl.Log.WithName("controllers"),
		scheme:            scheme,
		dynamicCache:      dc,
	}
	return controller, c, dc
}
