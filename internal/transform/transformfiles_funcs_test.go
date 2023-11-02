package transform

import (
	"bytes"
	"fmt"
	"testing"
	"text/template"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var forbiddenFuncs = []string{
	// Dates
	"ago",
	"date",
	"date_in_zone",
	"date_modify",
	"dateInZone",
	"dateModify",
	"duration",
	"durationRound",
	"htmlDate",
	"htmlDateInZone",
	"must_date_modify",
	"mustDateModify",
	"mustToDate",
	"now",
	"toDate",
	"unixEpoch",
	// Random
	"randAlphaNum",
	"randAlpha",
	"randAscii",
	"randNumeric",
	"randInt",
	"randBytes",
	"shuffle",
	// Crypto
	"bcrypt",
	"htpasswd",
	"genPrivateKey",
	"derivePassword",
	"buildCustomCert",
	"genCA",
	"genCAWithKey",
	"genSelfSignedCert",
	"genSelfSignedCertWithKey",
	"genSignedCert",
	"genSignedCertWithKey",
	"encryptAES",
	"decryptAES",
	"uuidv4",
	// OS
	"env",
	"expandenv",
	// Network
	"getHostByName",
	// Filepath
	"osBase",
	"osDir",
	"osClean",
	"osExt",
	"osIsAbs",
	// Deprecated
	"trimall",
}

func TestSprigAllowedFuncs(t *testing.T) {
	t.Parallel()

	tmpl := template.New("xxx")
	actual := SprigFuncs(tmpl)

	require.Equal(t, len(allowedFuncNames)+4, len(actual))

	for key := range allowedFuncNames {
		require.Contains(t, actual, key)
	}
}

func TestSprigForbiddenFuncs(t *testing.T) {
	t.Parallel()

	for i := range forbiddenFuncs {
		funcName := forbiddenFuncs[i]
		t.Run(funcName, func(t *testing.T) {
			t.Parallel()
			input := fmt.Sprintf("{{ %s }}", funcName)
			expectedErrMsg := fmt.Sprintf("template: :1: function \"%s\" not defined", funcName)
			_, err := TemplateWithSprigFuncs(input)
			require.ErrorContains(t, err, expectedErrMsg)
		})
	}
}

func Test_include(t *testing.T) {
	t.Parallel()

	tmpl := template.New("xxx")
	tmpl = tmpl.Funcs(SprigFuncs(tmpl))

	_, err := tmpl.Parse(`{{- define "test-helper" -}}{{.}}{{- end -}}{{- include "test-helper" . | upper -}}`)
	require.NoError(t, err)

	var buf bytes.Buffer
	require.NoError(t, tmpl.Execute(&buf, "test"))

	assert.Equal(t, "TEST", buf.String())
}

func Test_include_recursionError(t *testing.T) {
	t.Parallel()

	tmpl := template.New("xxx")
	tmpl = tmpl.Funcs(SprigFuncs(tmpl))

	_, err := tmpl.Parse(
		`{{- define "test-helper" -}}{{- include "test-helper" . -}}{{- end -}}{{- include "test-helper" . -}}`,
	)
	require.NoError(t, err)

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, "test")
	assert.ErrorIs(t, err, ErrExceededIncludeRecursion)
}

func Test_base64decodeMap(t *testing.T) {
	t.Parallel()
	d := map[string]any{
		"test": "YWJjZGVm",
	}
	out, err := base64decodeMap(d)
	require.NoError(t, err)

	assert.Equal(t, map[string]any{
		"test": "abcdef",
	}, out)
}

func Test_toYAML(t *testing.T) {
	t.Parallel()

	obj := map[string]string{
		"t": "2",
	}
	y, err := toYAML(obj)
	require.NoError(t, err)
	assert.Equal(t, `t: "2"`, y)
}

func Test_fromYAML(t *testing.T) {
	t.Parallel()

	t.Run("string", func(t *testing.T) {
		t.Parallel()

		y, err := fromYAML(`t: "2"`)
		require.NoError(t, err)
		assert.Equal(t, map[string]any{
			"t": "2",
		}, y)
	})

	t.Run("[]byte", func(t *testing.T) {
		t.Parallel()

		y, err := fromYAML([]byte(`t: "2"`))
		require.NoError(t, err)
		assert.Equal(t, map[string]any{
			"t": "2",
		}, y)
	})

	t.Run("error", func(t *testing.T) {
		t.Parallel()

		_, err := fromYAML(map[string]any{})
		require.ErrorIs(t, err, ErrInvalidType)
	})
}
