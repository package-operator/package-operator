package objecttemplate

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	apimachineryerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/internal/apis/manifests"
	"package-operator.run/internal/controllers"
	"package-operator.run/internal/environment"
	"package-operator.run/internal/preflight"
	"package-operator.run/internal/testutil"
	"package-operator.run/internal/testutil/dynamiccachemocks"
)

func Test_templateReconciler_getSourceObject(t *testing.T) {
	t.Parallel()
	client := testutil.NewClient()
	uncachedClient := testutil.NewClient()
	dynamicCache := &dynamiccachemocks.DynamicCacheMock{}

	dynamicCache.
		On("Watch", mock.Anything, mock.Anything, mock.Anything).
		Return(nil)

	dynamicCache.
		On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(apimachineryerrors.NewNotFound(schema.GroupResource{}, ""))

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
	t.Parallel()
	r := &templateReconciler{
		preflightChecker: preflight.CheckerFn(
			func(context.Context, client.Object, client.Object) ([]preflight.Violation, error) {
				return []preflight.Violation{{Position: "here", Error: "aaaaaaah!"}}, nil
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
	t.Parallel()
	tests := []struct {
		name     string
		object   map[string]any
		source   string
		dest     string
		expected map[string]any
	}{
		{
			name: "string stays string",
			object: map[string]any{
				"data": map[string]any{
					"something": "123",
				},
			},
			source: ".data.something",
			dest:   ".banana",
			expected: map[string]any{
				"banana": "123",
			},
		},
		{
			name: "number stays number",
			object: map[string]any{
				"data": map[string]any{
					"something": 123,
				},
			},
			source: ".data.something",
			dest:   ".banana",
			expected: map[string]any{
				"banana": float64(123), // json numbers are floats
			},
		},
		{
			name: "supports dots",
			object: map[string]any{
				"data": map[string]any{
					"some.thing": "123",
				},
			},
			source: `.data['some\.thing']`,
			dest:   ".banana",
			expected: map[string]any{
				"banana": "123",
			},
		},
		{
			name: "multiple results",
			object: map[string]any{
				"data": []any{
					map[string]any{
						"name": "123",
					},
					map[string]any{
						"name": "456",
					},
				},
			},
			source: ".data..name",
			dest:   ".banana",
			expected: map[string]any{
				"banana": []any{"123", "456"},
			},
		},
	}

	for i := range tests {
		test := tests[i]
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			sourceObj := &unstructured.Unstructured{
				Object: test.object,
			}
			sourcesConfig := map[string]any{}
			items := []corev1alpha1.ObjectTemplateSourceItem{
				{Key: test.source, Destination: test.dest},
			}
			err := copySourceItems(
				items, sourceObj, sourcesConfig)
			require.NoError(t, err)
			assert.Equal(t, test.expected, sourcesConfig)
		})
	}
}

func Test_copySourceItems_notfound(t *testing.T) {
	t.Parallel()
	sourceObj := &unstructured.Unstructured{
		Object: map[string]any{},
	}
	sourcesConfig := map[string]any{}
	items := []corev1alpha1.ObjectTemplateSourceItem{
		{Key: ".data.something", Destination: ".banana"},
	}
	err := copySourceItems(
		items, sourceObj, sourcesConfig)
	require.EqualError(t, err, "data is not found")
}

func Test_copySourceItems_nonJSONPath_destination(t *testing.T) {
	t.Parallel()
	sourceObj := &unstructured.Unstructured{
		Object: map[string]any{
			"data": map[string]any{
				"something": "123",
			},
		},
	}
	sourcesConfig := map[string]any{}
	items := []corev1alpha1.ObjectTemplateSourceItem{
		{Key: ".data.something", Destination: "banana"},
	}
	err := copySourceItems(
		items, sourceObj, sourcesConfig)
	require.EqualError(t, err, "path banana must be a JSONPath with a leading dot")
}

func Test_templateReconciler_templateObject(t *testing.T) {
	t.Parallel()
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
	for i := range tests {
		test := tests[i]
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			r := &templateReconciler{
				Sink:             environment.NewSink(nil),
				preflightChecker: preflight.List{},
			}
			r.Sink.SetEnvironment(&manifests.PackageEnvironment{})

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
			sourcesConfig := map[string]any{
				"Database":      "asdf",
				"username1":     "user",
				"auth_password": "hunter2",
			}

			ctx := context.Background()
			err = r.templateObject(ctx, sourcesConfig, &objectTemplate, pkg)

			require.NoError(t, err)

			for key, value := range sourcesConfig {
				config := map[string]any{}
				require.NoError(t, yaml.Unmarshal(pkg.Spec.Config.Raw, &config))
				assert.Equal(t, value, config[key])
			}
		})
	}
}

func Test_updateStatusConditionsFromOwnedObject(t *testing.T) {
	t.Parallel()

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
				Object: map[string]any{
					"metadata": map[string]any{
						"generation": int64(4),
					},
					"status": map[string]any{
						"conditions": []any{
							map[string]any{
								"type":               "Available",
								"status":             "True",
								"observedGeneration": int64(4),
								"reason":             "",
								"message":            "",
							},
							// outdated
							map[string]any{
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
				Object: map[string]any{
					"metadata": map[string]any{
						"generation": int64(4),
					},
					"status": map[string]any{
						"observedGeneration": int64(2),
						"conditions": []any{
							map[string]any{
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

	for i := range tests {
		test := tests[i]
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

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
	t.Parallel()
	tests := []struct {
		name              string
		deletionTimestamp *metav1.Time
	}{
		{
			name: "exists",
		},
	}
	for i := range tests {
		test := tests[i]

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

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
			require.NoError(t, err)
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
		Sink: environment.NewSink(c),

		client:           c,
		uncachedClient:   uncachedC,
		scheme:           scheme,
		dynamicCache:     dc,
		preflightChecker: preflight.List{},
	}
	return r, c, uncachedC, dc
}

var errTest = errors.New("something")

func Test_setObjectTemplateConditionBasedOnError(t *testing.T) {
	t.Parallel()

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
	for i := range tests {
		test := tests[i]
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			outErr := setObjectTemplateConditionBasedOnError(test.objectTemplate, test.err)
			if test.expectedErr == nil {
				require.NoError(t, outErr)
			} else {
				require.ErrorIs(t, outErr, test.expectedErr)
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
