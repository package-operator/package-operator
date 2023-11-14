/* #nosec */

package kubectlpackage

import (
	"path"
	"path/filepath"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

var _ = ginkgo.DescribeTable("build subcommand",
	testSubCommand("build"),
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
		},
	),
	ginkgo.Entry("given the path of a package with an invalid manifest",
		subCommandTestCase{
			Args:                []string{sourcePathFixture("invalid_bad_manifest")},
			ExpectedExitCode:    1,
			ExpectedErrorOutput: []string{"spec.scopes: Required value"},
		},
	),
	ginkgo.Entry("given the path of a package with images, but no lock file",
		subCommandTestCase{
			Args:                []string{sourcePathFixture("invalid_missing_lock_file")},
			ExpectedExitCode:    1,
			ExpectedErrorOutput: []string{"manifest.lock.yaml is missing"},
		},
	),
	// TODO: Add test registry and fixture with stale lock file
	// Entry("given the path of a package with images, but lock file is stale",
	// 	TestCase{
	// 		Args:                []string{filepath.Join("testdata", "")},
	// 		ExpectedExitCode:    1,
	// 		ExpectedErrorOutput: []string{""},
	// 	},
	// ),
	// TODO: Add test registry and fixture with lock file containing bad digests
	// Entry("given the path of a package with images, but lock file has invalid image ref(s)",
	// 	TestCase{
	// 		Args:                []string{filepath.Join("testdata", "")},
	// 		ExpectedExitCode:    1,
	// 		ExpectedErrorOutput: []string{""},
	// 	},
	// ),
	ginkgo.Entry("given the path of a package with images, but no lock file",
		subCommandTestCase{
			Args:                []string{sourcePathFixture("invalid_missing_lock_file")},
			ExpectedExitCode:    1,
			ExpectedErrorOutput: []string{"manifest.lock.yaml is missing"},
		},
	),
	ginkgo.Entry("given '--output' without tags",
		subCommandTestCase{
			Args: []string{
				"--output", filepath.Join("dne", "dne"),
				sourcePathFixture("valid_without_config"),
			},
			ExpectedExitCode:    1,
			ExpectedErrorOutput: []string{"output or push is requested but no tags are set"},
		},
	),
	ginkgo.Entry("given '--output' with an invalid path",
		subCommandTestCase{
			Args: []string{
				"--output", filepath.Join("dne", "dne"),
				"--tag", "test",
				sourcePathFixture("valid_without_config"),
			},
			ExpectedExitCode:    1,
			ExpectedErrorOutput: []string{"no such file or directory"},
		},
	),
	ginkgo.Entry("given '--output' with an invalid tag",
		subCommandTestCase{
			Args: []string{
				"--output", filepath.Join("dne", "dne"),
				"--tag", "****",
				sourcePathFixture("valid_without_config"),
			},
			ExpectedExitCode:    1,
			ExpectedErrorOutput: []string{"invalid tag specified as parameter"},
		},
	),
	ginkgo.Entry("given '--push' with no tags",
		subCommandTestCase{
			Args: []string{
				"--push",
				sourcePathFixture("valid_without_config"),
			},
			ExpectedExitCode:    1,
			ExpectedErrorOutput: []string{"output or push is requested but no tags are set"},
		},
	),
	ginkgo.Entry("given '--output' with valid path",
		subCommandTestCase{
			Args: []string{
				sourcePathFixture("valid_without_config"),
				"--output", filepath.Join(outputPath, "valid_build.tar"),
				"--tag", "valid-build",
			},
			ExpectedExitCode: 0,
			AdditionalValidations: func() {
				gomega.Expect(filepath.Join(outputPath, "valid_build.tar")).To(gomega.BeAnExistingFile())
			},
		},
	),
	ginkgo.Entry("given '--push' with valid tag",
		subCommandTestCase{
			Args: []string{
				"--push",
				"--insecure",
				"--tag", path.Join(registryPlaceholder, "valid-package"),
				sourcePathFixture("valid_without_config"),
			},
			ExpectedExitCode: 0,
			AdditionalValidations: func() {
				gomega.Expect(path.Join(_registryDomain, "valid-package")).To(ExistOnRegistry())
			},
		},
	),
)
