package objecttemplate

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"

	"package-operator.run/package-operator/internal/preflight"
	"package-operator.run/package-operator/internal/testutil/restmappermock"

	"sigs.k8s.io/yaml"

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
			name: "exists",
		},
		{
			name:              "already deleted",
			deletionTimestamp: &metav1.Time{Time: time.Now()},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			controller, c, dc, rm := newControllerAndMocks(t)

			c.On("Update", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Return(nil).Maybe()
			c.On("Patch", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Return(nil).Maybe()
			dc.On("Free", mock.Anything, mock.Anything).Return(nil).Maybe()
			rm.On("RESTMapping").Return(&meta.RESTMapping{Scope: meta.RESTScopeNamespace}, nil).Twice()

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

			// getting package
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
		APIVersion: "v1",
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
		APIVersion: "v1",
		Kind:       "Secret",
		Items: []corev1alpha1.ObjectTemplateSourceItem{
			{
				Key:         secretKey,
				Destination: secretDestination,
			},
		},
	}

	tests := []struct {
		name                    string
		source1                 corev1alpha1.ObjectTemplateSource
		source2                 corev1alpha1.ObjectTemplateSource
		sourceNamespace         string
		objectTemplateNamespace string
		isObjectTemplate        bool
		isClusterObjectTemplate bool
	}{
		{
			name:             "ObjectTemplate no namespace",
			isObjectTemplate: true,
		},
		{
			name:    "ObjectTemplate duplicate destination source",
			source1: cmSource,
			source2: cmSource,
		},
		{
			name:                    "ObjectTemplate matching namespace",
			isObjectTemplate:        true,
			source1:                 cmSource,
			source2:                 secretSource,
			objectTemplateNamespace: "right-namespace",
			sourceNamespace:         "right-namespace",
		},
		{
			name:                    "ObjectTemplate not matching namespace",
			isObjectTemplate:        true,
			source1:                 cmSource,
			source2:                 secretSource,
			objectTemplateNamespace: "right-namespace",
			sourceNamespace:         "wrong-namespace",
		},
		{
			name:                    "cluster scoped owner, sources no namespace",
			isClusterObjectTemplate: true,
			source1:                 cmSource,
			source2:                 secretSource,
		},
		{
			name:                    "ClusterObjectTemplate namespace",
			isClusterObjectTemplate: true,
			sourceNamespace:         "random-namespace",
			source1:                 cmSource,
			source2:                 secretSource,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// create genericObjectTemplate's and set source namespaces
			genericObjectTemplate := createGenericObjectTemplate(t, test.isObjectTemplate, test.source1, test.source2, test.objectTemplateNamespace, test.sourceNamespace)

			controller, _, dc, rm := newControllerAndMocks(t)
			rm.On("RESTMapping").Return(&meta.RESTMapping{Scope: meta.RESTScopeNamespace}, nil)

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
			if reflect.DeepEqual(test.source1, test.source2) {
				assert.Error(t, err)
				return
			}
			if test.sourceNamespace == "wrong-namespace" {
				assert.Error(t, err)
				return
			}
			if test.isClusterObjectTemplate && len(test.sourceNamespace) == 0 {
				require.Error(t, err)
				assert.ErrorContains(t, err, "Object doesn't have a namepsace and no default is provided.")
				return
			}

			require.NoError(t, err)
			dc.AssertNumberOfCalls(t, "Watch", 2)
			assert.Equal(t, sources.Object[cmDestination], cmValue)
			assert.Equal(t, sources.Object[secretDestination], secretValue)
		})
	}
}

