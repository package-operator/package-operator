package fix

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"package-operator.run/internal/testutil"
)

func TestControllerOfVersion_Check(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		setupMock      func(*testutil.CtrlClient)
		expectedResult bool
		expectedError  bool
	}{
		{
			name: "returns true when ClusterObjectSet CRD is missing version field",
			setupMock: func(c *testutil.CtrlClient) {
				// ClusterObjectSet CRD without version field
				cosCRD := &apiextensionsv1.CustomResourceDefinition{
					ObjectMeta: metav1.ObjectMeta{
						Name: cosCRDName,
					},
					Spec: apiextensionsv1.CustomResourceDefinitionSpec{
						Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
							{
								Name: "v1alpha1",
								Schema: &apiextensionsv1.CustomResourceValidation{
									OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{
										Type: "object",
										Properties: map[string]apiextensionsv1.JSONSchemaProps{
											"status": {
												Properties: map[string]apiextensionsv1.JSONSchemaProps{
													"controllerOf": {
														Items: &apiextensionsv1.JSONSchemaPropsOrArray{
															Schema: &apiextensionsv1.JSONSchemaProps{
																Properties: map[string]apiextensionsv1.JSONSchemaProps{
																	"group": {
																		Type: "string",
																	},
																},
															},
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				}

				c.On("Get",
					mock.Anything,
					client.ObjectKey{Name: cosCRDName},
					mock.IsType(&apiextensionsv1.CustomResourceDefinition{}),
					mock.Anything).
					Run(func(args mock.Arguments) {
						crd := args.Get(2).(*apiextensionsv1.CustomResourceDefinition)
						*crd = *cosCRD
					}).
					Return(nil)

				// ObjectSet CRD with version field
				osCRD := createCRDWithVersionField(osCRDName)
				c.On("Get",
					mock.Anything,
					client.ObjectKey{Name: osCRDName},
					mock.IsType(&apiextensionsv1.CustomResourceDefinition{}),
					mock.Anything).
					Run(func(args mock.Arguments) {
						crd := args.Get(2).(*apiextensionsv1.CustomResourceDefinition)
						*crd = *osCRD
					}).
					Return(nil)
			},
			expectedResult: true,
			expectedError:  false,
		},
		{
			name: "returns true when ObjectSet CRD is missing version field",
			setupMock: func(c *testutil.CtrlClient) {
				// ClusterObjectSet CRD with version field
				cosCRD := createCRDWithVersionField(cosCRDName)
				c.On("Get",
					mock.Anything,
					client.ObjectKey{Name: cosCRDName},
					mock.IsType(&apiextensionsv1.CustomResourceDefinition{}),
					mock.Anything).
					Run(func(args mock.Arguments) {
						crd := args.Get(2).(*apiextensionsv1.CustomResourceDefinition)
						*crd = *cosCRD
					}).
					Return(nil)

				// ObjectSet CRD without version field
				osCRD := &apiextensionsv1.CustomResourceDefinition{
					ObjectMeta: metav1.ObjectMeta{
						Name: osCRDName,
					},
					Spec: apiextensionsv1.CustomResourceDefinitionSpec{
						Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
							{
								Name: "v1alpha1",
								Schema: &apiextensionsv1.CustomResourceValidation{
									OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{
										Type: "object",
									},
								},
							},
						},
					},
				}

				c.On("Get",
					mock.Anything,
					client.ObjectKey{Name: osCRDName},
					mock.IsType(&apiextensionsv1.CustomResourceDefinition{}),
					mock.Anything).
					Run(func(args mock.Arguments) {
						crd := args.Get(2).(*apiextensionsv1.CustomResourceDefinition)
						*crd = *osCRD
					}).
					Return(nil)
			},
			expectedResult: true,
			expectedError:  false,
		},
		{
			name: "returns false when both CRDs have version field",
			setupMock: func(c *testutil.CtrlClient) {
				// ClusterObjectSet CRD with version field
				cosCRD := createCRDWithVersionField(cosCRDName)
				c.On("Get",
					mock.Anything,
					client.ObjectKey{Name: cosCRDName},
					mock.IsType(&apiextensionsv1.CustomResourceDefinition{}),
					mock.Anything).
					Run(func(args mock.Arguments) {
						crd := args.Get(2).(*apiextensionsv1.CustomResourceDefinition)
						*crd = *cosCRD
					}).
					Return(nil)

				// ObjectSet CRD with version field
				osCRD := createCRDWithVersionField(osCRDName)
				c.On("Get",
					mock.Anything,
					client.ObjectKey{Name: osCRDName},
					mock.IsType(&apiextensionsv1.CustomResourceDefinition{}),
					mock.Anything).
					Run(func(args mock.Arguments) {
						crd := args.Get(2).(*apiextensionsv1.CustomResourceDefinition)
						*crd = *osCRD
					}).
					Return(nil)
			},
			expectedResult: false,
			expectedError:  false,
		},
		{
			name: "returns error when ClusterObjectSet CRD lookup fails",
			setupMock: func(c *testutil.CtrlClient) {
				c.On("Get",
					mock.Anything,
					client.ObjectKey{Name: cosCRDName},
					mock.IsType(&apiextensionsv1.CustomResourceDefinition{}),
					mock.Anything).
					Return(errTest)
			},
			expectedResult: false,
			expectedError:  true,
		},
		{
			name: "returns error when ObjectSet CRD lookup fails",
			setupMock: func(c *testutil.CtrlClient) {
				// ClusterObjectSet CRD succeeds
				cosCRD := createCRDWithVersionField(cosCRDName)
				c.On("Get",
					mock.Anything,
					client.ObjectKey{Name: cosCRDName},
					mock.IsType(&apiextensionsv1.CustomResourceDefinition{}),
					mock.Anything).
					Run(func(args mock.Arguments) {
						crd := args.Get(2).(*apiextensionsv1.CustomResourceDefinition)
						*crd = *cosCRD
					}).
					Return(nil)

				// ObjectSet CRD fails
				c.On("Get",
					mock.Anything,
					client.ObjectKey{Name: osCRDName},
					mock.IsType(&apiextensionsv1.CustomResourceDefinition{}),
					mock.Anything).
					Return(errTest)
			},
			expectedResult: false,
			expectedError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			c := testutil.NewClient()
			tt.setupMock(c)

			fix := &ControllerOfVersion{}
			result, err := fix.Check(ctx, Context{Client: c})

			if tt.expectedError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.expectedResult, result)
			c.AssertExpectations(t)
		})
	}
}

func TestControllerOfVersion_Run(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		setupMock     func(*testutil.CtrlClient)
		expectedError bool
		validateCalls func(*testing.T, *testutil.CtrlClient)
	}{
		{
			name: "successfully patches both CRDs",
			setupMock: func(c *testutil.CtrlClient) {
				c.On("Patch",
					mock.Anything,
					mock.MatchedBy(func(obj client.Object) bool {
						return obj.GetName() == osCRDName
					}),
					mock.MatchedBy(func(patch client.Patch) bool {
						return patch.Type() == types.JSONPatchType
					}),
					mock.Anything).
					Return(nil).
					Once()

				c.On("Patch",
					mock.Anything,
					mock.MatchedBy(func(obj client.Object) bool {
						return obj.GetName() == cosCRDName
					}),
					mock.MatchedBy(func(patch client.Patch) bool {
						return patch.Type() == types.JSONPatchType
					}),
					mock.Anything).
					Return(nil).
					Once()
			},
			expectedError: false,
			validateCalls: func(t *testing.T, c *testutil.CtrlClient) {
				t.Helper()
				// Verify the patch was called exactly twice
				c.AssertNumberOfCalls(t, "Patch", 2)
			},
		},
		{
			name: "returns error when ObjectSet CRD patch fails",
			setupMock: func(c *testutil.CtrlClient) {
				c.On("Patch",
					mock.Anything,
					mock.MatchedBy(func(obj client.Object) bool {
						return obj.GetName() == osCRDName
					}),
					mock.Anything,
					mock.Anything).
					Return(errTest)
			},
			expectedError: true,
		},
		{
			name: "returns error when ClusterObjectSet CRD patch fails",
			setupMock: func(c *testutil.CtrlClient) {
				c.On("Patch",
					mock.Anything,
					mock.MatchedBy(func(obj client.Object) bool {
						return obj.GetName() == osCRDName
					}),
					mock.Anything,
					mock.Anything).
					Return(nil).
					Once()

				c.On("Patch",
					mock.Anything,
					mock.MatchedBy(func(obj client.Object) bool {
						return obj.GetName() == cosCRDName
					}),
					mock.Anything,
					mock.Anything).
					Return(errTest)
			},
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			c := testutil.NewClient()
			tt.setupMock(c)

			fix := &ControllerOfVersion{}
			err := fix.Run(ctx, Context{Client: c})

			if tt.expectedError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			if tt.validateCalls != nil {
				tt.validateCalls(t, c)
			}
			c.AssertExpectations(t)
		})
	}
}

func TestControllerOfVersion_hasControllerOfVersionField(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		crd            *apiextensionsv1.CustomResourceDefinition
		expectedResult bool
		expectedError  bool
	}{
		{
			name:           "returns true when version field is present",
			crd:            createCRDWithVersionField("test.example.com"),
			expectedResult: true,
			expectedError:  false,
		},
		{
			name: "returns false when version field is not present",
			crd: &apiextensionsv1.CustomResourceDefinition{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test.example.com",
				},
				Spec: apiextensionsv1.CustomResourceDefinitionSpec{
					Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
						{
							Name: "v1alpha1",
							Schema: &apiextensionsv1.CustomResourceValidation{
								OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{
									Type: "object",
									Properties: map[string]apiextensionsv1.JSONSchemaProps{
										"status": {
											Properties: map[string]apiextensionsv1.JSONSchemaProps{
												"controllerOf": {
													Items: &apiextensionsv1.JSONSchemaPropsOrArray{
														Schema: &apiextensionsv1.JSONSchemaProps{
															Properties: map[string]apiextensionsv1.JSONSchemaProps{
																"group": {
																	Type: "string",
																},
															},
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expectedResult: false,
			expectedError:  false,
		},
		{
			name: "returns false when CRD has no schema",
			crd: &apiextensionsv1.CustomResourceDefinition{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test.example.com",
				},
				Spec: apiextensionsv1.CustomResourceDefinitionSpec{
					Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
						{
							Name: "v1alpha1",
						},
					},
				},
			},
			expectedResult: false,
			expectedError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			fix := &ControllerOfVersion{}
			result, err := fix.hasControllerOfVersionField(tt.crd)

			if tt.expectedError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

// Helper function to create a CRD with the version field properly structured.
func createCRDWithVersionField(name string) *apiextensionsv1.CustomResourceDefinition {
	return &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
				{
					Name: "v1alpha1",
					Schema: &apiextensionsv1.CustomResourceValidation{
						OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{
							Type: "object",
							Properties: map[string]apiextensionsv1.JSONSchemaProps{
								"status": {
									Properties: map[string]apiextensionsv1.JSONSchemaProps{
										"controllerOf": {
											Items: &apiextensionsv1.JSONSchemaPropsOrArray{
												Schema: &apiextensionsv1.JSONSchemaProps{
													Properties: map[string]apiextensionsv1.JSONSchemaProps{
														"version": {
															Description: "Object Version.",
															Type:        "string",
														},
														"group": {
															Type: "string",
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}
