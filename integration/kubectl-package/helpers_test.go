//go:build integration

package kubectlpackage

import (
	"context"
	"os/exec"
	"strings"
	"time"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/types"
)

type subCommandTestCase struct {
	Args                  []string
	ExpectedExitCode      int
	ExpectedOutput        []string
	ExpectedErrorOutput   []string
	AdditionalValidations func()
	Timeout               time.Duration
}

func testSubCommand(subcommand string) func(tc subCommandTestCase) {
	return func(tc subCommandTestCase) {
		args := append([]string{subcommand}, substitutePlaceholders(tc.Args...)...)
		cmd := exec.CommandContext(context.Background(), _pluginPath, args...)

		if tc.Timeout == 0 {
			tc.Timeout = 1 * time.Second
		}

		session, err := gexec.Start(cmd, ginkgo.GinkgoWriter, ginkgo.GinkgoWriter)
		gomega.Expect(err).ToNot(gomega.HaveOccurred())
		gomega.Eventually(session, tc.Timeout.String()).Should(gexec.Exit(tc.ExpectedExitCode))

		for _, line := range tc.ExpectedOutput {
			gomega.Expect(session.Out).To(gbytes.Say(line))
		}
		for _, line := range tc.ExpectedErrorOutput {
			gomega.Expect(session.Err).To(gbytes.Say(line))
		}

		if tc.AdditionalValidations != nil {
			tc.AdditionalValidations()
		}
	}
}

const (
	registryPlaceholder = "TEST_REGISTRY"
	TempDirPlaceholder  = "TEMP_DIR"
)

func substitutePlaceholders(args ...string) []string {
	res := make([]string, 0, len(args))

	for _, arg := range args {
		if strings.HasPrefix(arg, registryPlaceholder+"/") {
			sfx := strings.TrimPrefix(arg, registryPlaceholder)

			arg = _registryDomain + sfx
		} else if strings.HasPrefix(arg, TempDirPlaceholder) {
			sfx := strings.TrimPrefix(arg, TempDirPlaceholder)

			arg = _tempDir + sfx
		}

		res = append(res, arg)
	}

	return res
}

func ExistOnRegistry() types.GomegaMatcher {
	return gomega.WithTransform(func(ref string) error {
		_, err := crane.Head(ref, crane.Insecure)

		return err
	}, gomega.Succeed())
}
