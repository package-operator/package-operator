//go:build integration

package kubectlpackage

import (
	"os/exec"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = ginkgo.Describe("root command", func() {
	ginkgo.When("given no arguments", func() {
		ginkgo.It("should display 'usage'", func() {
			pluginCommand := exec.Command(_pluginPath)

			session, err := gexec.Start(pluginCommand, ginkgo.GinkgoWriter, ginkgo.GinkgoWriter)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Eventually(session).Should(gexec.Exit(0))

			gomega.Expect(session.Out).To(gbytes.Say("Usage:"))
		})
	})
})
