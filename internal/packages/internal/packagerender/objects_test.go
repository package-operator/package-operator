package packagerender

import (
	"reflect"
	"testing"

	"package-operator.run/internal/apis/manifests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"package-operator.run/apis/manifests/v1alpha1"

	"package-operator.run/internal/packages/internal/packagerender/celctx"
	"package-operator.run/internal/packages/internal/packagetypes"
)

func newConfigMap(name, cel string) unstructured.Unstructured {
	cm := unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]any{
				"name": "cm-" + name,
			},
			"data": map[string]any{
				"banana": "bread",
			},
		},
	}

	if cel != "" {
		cm.SetAnnotations(map[string]string{v1alpha1.PackageCELConditionAnnotation: cel})
	}

	return cm
}

func TestFilterWithCELAnnotation(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name            string
		objects         []unstructured.Unstructured
		tmplCtx         packagetypes.PackageRenderContext
		conditions      []manifests.PackageManifestNamedCondition
		filtered        []unstructured.Unstructured
		filteredIndexes []int
		err             string
	}{
		{
			name:       "no annotation",
			objects:    []unstructured.Unstructured{newConfigMap("a", "")},
			tmplCtx:    packagetypes.PackageRenderContext{},
			conditions: nil,
			filtered:   []unstructured.Unstructured{newConfigMap("a", "")},
			err:        "",
		},
		{
			name:            "simple annotation",
			objects:         []unstructured.Unstructured{newConfigMap("a", ""), newConfigMap("b", "true && false")},
			tmplCtx:         packagetypes.PackageRenderContext{},
			conditions:      nil,
			filtered:        []unstructured.Unstructured{newConfigMap("a", "")},
			filteredIndexes: []int{1},
			err:             "",
		},
		{
			name:    "condition annotation",
			objects: []unstructured.Unstructured{newConfigMap("a", ""), newConfigMap("b", "false || cond.mycondition")},
			tmplCtx: packagetypes.PackageRenderContext{},
			conditions: []manifests.PackageManifestNamedCondition{
				{Name: "mycondition", Expression: "false"},
			},
			filtered:        []unstructured.Unstructured{newConfigMap("a", "")},
			filteredIndexes: []int{1},
			err:             "",
		},
		{
			name:    "condition annotation",
			objects: []unstructured.Unstructured{newConfigMap("a", ""), newConfigMap("b", "false || cond.mycondition")},
			tmplCtx: packagetypes.PackageRenderContext{},
			conditions: []manifests.PackageManifestNamedCondition{
				{Name: "mycondition", Expression: "true"},
			},
			filtered: []unstructured.Unstructured{newConfigMap("a", ""), newConfigMap("b", "false || cond.mycondition")},
			err:      "",
		},
		{
			name:       "invalid expression",
			objects:    []unstructured.Unstructured{newConfigMap("a", "invalid && expression")},
			tmplCtx:    packagetypes.PackageRenderContext{},
			conditions: nil,
			filtered:   nil,
			err:        string(packagetypes.ViolationReasonInvalidCELExpression),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cc, err := celctx.New(tc.conditions, tc.tmplCtx)
			require.NoError(t, err)

			filtered, filteredIndexes, err := filterWithCELAnnotation(tc.objects, cc)
			if tc.err == "" {
				require.NoError(t, err)
				require.Equal(t, len(tc.filtered), len(filtered))
				require.Equal(t, tc.filteredIndexes, filteredIndexes)
				for i := range len(filtered) {
					assert.Equal(t, tc.filtered[i], filtered[i])
				}
			} else {
				require.ErrorContains(t, err, tc.err)
			}
		})
	}
}

