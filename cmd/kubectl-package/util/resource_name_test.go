package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	internalcmd "package-operator.run/internal/cmd"
)

func TestParseResourceName(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name        string
		args        []string
		result      *Arguments
		errContains string
	}{
		{
			name: "valid single argument with resource/name format",
			args: []string{"resource/myname"},
			result: &Arguments{
				Resource: "resource",
				Name:     "myname",
			},
		},
		{
			name: "valid two arguments",
			args: []string{"resource", "myname"},
			result: &Arguments{
				Resource: "resource",
				Name:     "myname",
			},
		},
		{
			name:        "invalid single argument without slash",
			args:        []string{"resourceonly"},
			result:      nil,
			errContains: "arguments in resource/name form must have a single resource and name",
		},
		{
			name:        "no arguments",
			args:        []string{},
			result:      nil,
			errContains: "no less than 1 and no more than 2 arguments may be provided",
		},
		{
			name:        "too many arguments",
			args:        []string{"resource", "name", "extra"},
			result:      nil,
			errContains: "no less than 1 and no more than 2 arguments may be provided",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			resName, err := ParseResourceName(tc.args)

			if tc.errContains == "" {
				require.NoError(t, err)
				assert.Equal(t, tc.result, resName)
				return
			}

			require.ErrorIs(t, err, internalcmd.ErrInvalidArgs)
			require.ErrorContains(t, err, tc.errContains)
			assert.Nil(t, resName)
		})
	}
}
