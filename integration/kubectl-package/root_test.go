/* #nosec */

package kubectlpackage

import (
	"os/exec"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gbytes"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("root command", func() {
	When("given no arguments", func() {
		It("should display 'usage'", func() {
			pluginCommand := exec.Command(_pluginPath)

			session, err := Start(pluginCommand, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())
			Eventually(session).Should(Exit(0))

			Expect(session.Out).To(Say("Usage:"))
		})
	})
})
