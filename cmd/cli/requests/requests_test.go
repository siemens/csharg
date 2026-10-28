// (c) Siemens AG 2026
//
// SPDX-License-Identifier: MIT

package requests

import (
	"time"

	"github.com/spf13/cobra"
	"github.com/thediveo/clippy/cliplugin"
	"github.com/thediveo/go-plugger/v3"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("request timeout", func() {

	It("registers as a CLI setup plugin", func() {
		Expect(plugger.Group[cliplugin.SetupCLI]().Plugins()).To(ContainElement("req"))
	})

	It("returns zero without the flag", func() {
		Expect(GetTimeout(&cobra.Command{})).To(BeZero())
	})

	DescribeTable("getting the timeout in a child command",
		func(args []string, expected time.Duration) {
			var timeout time.Duration
			root := &cobra.Command{Use: "root"}
			root.SetOut(GinkgoWriter)
			root.SetErr(GinkgoWriter)
			reqSetupCLI(root)
			root.AddCommand(&cobra.Command{
				Use: "child",
				Run: func(cmd *cobra.Command, _ []string) {
					timeout = GetTimeout(cmd)
				},
			})
			root.SetArgs(append([]string{"child"}, args...))
			Expect(root.Execute()).To(Succeed())
			Expect(timeout).To(Equal(expected))
		},
		Entry("default", nil, RequestTimeoutDefault),
		Entry("explicit", []string{"--" + RequestTimeoutFlag, "42s"}, 42*time.Second),
	)

	It("rejects an invalid timeout", func() {
		root := &cobra.Command{Use: "root", Run: func(*cobra.Command, []string) {}}
		root.SetOut(GinkgoWriter)
		root.SetErr(GinkgoWriter)
		reqSetupCLI(root)
		root.SetArgs([]string{"--" + RequestTimeoutFlag, "forever"})
		Expect(root.Execute()).To(MatchError(ContainSubstring("invalid argument")))
	})

})
