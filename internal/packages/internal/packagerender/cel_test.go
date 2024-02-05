package packagerender

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"package-operator.run/internal/packages/internal/packagetypes"
)

func Test_evaluateCELCondition(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name       string
		expression string
		expected   bool
		err        string
	}{
		{
			"just true",
			"true",
			true,
			"",
		},
		{
			"simple &&",
			"true && false",
			false,
			"",
		},

		{
			"invalid expression",
			"true && fals",
			false,
			"compile error: ERROR: <input>:1:9: undeclared reference to 'fals' (in container '')\n" +
				" | true && fals\n" +
				" | ........^",
		},
		{
			"invalid return type",
			"2 + 3",
			false,
			"invalid return type: int, expected bool",
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result, err := evaluateCELCondition(tc.expression, packagetypes.PackageRenderContext{})
			if tc.err == "" {
				require.NoError(t, err)
				assert.Equal(t, tc.expected, result)
			} else {
				require.EqualError(t, err, tc.err)
			}
		})
	}
}
