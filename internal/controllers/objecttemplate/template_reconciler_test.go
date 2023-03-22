package objecttemplate

import (
	"context"
	goerrors "errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/package-operator/internal/controllers"
	"package-operator.run/package-operator/internal/preflight"
	"package-operator.run/package-operator/internal/testutil"
	"package-operator.run/package-operator/internal/testutil/dynamiccachemocks"
)

func Test_templateReconciler_getSourceObject(t *testing.T) {
	client := testutil.NewClient()
	uncachedClient := testutil.NewClient()
	dynamicCache := &dynamiccachemocks.DynamicCacheMock{}

	dynamicCache.
		On("Watch", mock.Anything, mock.Anything, mock.Anything).
		Return(nil)

	dynamicCache.
		On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(errors.NewNotFound(schema.GroupResource{}, ""))

	uncachedClient.
		On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil)

	client.
		On("Patch", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil)

	r := &templateReconciler{
		client:           client,
		uncachedClient:   uncachedClient,
		dynamicCache:     dynamicCache,
		preflightChecker: preflight.List{},
	}

	objectTemplate := &corev1alpha1.ObjectTemplate{}

	ctx := context.Background()
	srcObj, _, err := r.getSourceObject(
		ctx, objectTemplate, corev1alpha1.ObjectTemplateSource{})
	require.NoError(t, err)

	if assert.NotNil(t, srcObj) {
		assert.Equal(t, map[string]string{
			controllers.DynamicCacheLabel: "True",
		}, srcObj.GetLabels())
	}
	client.AssertCalled(
		t, "Patch", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func Test_templateReconciler_getSourceObject_stopAtViolation(t *testing.T) {
	r := &templateReconciler{
		preflightChecker: preflight.CheckerFn(func(
			ctx context.Context, owner, obj client.Object,
		) (violations []preflight.Violation, err error) {
			return []preflight.Violation{{
				Position: "here", Error: "aaaaaaah!",
			}}, nil
		}),
	}

	objectTemplate := &corev1alpha1.ObjectTemplate{}

	ctx := context.Background()
	_, _, err := r.getSourceObject(
		ctx, objectTemplate, corev1alpha1.ObjectTemplateSource{
			Kind:      "ConfigMap",
			Name:      "test",
			Namespace: "default",
		})
	require.EqualError(t, err, "for source ConfigMap default/test: here: aaaaaaah!")
}

func Test_copySourceItems(t *testing.T) {
	ctx := context.Background()
	sourceObj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"data": map[string]interface{}{
				"something": "123",
			},
		},
	}
	sourcesConfig := map[string]interface{}{}
	items := []corev1alpha1.ObjectTemplateSourceItem{
		{Key: ".data.something", Destination: ".banana"},
	}
	err := copySourceItems(
		ctx, items, sourceObj, sourcesConfig)
	require.NoError(t, err)
	assert.Equal(t, map[string]interface{}{
		"banana": "123",
	}, sourcesConfig)
}

func Test_copySourceItems_notfound(t *testing.T) {
	ctx := context.Background()
	sourceObj := &unstructured.Unstructured{
		Object: map[string]interface{}{},
	}
	sourcesConfig := map[string]interface{}{}
	items := []corev1alpha1.ObjectTemplateSourceItem{
		{Key: ".data.something", Destination: ".banana"},
	}
	err := copySourceItems(
		ctx, items, sourceObj, sourcesConfig)
	require.EqualError(t, err, "key .data.something not found")
}

func Test_copySourceItems_nonJSONPath_key(t *testing.T) {
	ctx := context.Background()
	sourceObj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"data": map[string]interface{}{
				"something": "123",
			},
		},
	}
	sourcesConfig := map[string]interface{}{}
	items := []corev1alpha1.ObjectTemplateSourceItem{
		{Key: "data.something", Destination: ".banana"},
	}
	err := copySourceItems(
		ctx, items, sourceObj, sourcesConfig)
	require.EqualError(t, err, "path data.something must be a JSONPath with a leading dot")
}

