package util

import (
	"fmt"
	"strings"

	internalcmd "package-operator.run/internal/cmd"
)

func ParseResourceName(args []string) (*Arguments, error) {
	switch len(args) {
	case 1:
		parts := strings.SplitN(args[0], "/", 2)
		if len(parts) < 2 {
			return nil, fmt.Errorf(
				"%w: arguments in resource/name form must have a single resource and name",
				internalcmd.ErrInvalidArgs,
			)
		}

		return &Arguments{
			Resource: parts[0],
			Name:     parts[1],
		}, nil
	case 2:
		return &Arguments{
			Resource: args[0],
			Name:     args[1],
		}, nil
	default:
		return nil, fmt.Errorf(
			"%w: no less than 1 and no more than 2 arguments may be provided",
			internalcmd.ErrInvalidArgs,
		)
	}
}

type Arguments struct {
	Resource string
	Name     string
}
