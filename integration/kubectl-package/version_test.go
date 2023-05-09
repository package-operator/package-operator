/* #nosec */

package kubectlpackage

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
)

var _ = DescribeTable("version subcommand",
	testSubCommand("version"),
	Entry("given no options",
		subCommandTestCase{
			ExpectedExitCode: 0,
			ExpectedOutput: []string{
				"version " + version,
			},
		},
	),
	Entry("using the '--embedded' option",
		subCommandTestCase{
			Args:             []string{"--embedded"},
			ExpectedExitCode: 0,
			ExpectedOutput: []string{
				"version " + version,
				fmt.Sprintf("path %s/cmd/kubectl-package", module),
			},
		},
	),
	Entry("using an unknown flag",
		subCommandTestCase{
			Args:             []string{"--unknown"},
			ExpectedExitCode: 1,
		},
	),
)
