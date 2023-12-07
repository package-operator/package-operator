//go:build integration

package kubectlpackage

import (
	"path"

	"github.com/onsi/ginkgo/v2"
)

var _ = ginkgo.DescribeTable("validate subcommand",
	testSubCommand("validate"),
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
	ginkgo.Entry("given the path of a package with an invalid manifest",
		subCommandTestCase{
			Args:                []string{sourcePathFixture("invalid_bad_manifest")},
			ExpectedExitCode:    1,
			ExpectedErrorOutput: []string{"spec.scopes: Required value"},
		},
	),
	ginkgo.Entry("given the path of a valid package",
		subCommandTestCase{
			Args:             []string{sourcePathFixture("valid_without_config")},
			ExpectedExitCode: 0,
		},
	),
	ginkgo.Entry("given the path of a valid multi component package",
		subCommandTestCase{
			Args:             []string{sourcePathFixture("valid_without_config_multi")},
			ExpectedExitCode: 0,
		},
	),
	ginkgo.Entry("given the path of a valid package with configuration",
		subCommandTestCase{
			Args:             []string{sourcePathFixture("valid_with_config")},
			ExpectedExitCode: 0,
		},
	),
	ginkgo.Entry("given the path of a valid multi component package with configuration",
		subCommandTestCase{
			Args:             []string{sourcePathFixture("valid_with_config_multi")},
			ExpectedExitCode: 0,
		},
	),
	ginkgo.Entry("given the path of a valid package with configuration, no tests and no required properties",
		subCommandTestCase{
			Args:             []string{sourcePathFixture("valid_with_config_no_tests_no_required_properties")},
			ExpectedExitCode: 0,
		},
	),
	ginkgo.Entry("given the path of a valid multi component package with configuration, no tests and no required properties",
		subCommandTestCase{
			Args:             []string{sourcePathFixture("valid_with_config_no_tests_no_required_properties_multi")},
			ExpectedExitCode: 0,
		},
	),
	ginkgo.Entry("given the path of an invalid package with resource missing phase annotation",
		subCommandTestCase{
			Args:                []string{sourcePathFixture("invalid_missing_phase_annotation")},
			ExpectedExitCode:    1,
			ExpectedErrorOutput: []string{"Missing package-operator.run/phase Annotation"},
		},
	),
	ginkgo.Entry("given the path of an invalid package with resource missing gvk",
		subCommandTestCase{
			Args:                []string{sourcePathFixture("invalid_missing_resource_gvk")},
			ExpectedExitCode:    1,
			ExpectedErrorOutput: []string{"Object 'Kind' is missing"},
		},
	),
	ginkgo.Entry("given the path of an invalid package with resource having invalid labels",
		subCommandTestCase{
			Args:                []string{sourcePathFixture("invalid_invalid_resource_label")},
			ExpectedExitCode:    1,
			ExpectedErrorOutput: []string{"Labels invalid"},
		},
	),
	ginkgo.Entry("using the '--pull' option with an invalid image reference",
		subCommandTestCase{
			Args:             []string{"--pull", path.Join(registryPlaceholder, "dne")},
			ExpectedExitCode: 1,
			ExpectedErrorOutput: []string{
				"Error: validating package: getting package from remote reference",
			},
		},
	),
	ginkgo.Entry("using the '--pull' option with a valid reference to an invalid package",
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
	ginkgo.Entry("using the '--pull' option with a valid image reference to a valid package",
		subCommandTestCase{
			Args: []string{
				"--insecure",
				"--pull", path.Join(registryPlaceholder, "valid-package-fixture"),
			},
			ExpectedExitCode: 0,
		},
	),
)
