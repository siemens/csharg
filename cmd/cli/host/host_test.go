// (c) Siemens AG 2026
//
// SPDX-License-Identifier: MIT

package host

import (
	"github.com/spf13/cobra"
	"github.com/thediveo/clippy/cliplugin"
	"github.com/thediveo/go-plugger/v3"

	"github.com/siemens/csharg/v2"
	"github.com/siemens/csharg/v2/cmd/cli/client"
	"github.com/siemens/csharg/v2/cmd/cli/examples"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// newClientFromArgs runs a fresh root command with a child command, where the
// child command then tries to create a host client.
func newClientFromArgs(args ...string) (st csharg.SharkTank, err error) {
	root := &cobra.Command{Use: "root"}
	root.SetOut(GinkgoWriter)
	root.SetErr(GinkgoWriter)
	setupCLI(root)
	root.AddCommand(&cobra.Command{
		Use: "child",
		RunE: func(cmd *cobra.Command, _ []string) error {
			st, err = newClient(cmd)
			return nil
		},
	})
	root.SetArgs(append([]string{"child"}, args...))
	Expect(root.Execute()).To(Succeed())
	return
}

var _ = Describe("standalone host client", func() {

	It("registers its plugins", func() {
		Expect(plugger.Group[cliplugin.SetupCLI]().Plugins()).To(ContainElement("host"))
		Expect(plugger.Group[client.New]().Plugins()).To(ContainElement("host"))
		Expect(plugger.Group[examples.Illustrate]().Plugins()).To(ContainElement("host"))
	})

	It("provides examples", func() {
		Expect(examples.For("list")).To(ContainSubstring("--host"))
		Expect(examples.For("capture")).To(ContainSubstring("--host"))
	})

	It("adds its flags", func() {
		cmd := &cobra.Command{Use: "root"}
		setupCLI(cmd)
		pf := cmd.PersistentFlags()

		hostFlag := pf.Lookup(StandaloneHostFlag)
		Expect(hostFlag).NotTo(BeNil())
		Expect(hostFlag.Annotations).To(ContainElement(
			ConsistOf(client.MutuallyExclusiveGroup)))

		insecureFlag := pf.Lookup(InsecureFlag)
		Expect(insecureFlag).NotTo(BeNil())
		Expect(insecureFlag.Shorthand).To(Equal(InsecureShorthandFlag))
	})

	It("isn't responsible without --host", func() {
		st, err := newClientFromArgs()
		Expect(err).NotTo(HaveOccurred())
		Expect(st).To(BeNil())
	})

	It("returns a client for --host", func() {
		st, err := newClientFromArgs("--host", "localhost:5001", "-k")
		Expect(err).NotTo(HaveOccurred())
		Expect(st).NotTo(BeNil())
	})

	It("rejects an invalid --host", func() {
		st, err := newClientFromArgs("--host", "localhost:5001?foo=bar")
		Expect(err).To(MatchError(ContainSubstring("invalid --host")))
		Expect(st).To(BeNil())
	})

})