func createGenericObjectTemplate(t *testing.T, isObjectTemplate bool, source1, source2 corev1alpha1.ObjectTemplateSource, objectTemplateNamespace, sourceNamespace string) genericObjectTemplate {
	t.Helper()
	source1.Namespace = sourceNamespace
	source2.Namespace = sourceNamespace
	if isObjectTemplate {
		objectTemplate := corev1alpha1.ObjectTemplate{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: objectTemplateNamespace,
			},
			Spec: corev1alpha1.ObjectTemplateSpec{
				Sources: []corev1alpha1.ObjectTemplateSource{
					source1,
					source2,
				},
			},
		}
		return &GenericObjectTemplate{objectTemplate}
	}

	clusterObjectTemplate := corev1alpha1.ClusterObjectTemplate{
		Spec: corev1alpha1.ObjectTemplateSpec{
			Sources: []corev1alpha1.ObjectTemplateSource{
				source1,
				source2,
			},
		},
	}
	return &GenericClusterObjectTemplate{clusterObjectTemplate}
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
			name:        "template with toJSON",
			packageFile: "package_template_to_json.yaml",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			controller, _, _, rm := newControllerAndMocks(t)
			rm.On("RESTMapping").Return(&meta.RESTMapping{Scope: meta.RESTScopeNamespace}, nil).Twice()
			template, err := os.ReadFile(filepath.Join("test_files", test.packageFile))
			require.NoError(t, err)

			objectTemplate := GenericObjectTemplate{
				ObjectTemplate: corev1alpha1.ObjectTemplate{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "default",
					},
					Spec: corev1alpha1.ObjectTemplateSpec{
						Template: string(template),
					},
				},
			}

			pkg := &corev1alpha1.Package{}
			sources := &unstructured.Unstructured{
				Object: map[string]interface{}{
					"Database":      "asdf",
					"username1":     "user",
					"auth_password": "hunter2",
				},
			}

			err = controller.TemplateObject(context.TODO(), &objectTemplate, sources, pkg)

			require.NoError(t, err)

			for key, value := range sources.Object {
				config := map[string]interface{}{}
				require.NoError(t, yaml.Unmarshal(pkg.Spec.Config.Raw, &config))
				assert.Equal(t, value, config[key])
			}
		})
	}
}

func TestGenericObjectTemplateController_TemplatePackage_Mismatch(t *testing.T) {
	controller, _, _, rm := newControllerAndMocks(t)
	// clusterpackage is meta.RESTScopeRoot scoped
	rm.On("RESTMapping").Return(&meta.RESTMapping{Scope: meta.RESTScopeRoot}, nil).Twice()

	template, err := os.ReadFile(filepath.Join("test_files", "clusterpackage_template_to_json.yaml"))
	require.NoError(t, err)
	objectTemplate := &GenericObjectTemplate{
		ObjectTemplate: corev1alpha1.ObjectTemplate{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "default",
			},
			Spec: corev1alpha1.ObjectTemplateSpec{
				Template: string(template),
			},
		},
	}
	err = controller.TemplateObject(context.TODO(), objectTemplate, &unstructured.Unstructured{}, &unstructured.Unstructured{})
	assert.ErrorContains(t, err, "Must be namespaced scoped when part of an non-cluster-scoped API")
}

func newControllerAndMocks(t *testing.T) (*GenericObjectTemplateController, *testutil.CtrlClient, *dynamiccachemocks.DynamicCacheMock, *restmappermock.RestMapperMock) {
	t.Helper()
	scheme := testutil.NewTestSchemeWithCoreV1Alpha1()
	c := testutil.NewClient()
	dc := &dynamiccachemocks.DynamicCacheMock{}
	rm := &restmappermock.RestMapperMock{}

	controller := &GenericObjectTemplateController{
		newObjectTemplate: newGenericObjectTemplate,
		client:            c,
		log:               ctrl.Log.WithName("controllers"),
		scheme:            scheme,
		dynamicCache:      dc,
		preflightChecker: preflight.List{
			preflight.NewAPIExistence(rm),
			preflight.NewEmptyNamespaceNoDefault(rm),
			preflight.NewNamespaceEscalation(rm),
		},
	}
	return controller, c, dc, rm
}
