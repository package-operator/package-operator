//go:build integration

package kubectlpackage

import (
	"context"

	"github.com/onsi/ginkgo/v2"
)

var _ = ginkgo.DescribeTable("version subcommand",
	testSubCommand(context.Background(), "version"),
	ginkgo.Entry("using an unknown flag",
		subCommandTestCase{
			Args:             []string{"--unknown"},
			ExpectedExitCode: 1,
		},
	),
)
