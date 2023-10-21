/* #nosec */

package kubectlpackage

import (
	"path/filepath"

	"github.com/onsi/ginkgo/v2"
)

var _ = ginkgo.DescribeTable("tree subcommand",
	testSubCommand("tree"),
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
	ginkgo.Entry("given the path of a valid package",
		subCommandTestCase{
			Args:             []string{sourcePathFixture("valid_without_config")},
			ExpectedExitCode: 0,
			ExpectedOutput: []string{
				"test-stub",
				"Package test-ns/test",
				"└── Phase deploy",
				`\s+└── apps/v1, Kind=Deployment /test-stub-test`,
			},
		},
	),
	ginkgo.Entry("given the path of an invalid package",
		subCommandTestCase{
			Args:             []string{sourcePathFixture("invalid_bad_manifest")},
			ExpectedExitCode: 1,
			ExpectedErrorOutput: []string{
				"spec.scopes: Required value",
			},
		},
	),
	ginkgo.Entry("using '--cluster' flag",
		subCommandTestCase{
			Args: []string{
				"--cluster",
				sourcePathFixture("valid_without_config"),
			},
			ExpectedExitCode: 0,
			ExpectedOutput: []string{
				"ClusterPackage /test",
				"└── Phase namespace",
				"│   ├── /v1, Kind=Namespace /test",
				"└── Phase deploy",
				`\s+└── apps/v1, Kind=Deployment test/test-stub-test`,
			},
		},
	),
	ginkgo.Entry("given a path of a valid package with configuration, no tests and no required properties",
		subCommandTestCase{
			Args:             []string{sourcePathFixture("valid_with_config_no_tests_no_required_properties")},
			ExpectedExitCode: 0,
			ExpectedOutput: []string{
				"test-stub",
				"Package namespace/name",
				"└── Phase deploy",
				`\s+└── apps/v1, Kind=Deployment /test-stub-name`,
				`\s+└── apps/v1, Kind=Deployment external-name/test-external-name \(EXTERNAL\)`,
			},
		},
	),
	ginkgo.Entry("using '--config-testcase' flag",
		subCommandTestCase{
			Args: []string{
				"--config-testcase", "cluster-scope",
				sourcePathFixture("valid_with_config"),
			},
			ExpectedExitCode: 0,
			ExpectedOutput: []string{
				"test-stub",
				"ClusterPackage /test",
				"└── Phase namespace",
				"│   ├── /v1, Kind=Namespace /test",
				"└── Phase deploy",
				`\s+└── apps/v1, Kind=Deployment test/test-stub-test`,
			},
		},
	),
	ginkgo.Entry("using '--config-path' flag with invalid path",
		subCommandTestCase{
			Args: []string{
				"--config-path", "dne",
				sourcePathFixture("valid_with_config"),
			},
			ExpectedExitCode: 1,
			ExpectedErrorOutput: []string{
				"no such file or directory",
			},
		},
	),
	ginkgo.Entry("using '--config-path' flag with bad config file",
		subCommandTestCase{
			Args: []string{
				"--config-path", filepath.Join(sourcePathFixture("valid_with_config"), "manifest.yaml"),
				sourcePathFixture("valid_with_config"),
			},
			ExpectedExitCode: 1,
			ExpectedErrorOutput: []string{
				"spec.config.image: Required value",
			},
		},
	),
	ginkgo.Entry("using '--config-path' flag with valid config file",
		subCommandTestCase{
			Args: []string{
				"--config-path", filepath.Join(sourcePathFixture("valid_with_config"), ".config.yaml"),
				sourcePathFixture("valid_with_config"),
			},
			ExpectedExitCode: 0,
			ExpectedOutput: []string{
				"test-stub",
				"Package namespace/name",
				"└── Phase deploy",
				`\s+└── apps/v1, Kind=Deployment /test-stub-name`,
				`\s+└── apps/v1, Kind=Deployment external-name/test-external-name \(EXTERNAL\)`,
			},
		},
	),
	ginkgo.Entry("using '--config-path' flag with '--config-testcase' flag",
		subCommandTestCase{
			Args: []string{
				"--config-path", filepath.Join(sourcePathFixture("valid_with_config"), ".config.yaml"),
				"--config-testcase", "namespace-scope",
				sourcePathFixture("valid_with_config"),
			},
			ExpectedExitCode: 1,
			ExpectedErrorOutput: []string{
				`\[config-path config-testcase\] were all set`,
			},
		},
	),
	// TODO: When not using --cluster and given a package with no namespace should render package at cluster scope
	// TODO: When using --config-testcase and given testcase with invalid config should fail
)
