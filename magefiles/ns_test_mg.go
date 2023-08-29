//go:build mage

package main

// This file can't be named ns_test.go because go then thinks this is test code.

import (
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"
	"runtime"
	"strings"

	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
	"golang.org/x/mod/semver"
)

type Test mg.Namespace

// Runs linters.
func (Test) FixLint() { mg.SerialDeps(Test.GolangCILintFix, Test.GoModTidy) }
func (Test) Lint()    { mg.SerialDeps(Test.GolangCILint) }

func (Test) GolangCILint() {
	// Generate.All ensures code generators are re-triggered.
	mg.Deps(Generate.All, Dependency.GolangciLint)
	must(sh.RunV("golangci-lint", "run", "./...", "--deadline=15m"))
}

func (Test) GolangCILintFix() {
	// Generate.All ensures code generators are re-triggered.
	mg.Deps(Generate.All, Dependency.GolangciLint)
	must(sh.RunV("golangci-lint", "run", "./...", "--deadline=15m", "--fix"))
}

func (Test) GoModTidy() {
	// Generate.All ensures code generators are re-triggered.
	mg.Deps(Generate.All)
	must(sh.RunV("go", "mod", "tidy"))
}

func (Test) ValidateGitClean() {
	// Generate.All ensures code generators are re-triggered.
	mg.Deps(Generate.All)

	o, err := sh.Output("git", "status", "--porcelain")
	must(err)

	if len(o) != 0 {
		panic("Repo is dirty! Probably because gofmt or make generate touched something...")
	}
}

// Runs unittests.
func (Test) Unit() {
	testCmd := fmt.Sprintf("set -o pipefail; go test -coverprofile=%s -race -test.v", locations.UnitTestCoverageReport())
	testCmd += " ./internal/... ./cmd/... ./apis/... "
	testCmd += "| tee " + locations.UnitTestStdOut()

	// cgo needed to enable race detector -race
	testErr := sh.RunWithV(map[string]string{"CGO_ENABLED": "1"}, "bash", "-c", testCmd)
	must(sh.RunV("bash", "-c", "set -o pipefail; cat "+locations.UnitTestStdOut()+" | go tool test2json > "+locations.UnitTestExecReport()))
	must(testErr)
}

// Runs the given integration suite(s) as given by the first
// positional argument. The options are 'all', 'all-local',
// 'kubectl-package', 'package-operator', and
// 'package-operator-local'.
func (t Test) Integration(ctx context.Context, suite string) {
	var testFns []any

	switch strings.ToLower(strings.TrimSpace(suite)) {
	case "all":
		testFns = append(testFns,
			mg.F(t.packageOperatorIntegration, ""),
			t.kubectlPackageIntegration,
		)
	case "all-local":
		testFns = append(testFns,
			Dev.Integration,
			t.kubectlPackageIntegration,
		)
	case "kubectl-package":
		testFns = append(testFns,
			t.kubectlPackageIntegration,
		)
	case "package-operator":
		testFns = append(testFns,
			mg.F(t.packageOperatorIntegration, ""),
		)
	case "package-operator-local":
		testFns = append(testFns,
			Dev.Integration,
		)
	default:
		panic(fmt.Sprintf("unknown test suite: %s", suite))
	}

	mg.CtxDeps(
		ctx,
		testFns...,
	)
}

// Runs PKO integration tests against whatever cluster your KUBECONFIG is pointing at.
// Also allows specifying only sub tests to run e.g. ./mage test:integrationrun TestPackage_success
func (t Test) PackageOperatorIntegrationRun(ctx context.Context, filter string) {
	t.packageOperatorIntegration(ctx, filter)
}

func (Test) packageOperatorIntegration(ctx context.Context, filter string) {
	os.Setenv("PKO_TEST_SUCCESS_PACKAGE_IMAGE", locations.ImageURL("test-stub-package", false))
	os.Setenv("PKO_TEST_STUB_IMAGE", locations.ImageURL("test-stub", false))
	if len(os.Getenv("PKO_TEST_LATEST_BOOTSTRAP_JOB")) == 0 {
		os.Setenv("PKO_TEST_LATEST_BOOTSTRAP_JOB", defaultPKOLatestBootstrapJob)
	}

	// count=1 will force a new run, instead of using the cache
	args := []string{
		"test", "-v",
		"-failfast", "-count=1", "-timeout=20m",
		"-coverpkg=./apis/...,./cmd/...,./internal/...",
		fmt.Sprintf("-coverprofile=%s", locations.PKOIntegrationTestCoverageReport()),
	}
	if len(filter) > 0 {
		args = append(args, "-run", filter)
	}
	args = append(args, "./integration/package-operator/...")

	testErr := sh.Run("go", args...)

	devEnv := locations.DevEnvNoInit()

	// always export logs
	if devEnv != nil {
		args := []string{"export", "logs", locations.IntegrationTestLogs(), "--name", clusterName}
		if err := devEnv.RunKindCommand(ctx, os.Stdout, os.Stderr, args...); err != nil {
			logger.Error(err, "exporting logs")
		}
	}

	if testErr != nil {
		panic(testErr)
	}
}

func (Test) kubectlPackageIntegration() {
	tmp, err := os.MkdirTemp("", "kubectl-package-integration-cov-*")
	if err != nil {
		panic(err)
	}

	defer os.RemoveAll(tmp)

	env := map[string]string{
		"GOCOVERDIR": tmp,
	}

	args := []string{
		"test", "-v", "-failfast",
		"-count=1", "-timeout=5m",
		"./integration/kubectl-package/...",
	}
	_, isCI := os.LookupEnv("CI")
	if isCI {
		// test output in json format
		args = append(args, "-json", " > "+locations.PluginIntegrationTestExecReport())
	}

	if err := sh.RunWith(env, "go", args...); err != nil {
		panic(err)
	}

	goVersion, err := getGoVersion()
	must(err)

	if semver.Compare("v"+goVersion, "v"+coverProfilingMinGoVersion) >= 0 {
		covArgs := []string{
			"tool", "covdata", "textfmt",
			"-i", tmp,
			"-o", locations.PluginIntegrationTestCoverageReport(),
		}
		if err := sh.Run("go", covArgs...); err != nil {
			panic(err)
		}
	}
}

var errRegexpMatchNotFound = errors.New("no match found for regexp")

func getGoVersion() (string, error) {
	goVersion := runtime.Version()
	r := regexp.MustCompile(`\d(?:\.\d+){2}`)
	parsedVersion := r.FindString(goVersion)
	if parsedVersion == "" {
		return parsedVersion, errRegexpMatchNotFound
	}
	return parsedVersion, nil
}
