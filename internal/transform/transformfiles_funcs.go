package transform

import (
	"text/template"

	"github.com/Masterminds/sprig/v3"
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
	"shuffle":    {},
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
	return template.New("").Option("missingkey=error").Funcs(SprigFuncs()).Parse(content)
}

func SprigFuncs() template.FuncMap {
	allowedFuncs := map[string]any{}
	for key, value := range sprig.FuncMap() {
		if _, exists := allowedFuncNames[key]; exists {
			allowedFuncs[key] = value
		}
	}
	return allowedFuncs
}
