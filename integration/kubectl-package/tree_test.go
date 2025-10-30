//go:build integration

package kubectlpackage

import (
	"context"
	"path/filepath"

	"github.com/onsi/ginkgo/v2"
)

var _ = ginkgo.DescribeTable("tree subcommand",
	testSubCommand(context.Background(), "tree"),
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
	ginkgo.Entry("given the path of a valid multi component package",
		subCommandTestCase{
			Args:             []string{sourcePathFixture("valid_without_config_multi")},
			ExpectedExitCode: 0,
			ExpectedOutput: []string{
				"test-stub-multi",
				"Package test-ns/test",
				"└── Phase deploy-backend",
				"│   ├── package-operator.run/v1alpha1, Kind=Package /test-backend",
				"└── Phase deploy-frontend",
				`\s+└── package-operator.run/v1alpha1, Kind=Package /test-frontend`,
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
	ginkgo.Entry("using '--cluster' flag in multi component package",
		subCommandTestCase{
			Args: []string{
				"--cluster",
				sourcePathFixture("valid_without_config_multi"),
			},
			ExpectedExitCode: 0,
			ExpectedOutput: []string{
				"ClusterPackage /test",
				"└── Phase namespace",
				"│   ├── /v1, Kind=Namespace /test",
				"└── Phase deploy-backend",
				"│   ├── package-operator.run/v1alpha1, Kind=ClusterPackage /test-backend",
				"└── Phase deploy-frontend",
				`\s+└── package-operator.run/v1alpha1, Kind=ClusterPackage /test-frontend`,
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
			},
		},
	),
	ginkgo.Entry("given a path of a valid multi component package with configuration, no tests and no required properties",
		subCommandTestCase{
			Args:             []string{sourcePathFixture("valid_with_config_no_tests_no_required_properties_multi")},
			ExpectedExitCode: 0,
			ExpectedOutput: []string{
				"test-stub-multi",
				"Package namespace/name",
				"└── Phase deploy-backend",
				"│   ├── package-operator.run/v1alpha1, Kind=Package /name-backend",
				"└── Phase deploy-frontend",
				`\s+└── package-operator.run/v1alpha1, Kind=Package /name-frontend`,
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
	ginkgo.Entry("using '--config-testcase' flag in multi component package",
		subCommandTestCase{
			Args: []string{
				"--config-testcase", "cluster-scope",
				sourcePathFixture("valid_with_config_multi"),
			},
			ExpectedExitCode: 0,
			ExpectedOutput: []string{
				"ClusterPackage /test1",
				"└── Phase namespace",
				"│   ├── /v1, Kind=Namespace /test1",
				"└── Phase deploy-backend",
				"│   ├── package-operator.run/v1alpha1, Kind=ClusterPackage /test1-backend",
				"└── Phase deploy-frontend",
				`\s+└── package-operator.run/v1alpha1, Kind=ClusterPackage /test1-frontend`,
			},
		},
	),
	ginkgo.Entry("using '--config-path' flag with invalid path",
		subCommandTestCase{
			Args: []string{
				"--config-path", "aoigda-aoishasoiah",
				sourcePathFixture("valid_with_config"),
			},
			ExpectedExitCode: 1,
			ExpectedErrorOutput: []string{
				"no such file or directory",
			},
		},
	),
	ginkgo.Entry("using '--config-path' flag with invalid path in multi component package",
		subCommandTestCase{
			Args: []string{
				"--config-path", "aoigda-apsiodynkld",
				sourcePathFixture("valid_with_config_multi"),
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
	ginkgo.Entry("using '--config-path' flag with bad config file in multi component package",
		subCommandTestCase{
			Args: []string{
				"--config-path", filepath.Join(sourcePathFixture("valid_with_config"), "manifest.yaml"),
				sourcePathFixture("valid_with_config_multi"),
			},
			ExpectedExitCode: 1,
			ExpectedErrorOutput: []string{
				"spec.config.testStubMultiPackageImage: Required value, spec.config.testStubImage: Required value",
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
			},
		},
	),
	ginkgo.Entry("using '--config-path' flag with valid config file in multi component package",
		subCommandTestCase{
			Args: []string{
				"--config-path", filepath.Join(sourcePathFixture("valid_with_config_multi"), ".config.yaml"),
				sourcePathFixture("valid_with_config_multi"),
			},
			ExpectedExitCode: 0,
			ExpectedOutput: []string{
				"test-stub-multi",
				"Package test-ns/test",
				"└── Phase deploy-backend",
				"│   ├── package-operator.run/v1alpha1, Kind=Package /test-backend",
				"└── Phase deploy-frontend",
				`\s+└── package-operator.run/v1alpha1, Kind=Package /test-frontend`,
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
	ginkgo.Entry("using '--config-path' flag with '--config-testcase' flag in multi component package",
		subCommandTestCase{
			Args: []string{
				"--config-path", filepath.Join(sourcePathFixture("valid_with_config_multi"), ".config.yaml"),
				"--config-testcase", "namespace-scope",
				sourcePathFixture("valid_with_config_multi"),
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
