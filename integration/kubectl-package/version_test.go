/* #nosec */

package kubectlpackage

import (
	. "github.com/onsi/ginkgo/v2"
)

var _ = DescribeTable("version subcommand",
	testSubCommand("version"),
	Entry("using an unknown flag",
		subCommandTestCase{
			Args:             []string{"--unknown"},
			ExpectedExitCode: 1,
		},
	),
)
