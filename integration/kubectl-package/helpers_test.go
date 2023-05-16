/* #nosec */

package kubectlpackage

import (
	"os/exec"
	"strings"
	"time"

	"github.com/google/go-containerregistry/pkg/crane"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gbytes"
	. "github.com/onsi/gomega/gexec"
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
		cmd := exec.Command(_pluginPath, args...)

		if tc.Timeout == 0 {
			tc.Timeout = 1 * time.Second
		}

		session, err := Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).ToNot(HaveOccurred())
		Eventually(session, tc.Timeout.String()).Should(Exit(tc.ExpectedExitCode))

		for _, line := range tc.ExpectedOutput {
			Expect(session.Out).To(Say(line))
		}
		for _, line := range tc.ExpectedErrorOutput {
			Expect(session.Err).To(Say(line))
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
	return WithTransform(func(ref string) error {
		_, err := crane.Head(ref, crane.Insecure)

		return err
	}, Succeed())
}
