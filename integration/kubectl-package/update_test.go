/* #nosec */

package kubectlpackage

import (
	. "github.com/onsi/ginkgo/v2"
)

var _ = DescribeTable("update subcommand",
	testSubCommand("update"),
	Entry("given no path",
		subCommandTestCase{
			ExpectedExitCode: 1,
		},
	),
	Entry("given an invalid path",
		subCommandTestCase{
			Args:             []string{"dne"},
			ExpectedExitCode: 1,
		},
	),
	// TODO: Add test registry
	// When given a valid source path which requires update, but cannot be written to should fail
	// TODO: Add test registry
	// When given a valid source path and lock file with unresolvable lock images should fail
	// TODO: Add test registry
	// When given a valid source path and no updates to lock file are required should succeed with no action
)
