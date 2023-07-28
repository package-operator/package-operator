package transform

import (
	"fmt"
	"testing"

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

	actual := SprigFuncs()

	require.Equal(t, len(allowedFuncNames)+1, len(actual))

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

func Test_base64decodeMap(t *testing.T) {
	t.Parallel()
	d := map[string]interface{}{
		"test": "YWJjZGVm",
	}
	out, err := base64decodeMap(d)
	require.NoError(t, err)

	assert.Equal(t, map[string]interface{}{
		"test": "abcdef",
	}, out)
}
