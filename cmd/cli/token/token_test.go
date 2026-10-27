// (c) Siemens AG 2026
//
// SPDX-License-Identifier: MIT

package token

import (
	"github.com/spf13/cobra"
	"github.com/thediveo/clippy/cliplugin"
	"github.com/thediveo/go-plugger/v3"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("bearer token", func() {

	It("registers as a CLI setup plugin", func() {
		Expect(plugger.Group[cliplugin.SetupCLI]().Plugins()).To(ContainElement("token"))
	})

	It("returns an empty token without the flag", func() {
		Expect(Get(&cobra.Command{})).To(BeEmpty())
	})

	DescribeTable("getting the token in a child command",
		func(args []string, expected string) {
			var token string
			root := &cobra.Command{Use: "root"}
			root.SetOut(GinkgoWriter)
			root.SetErr(GinkgoWriter)
			tokenSetupCLI(root)
			root.AddCommand(&cobra.Command{
				Use: "child",
				Run: func(cmd *cobra.Command, _ []string) {
					token = Get(cmd)
				},
			})
			root.SetArgs(append([]string{"child"}, args...))
			Expect(root.Execute()).To(Succeed())
			Expect(token).To(Equal(expected))
		},
		Entry("default", nil, ""),
		Entry("explicit", []string{"--" + TokenFlag, "s3cr3t"}, "s3cr3t"),
	)

})
