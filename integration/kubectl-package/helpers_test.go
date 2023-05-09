/* #nosec */

package kubectlpackage

import (
	"os/exec"
	"strings"

	"github.com/google/go-containerregistry/pkg/crane"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gbytes"
	. "github.com/onsi/gomega/gexec"
)

type subCommandTestCase struct {
	Args                  []string
	ExpectedExitCode      int
	ExpectedOutput        []string
	ExpectedErrorOutput   []string
	AdditionalValidations func()
}

func testSubCommand(subcommand string) func(tc subCommandTestCase) {
	return func(tc subCommandTestCase) {
		args := append([]string{subcommand}, substitutePlaceholders(tc.Args...)...)
		cmd := exec.Command(_pluginPath, args...)

		session, err := Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).ToNot(HaveOccurred())
		Eventually(session).Should(Exit(tc.ExpectedExitCode))

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

const registryPlaceholder = "TEST_REGISTRY"

func substitutePlaceholders(args ...string) []string {
	res := make([]string, 0, len(args))

	for _, arg := range args {
		if strings.HasPrefix(arg, registryPlaceholder+"/") {
			sfx := strings.TrimPrefix(arg, registryPlaceholder)

			arg = _registryDomain + sfx
		}

		res = append(res, arg)
	}

	return res
}

func pushImageFromDisk(tarPath, imgRef string) {
	img, err := crane.Load(tarPath)
	ExpectWithOffset(1, err).ToNot(HaveOccurred())

	Expect(crane.Push(img, imgRef, crane.Insecure)).Error().ToNot(HaveOccurred())
}

func ExpectImageExists(ref string) {
	_, err := crane.Head(ref, crane.Insecure)
	ExpectWithOffset(1, err).ToNot(HaveOccurred())
}
