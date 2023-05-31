/* #nosec */

package kubectlpackage

import (
	"path"

	. "github.com/onsi/ginkgo/v2"
)

var _ = DescribeTable("validate subcommand",
	testSubCommand("validate"),
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
	Entry("given the path of a package with an invalid manifest",
		subCommandTestCase{
			Args:                []string{sourcePathFixture("invalid_bad_manifest")},
			ExpectedExitCode:    1,
			ExpectedErrorOutput: []string{"spec.scopes: Required value"},
		},
	),
	Entry("given the path of a valid package",
		subCommandTestCase{
			Args:             []string{sourcePathFixture("valid_without_config")},
			ExpectedExitCode: 0,
		},
	),
	Entry("given the path of an invalid package with resource missing phase annotation",
		subCommandTestCase{
			Args:                []string{sourcePathFixture("invalid_missing_phase_annotation")},
			ExpectedExitCode:    1,
			ExpectedErrorOutput: []string{"Missing package-operator.run/phase Annotation"},
		},
	),
	Entry("given the path of an invalid package with resource missing gvk",
		subCommandTestCase{
			Args:                []string{sourcePathFixture("invalid_missing_resource_gvk")},
			ExpectedExitCode:    1,
			ExpectedErrorOutput: []string{"Object 'Kind' is missing"},
		},
	),
	Entry("given the path of an invalid package with resource having invalid labels",
		subCommandTestCase{
			Args:                []string{sourcePathFixture("invalid_invalid_resource_label")},
			ExpectedExitCode:    1,
			ExpectedErrorOutput: []string{"Labels invalid"},
		},
	),
	Entry("using the '--pull' option with an invalid image reference",
		subCommandTestCase{
			Args:             []string{"--pull", path.Join(registryPlaceholder, "dne")},
			ExpectedExitCode: 1,
			ExpectedErrorOutput: []string{
				"Error: validating package: getting package from remote reference",
			},
		},
	),
	Entry("using the '--pull' option with a valid reference to an invalid package",
		subCommandTestCase{
			Args: []string{
				"--insecure",
				"--pull", path.Join(registryPlaceholder, "invalid-package-fixture"),
			},
			ExpectedExitCode: 1,
			ExpectedErrorOutput: []string{
				"spec.scopes: Required value",
			},
		},
	),
	Entry("using the '--pull' option with a valid image reference to a valid package",
		subCommandTestCase{
			Args: []string{
				"--insecure",
				"--pull", path.Join(registryPlaceholder, "valid-package-fixture"),
			},
			ExpectedExitCode: 0,
		},
	),
)