func Test_copySourceItems_nonJSONPath_destination(t *testing.T) {
	ctx := context.Background()
	sourceObj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"data": map[string]interface{}{
				"something": "123",
			},
		},
	}
	sourcesConfig := map[string]interface{}{}
	items := []corev1alpha1.ObjectTemplateSourceItem{
		{Key: ".data.something", Destination: "banana"},
	}
	err := copySourceItems(
		ctx, items, sourceObj, sourcesConfig)
	require.EqualError(t, err, "path banana must be a JSONPath with a leading dot")
}

func Test_templateReconciler_templateObject(t *testing.T) {
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
			r := &templateReconciler{
				preflightChecker: preflight.List{},
			}

			template, err := os.ReadFile(filepath.Join("testdata", test.packageFile))
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
			sourcesConfig := map[string]interface{}{
				"Database":      "asdf",
				"username1":     "user",
				"auth_password": "hunter2",
			}

			ctx := context.Background()
			err = r.templateObject(ctx, sourcesConfig, &objectTemplate, pkg)

			require.NoError(t, err)

			for key, value := range sourcesConfig {
				config := map[string]interface{}{}
				require.NoError(t, yaml.Unmarshal(pkg.Spec.Config.Raw, &config))
				assert.Equal(t, value, config[key])
			}
		})
	}
}

func Test_updateStatusConditionsFromOwnedObject(t *testing.T) {
	tests := []struct {
		name               string
		obj                *unstructured.Unstructured
		expectedConditions []metav1.Condition
	}{
		{
			name:               "no conditions reported",
			obj:                &unstructured.Unstructured{},
			expectedConditions: nil,
		},
		{
			name: "conditions reported",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"generation": int64(4),
					},
					"status": map[string]interface{}{
						"conditions": []interface{}{
							map[string]interface{}{
								"type":               "Available",
								"status":             "True",
								"observedGeneration": int64(4),
								"reason":             "",
								"message":            "",
							},
							// outdated
							map[string]interface{}{
								"type":               "Test",
								"status":             "True",
								"observedGeneration": int64(2),
								"reason":             "",
								"message":            "",
							},
						},
					},
				},
			},
			expectedConditions: []metav1.Condition{
				{
					Type: "Available", Status: metav1.ConditionTrue,
					ObservedGeneration: 4,
				},
			},
		},
		{
			name: "status outdated",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"generation": int64(4),
					},
					"status": map[string]interface{}{
						"observedGeneration": int64(2),
						"conditions": []interface{}{
							map[string]interface{}{
								"type":               "Available",
								"status":             "True",
								"observedGeneration": int64(4),
								"reason":             "",
								"message":            "",
							},
						},
					},
				},
			},
			expectedConditions: []metav1.Condition{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			objectTemplate := &GenericObjectTemplate{
				ObjectTemplate: corev1alpha1.ObjectTemplate{
					ObjectMeta: metav1.ObjectMeta{
						Generation: 4,
					},
				},
			}

			err := updateStatusConditionsFromOwnedObject(
				ctx, objectTemplate, test.obj)
			require.NoError(t, err)
			conds := *objectTemplate.GetConditions()
			if assert.Len(t, conds, len(test.expectedConditions)) {
				for i, expectedCond := range test.expectedConditions {
					cond := conds[i]
					assert.Equal(t, expectedCond.ObservedGeneration, cond.ObservedGeneration)
					assert.Equal(t, expectedCond.Type, cond.Type)
					assert.Equal(t, expectedCond.Status, cond.Status)
					assert.Equal(t, expectedCond.Message, cond.Message)
					assert.Equal(t, expectedCond.Reason, cond.Reason)
					assert.NotEmpty(t, cond.LastTransitionTime)
				}
			}
		})
	}
}

