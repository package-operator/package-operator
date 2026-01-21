//go:build integration

package kubectlpackage

import (
	"context"
	"path/filepath"

	"github.com/onsi/ginkgo/v2"
)

var _ = ginkgo.DescribeTable("update subcommand",
	testSubCommand(context.Background(), "update"),
	ginkgo.Entry("given no path",
		subCommandTestCase{
			ExpectedExitCode: 1,
		},
	),
	ginkgo.Entry("given an invalid path",
		subCommandTestCase{
			Args:             []string{"dne"},
			ExpectedExitCode: 1,
		},
	),
	ginkgo.Entry("given a valid package",
		subCommandTestCase{
			Args: []string{
				"--insecure",
				filepath.Join(TempDirPlaceholder, "valid_package"),
			},
			ExpectedExitCode: 0,
		},
	),
	ginkgo.Entry("given a valid package with a valid lock file",
		subCommandTestCase{
			Args: []string{
				"--insecure",
				filepath.Join(TempDirPlaceholder, "valid_package_valid_lockfile"),
			},
			ExpectedExitCode: 0,
			ExpectedOutput: []string{
				"Package is already up-to-date",
			},
		},
	),
	ginkgo.Entry("given a valid package with lock file containing unresolvable images",
		subCommandTestCase{
			Args: []string{
				"--insecure",
				filepath.Join(TempDirPlaceholder, "valid_package_invalid_lockfile_unresolvable_images"),
			},
			ExpectedExitCode: 1,
			ExpectedErrorOutput: []string{
				"resolving image digest",
			},
		},
	),
)
