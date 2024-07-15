package utils

import (
	"errors"
	"fmt"
	"regexp"
)

var jsonRegexp = regexp.MustCompile(`^\{\.?([^{}]+)\}$|^\.?([^{}]+)$`)

// RelaxedJSONPathExpression attempts to be flexible with JSONPath expressions, it accepts:
//   - metadata.name (no leading '.' or curly braces '{...}'
//   - {metadata.name} (no leading '.')
//   - .metadata.name (no curly braces '{...}')
//   - {.metadata.name} (complete expression)
//
// And transforms them all into a valid jsonpath expression:
//
//	{.metadata.name}
//
//nolint:goerr113
func RelaxedJSONPathExpression(pathExpression string) (string, error) {
	if len(pathExpression) == 0 {
		return pathExpression, nil
	}
	submatches := jsonRegexp.FindStringSubmatch(pathExpression)
	if submatches == nil {
		err := errors.New("path string, expected a 'name1.name2' or '.name1.name2' or '{name1.name2}' or '{.name1.name2}'")
		return "", err
	}
	if len(submatches) != 3 {
		return "", fmt.Errorf("unexpected submatch list: %v", submatches)
	}
	var fieldSpec string
	if len(submatches[1]) != 0 {
		fieldSpec = submatches[1]
	} else {
		fieldSpec = submatches[2]
	}
	return fmt.Sprintf("{.%s}", fieldSpec), nil
}