func Test_templateReconcilerReconcile(t *testing.T) {
	tests := []struct {
		name              string
		deletionTimestamp *metav1.Time
	}{
		{
			name: "exists",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			r, client, _, dc := newControllerAndMocks(t)

			client.On("Update", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Return(nil).Maybe()
			client.On("Patch", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Return(nil).Maybe()
			dc.
				On("Watch", mock.Anything, mock.Anything, mock.Anything).
				Return(nil)

			template, err := os.ReadFile("testdata/package_template_to_json.yaml")
			require.NoError(t, err)
			objectTemplate := &GenericObjectTemplate{
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
			objectTemplate.ClientObject().SetDeletionTimestamp(test.deletionTimestamp)

			// getting package
			dc.
				On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Return(nil).Once().Maybe()

			res, err := r.Reconcile(context.Background(), objectTemplate)
			assert.Empty(t, res)
			assert.NoError(t, err)
		})
	}
}

func newControllerAndMocks(t *testing.T) (
	*templateReconciler, *testutil.CtrlClient, *testutil.CtrlClient,
	*dynamiccachemocks.DynamicCacheMock,
) {
	t.Helper()
	scheme := testutil.NewTestSchemeWithCoreV1Alpha1()
	c := testutil.NewClient()
	uncachedC := testutil.NewClient()
	dc := &dynamiccachemocks.DynamicCacheMock{}

	r := &templateReconciler{
		client:           c,
		uncachedClient:   uncachedC,
		scheme:           scheme,
		dynamicCache:     dc,
		preflightChecker: preflight.List{},
	}
	return r, c, uncachedC, dc
}

var errTest = goerrors.New("something")

func Test_setObjectTemplateConditionBasedOnError(t *testing.T) {
	tests := []struct {
		name               string
		objectTemplate     *GenericObjectTemplate
		err                error
		expectedConditions []metav1.Condition
		expectedErr        error
	}{
		{
			name: "sets invalid condition for SourceError",
			objectTemplate: &GenericObjectTemplate{
				ObjectTemplate: corev1alpha1.ObjectTemplate{},
			},
			err: &SourceError{
				Source: &unstructured.Unstructured{},
			},
			expectedConditions: []metav1.Condition{
				{
					Type:    corev1alpha1.ObjectTemplateInvalid,
					Status:  metav1.ConditionTrue,
					Message: "for source  /: %!s(<nil>)",
					Reason:  "SourceError",
				},
			},
			expectedErr: nil,
		},
		{
			name: "sets invalid condition for TemplateError",
			objectTemplate: &GenericObjectTemplate{
				ObjectTemplate: corev1alpha1.ObjectTemplate{},
			},
			err: &TemplateError{Err: errTest},
			expectedConditions: []metav1.Condition{
				{
					Type:    corev1alpha1.ObjectTemplateInvalid,
					Status:  metav1.ConditionTrue,
					Message: "something",
					Reason:  "TemplateError",
				},
			},
			expectedErr: nil,
		},
		{
			name: "removes invalid condition",
			objectTemplate: &GenericObjectTemplate{
				ObjectTemplate: corev1alpha1.ObjectTemplate{
					Status: corev1alpha1.ObjectTemplateStatus{
						Conditions: []metav1.Condition{
							{
								Type:    corev1alpha1.ObjectTemplateInvalid,
								Status:  metav1.ConditionTrue,
								Message: "for source  /: %!s(<nil>)",
								Reason:  "SourceError",
							},
						},
					},
				},
			},
			err:                nil,
			expectedConditions: []metav1.Condition{},
			expectedErr:        nil,
		},
		{
			name: "just returns other errors",
			objectTemplate: &GenericObjectTemplate{
				ObjectTemplate: corev1alpha1.ObjectTemplate{},
			},
			err:                errTest,
			expectedConditions: []metav1.Condition{},
			expectedErr:        errTest,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			outErr := setObjectTemplateConditionBasedOnError(test.objectTemplate, test.err)
			if test.expectedErr == nil {
				assert.NoError(t, outErr)
			} else {
				assert.ErrorIs(t, outErr, test.expectedErr)
			}

			conds := *test.objectTemplate.GetConditions()
			if assert.Len(t, conds, len(test.expectedConditions)) {
				for i, expectedCond := range test.expectedConditions {
					cond := conds[i]
					assert.Equal(t, expectedCond.ObservedGeneration, cond.ObservedGeneration)
					assert.Equal(t, expectedCond.Type, cond.Type)
					assert.Equal(t, expectedCond.Status, cond.Status)
					assert.Equal(t, expectedCond.Message, cond.Message)
					assert.Equal(t, expectedCond.Reason, cond.Reason)
					assert.NotEmpty(t, cond.LastTransitionTime)
				}
			}
		})
	}
}