func TestFilterWithCEL(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name              string
		pathObjectMap     map[string][]unstructured.Unstructured
		tmplCtx           packagetypes.PackageRenderContext
		condFiltering     manifests.PackageManifestConditionalFiltering
		filtered          map[string][]unstructured.Unstructured
		pathFilteredIndex map[string][]int
		err               string
	}{
		{
			name: "no filtering",
			pathObjectMap: map[string][]unstructured.Unstructured{
				"a": {newConfigMap("a", "")},
			},
			tmplCtx:       packagetypes.PackageRenderContext{},
			condFiltering: manifests.PackageManifestConditionalFiltering{},
			filtered: map[string][]unstructured.Unstructured{
				"a": {newConfigMap("a", "")},
			},
			pathFilteredIndex: map[string][]int{},
			err:               "",
		},
		{
			name: "simple filtering",
			pathObjectMap: map[string][]unstructured.Unstructured{
				"a": {newConfigMap("a", "cond.justTrue")},
				"b": {newConfigMap("b", "true")},
			},
			tmplCtx: packagetypes.PackageRenderContext{},
			condFiltering: manifests.PackageManifestConditionalFiltering{
				NamedConditions: []manifests.PackageManifestNamedCondition{
					{Name: "justTrue", Expression: "true"},
				},
				ConditionalPaths: []manifests.PackageManifestConditionalPath{
					{Glob: "b", Expression: "!cond.justTrue"},
				},
			},
			filtered: map[string][]unstructured.Unstructured{
				"a": {newConfigMap("a", "cond.justTrue")},
			},
			pathFilteredIndex: map[string][]int{},
			err:               "",
		},
		{
			name: "invalid CEL annotation",
			pathObjectMap: map[string][]unstructured.Unstructured{
				"a": {newConfigMap("a", "fals")},
			},
			tmplCtx:       packagetypes.PackageRenderContext{},
			condFiltering: manifests.PackageManifestConditionalFiltering{},
			filtered:      nil,
			err:           string(packagetypes.ViolationReasonInvalidCELExpression),
		},
		{
			name:          "invalid condition expression",
			pathObjectMap: nil,
			tmplCtx:       packagetypes.PackageRenderContext{},
			condFiltering: manifests.PackageManifestConditionalFiltering{
				NamedConditions: []manifests.PackageManifestNamedCondition{
					{Name: "invalid", Expression: "fals"},
				},
			},
			filtered: nil,
			err:      celctx.ErrCELConditionEvaluation.Error(),
		},
		{
			name:          "invalid conditional path expression",
			pathObjectMap: nil,
			tmplCtx:       packagetypes.PackageRenderContext{},
			condFiltering: manifests.PackageManifestConditionalFiltering{
				ConditionalPaths: []manifests.PackageManifestConditionalPath{
					{Glob: "invalid", Expression: "fals"},
				},
			},
			filtered: nil,
			err:      ErrInvalidConditionalPathsExpression.Error(),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			pathFilteredIndex, err := filterWithCEL(tc.pathObjectMap, tc.condFiltering, tc.tmplCtx)
			if tc.err == "" {
				require.NoError(t, err)
				assert.True(t, reflect.DeepEqual(tc.pathObjectMap, tc.filtered))
				assert.Equal(t, tc.pathFilteredIndex, pathFilteredIndex)
			} else {
				require.ErrorContains(t, err, tc.err)
			}
		})
	}
}

func TestComputeIgnoredPaths(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name             string
		conditionalPaths []manifests.PackageManifestConditionalPath
		tmplCtx          packagetypes.PackageRenderContext
		conditions       []manifests.PackageManifestNamedCondition
		result           []string
		err              error
	}{
		{
			name:             "no paths",
			conditionalPaths: nil,
			tmplCtx:          packagetypes.PackageRenderContext{},
			conditions:       nil,
			result:           []string{},
			err:              nil,
		},
		{
			name: "simple paths",
			conditionalPaths: []manifests.PackageManifestConditionalPath{
				{
					Glob:       "banana*",
					Expression: "false",
				},
				{
					Glob:       "*bread",
					Expression: "true",
				},
			},
			tmplCtx:    packagetypes.PackageRenderContext{},
			conditions: nil,
			result:     []string{"banana*"},
			err:        nil,
		},
		{
			name: "invalid expression",
			conditionalPaths: []manifests.PackageManifestConditionalPath{
				{
					Glob:       "bananas/**",
					Expression: "notValid",
				},
			},
			tmplCtx:    packagetypes.PackageRenderContext{},
			conditions: nil,
			result:     nil,
			err:        ErrInvalidConditionalPathsExpression,
		},
		{
			name: "use context and conditions",
			conditionalPaths: []manifests.PackageManifestConditionalPath{
				{
					Glob:       "ignored",
					Expression: ".config.banana == \"notBread\"",
				},
				{
					Glob:       "notIgnored",
					Expression: "cond.justTrue",
				},
			},
			tmplCtx: packagetypes.PackageRenderContext{
				Package:     manifests.TemplateContextPackage{},
				Config:      map[string]any{"banana": "bread"},
				Images:      nil,
				Environment: manifests.PackageEnvironment{},
			},
			conditions: []manifests.PackageManifestNamedCondition{{Name: "justTrue", Expression: "true"}},
			result:     []string{"ignored"},
			err:        nil,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cc, err := celctx.New(tc.conditions, tc.tmplCtx)
			require.NoError(t, err)

			ignoredPaths, err := computeIgnoredPaths(tc.conditionalPaths, cc)
			if tc.err != nil {
				require.ErrorIs(t, err, tc.err)
			} else {
				require.NoError(t, err)
				assert.ElementsMatch(t, ignoredPaths, tc.result)
			}
		})
	}
}

func TestIsExcluded(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name           string
		path           string
		pathsToExclude []string
		result         bool
	}{
		{
			name:           "no paths",
			path:           "banana",
			pathsToExclude: []string{},
			result:         false,
		},
		{
			name:           "exclude",
			path:           "should/be/excluded",
			pathsToExclude: []string{"should/**/exclude?"},
			result:         true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			excluded, err := isExcluded(tc.path, tc.pathsToExclude)
			require.NoError(t, err)
			assert.Equal(t, tc.result, excluded)
		})
	}
}
