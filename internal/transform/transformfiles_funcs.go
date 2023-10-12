package transform

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"sigs.k8s.io/yaml"
)

// allow all sprig functions except dates, random, crypto, os, network and filepath.
var allowedFuncNames = map[string]struct{}{
	"hello": {},

	// Strings
	"abbrev":     {},
	"abbrevboth": {},
	"trunc":      {},
	"trim":       {},
	"upper":      {},
	"lower":      {},
	"title":      {},
	"untitle":    {},
	"substr":     {},
	"repeat":     {},
	"trimAll":    {},
	"trimSuffix": {},
	"trimPrefix": {},
	"nospace":    {},
	"initials":   {},
	"swapcase":   {},
	"snakecase":  {},
	"camelcase":  {},
	"kebabcase":  {},
	"wrap":       {},
	"wrapWith":   {},
	"contains":   {},
	"hasPrefix":  {},
	"hasSuffix":  {},
	"quote":      {},
	"squote":     {},
	"cat":        {},
	"indent":     {},
	"nindent":    {},
	"replace":    {},
	"plural":     {},
	"sha1sum":    {},
	"sha256sum":  {},
	"adler32sum": {},
	"toString":   {},

	// Conversions
	"atoi":      {},
	"int64":     {},
	"int":       {},
	"float64":   {},
	"seq":       {},
	"toDecimal": {},

	// Split
	"split":     {},
	"splitList": {},
	"splitn":    {},
	"toStrings": {},

	// Until
	"until":     {},
	"untilStep": {},

	// Arithmetic
	"add1":    {},
	"add":     {},
	"sub":     {},
	"div":     {},
	"mod":     {},
	"mul":     {},
	"add1f":   {},
	"addf":    {},
	"subf":    {},
	"divf":    {},
	"mulf":    {},
	"biggest": {},
	"max":     {},
	"min":     {},
	"maxf":    {},
	"minf":    {},
	"ceil":    {},
	"floor":   {},
	"round":   {},

	// Join
	"join":      {},
	"sortAlpha": {},

	// Defaults
	"default":          {},
	"empty":            {},
	"coalesce":         {},
	"all":              {},
	"any":              {},
	"compact":          {},
	"mustCompact":      {},
	"fromJson":         {},
	"toJson":           {},
	"toPrettyJson":     {},
	"toRawJson":        {},
	"mustFromJson":     {},
	"mustToJson":       {},
	"mustToPrettyJson": {},
	"mustToRawJson":    {},
	"ternary":          {},
	"deepCopy":         {},
	"mustDeepCopy":     {},

	// Reflection
	"typeOf":     {},
	"typeIs":     {},
	"typeIsLike": {},
	"kindOf":     {},
	"kindIs":     {},
	"deepEqual":  {},

	// Paths
	"base":  {},
	"dir":   {},
	"clean": {},
	"ext":   {},
	"isAbs": {},

	// Encoding
	"b64enc": {},
	"b64dec": {},
	"b32enc": {},
	"b32dec": {},

	// Data Structures
	"tuple":              {},
	"list":               {},
	"dict":               {},
	"get":                {},
	"set":                {},
	"unset":              {},
	"hasKey":             {},
	"pluck":              {},
	"keys":               {},
	"pick":               {},
	"omit":               {},
	"merge":              {},
	"mergeOverwrite":     {},
	"mustMerge":          {},
	"mustMergeOverwrite": {},
	"values":             {},
	"append":             {},
	"mustAppend":         {},
	"push":               {},
	"mustPush":           {},
	"prepend":            {},
	"mustPrepend":        {},
	"first":              {},
	"mustFirst":          {},
	"rest":               {},
	"mustRest":           {},
	"last":               {},
	"mustLast":           {},
	"initial":            {},
	"mustInitial":        {},
	"reverse":            {},
	"mustReverse":        {},
	"uniq":               {},
	"mustUniq":           {},
	"without":            {},
	"mustWithout":        {},
	"has":                {},
	"mustHas":            {},
	"slice":              {},
	"mustSlice":          {},
	"concat":             {},
	"dig":                {},
	"chunk":              {},
	"mustChunk":          {},

	// SemVer
	"semver":        {},
	"semverCompare": {},

	// Flow Control
	"fail": {},

	// Regex
	"regexMatch":                 {},
	"mustRegexMatch":             {},
	"regexFindAll":               {},
	"mustRegexFindAll":           {},
	"regexFind":                  {},
	"mustRegexFind":              {},
	"regexReplaceAll":            {},
	"mustRegexReplaceAll":        {},
	"regexReplaceAllLiteral":     {},
	"mustRegexReplaceAllLiteral": {},
	"regexSplit":                 {},
	"mustRegexSplit":             {},
	"regexQuoteMeta":             {},

	// URLs
	"urlParse": {},
	"urlJoin":  {},
}

func TemplateWithSprigFuncs(content string) (*template.Template, error) {
	tmpl := template.New("").Option("missingkey=error")
	return tmpl.Funcs(SprigFuncs(tmpl)).Parse(content)
}

const recursionDepth = 1000

var ErrExceededIncludeRecursion = errors.New("exceeded max include recursion depth")

func SprigFuncs(t *template.Template) template.FuncMap {
	allowedFuncs := map[string]any{}
	for key, value := range sprig.FuncMap() {
		if _, exists := allowedFuncNames[key]; exists {
			allowedFuncs[key] = value
		}
	}
	allowedFuncs["b64decMap"] = base64decodeMap

	includedNames := map[string]int{}
	// Include function executes a template with given data and returns the result as string.
	// Use this helper function if you need to modify the resulting output via e.g. | indent.
	// Example:
	// {{- define "test-helper" -}}{{.}}{{- end -}}{{- include "test-helper" . | upper -}}
	allowedFuncs["include"] = func(name string, data interface{}) (string, error) {
		var buf strings.Builder
		if v, ok := includedNames[name]; ok {
			if v > recursionDepth {
				return "", fmt.Errorf("including template with name %s: %w", name, ErrExceededIncludeRecursion)
			}
			includedNames[name]++
		} else {
			includedNames[name] = 1
		}
		err := t.ExecuteTemplate(&buf, name, data)
		includedNames[name]--
		return buf.String(), err
	}

	allowedFuncs["toYAML"] = toYAML
	allowedFuncs["fromYAML"] = fromYAML
	return allowedFuncs
}

func base64decodeMap(data map[string]interface{}) (
	map[string]interface{}, error,
) {
	decodedData := map[string]interface{}{}
	for k, vi := range data {
		v, ok := vi.(string)
		if !ok {
			continue
		}

		decodedV, err := base64.StdEncoding.DecodeString(v)
		if err != nil {
			return nil, fmt.Errorf(
				"decode base64 value at key %s: %w", k, err)
		}
		decodedData[k] = string(decodedV)
	}

	return decodedData, nil
}

func toYAML(v interface{}) (string, error) {
	data, err := yaml.Marshal(v)
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(string(data), "\n"), nil
}

var ErrInvalidType = errors.New("invalid type")

func fromYAML(y interface{}) (out interface{}, err error) {
	var b []byte
	switch v := y.(type) {
	case string:
		b = []byte(v)
	case []byte:
		b = v
	default:
		return nil, fmt.Errorf(
			"fromYAML requires string or []byte as input: %w", ErrInvalidType)
	}

	return out, yaml.Unmarshal(b, &out)
}
