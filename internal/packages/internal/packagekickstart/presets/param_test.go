package presets

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParametrizeOptionsFromFlags(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		opts []string
		o    ParametrizeOptions
	}{
		{
			name: "all",
			opts: []string{allParam},
			o: ParametrizeOptions{
				Namespaces:    true,
				Replicas:      true,
				Tolerations:   true,
				NodeSelectors: true,
				Resources:     true,
				Env:           true,
				Images:        true,
			},
		},
		{
			name: "namespaces",
			opts: []string{namespaceParam},
			o: ParametrizeOptions{
				Namespaces: true,
			},
		},
		{
			name: "nodeSelectors",
			opts: []string{nodeSelectorsParam},
			o: ParametrizeOptions{
				NodeSelectors: true,
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			o := ParametrizeOptionsFromFlags(test.opts)
			assert.Equal(t, test.o, o)
		})
	}
}
