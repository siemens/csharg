// (c) Siemens AG 2026
//
// SPDX-License-Identifier: MIT

package main

import (
	"io"

	"github.com/thediveo/safe"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("csharg commands", func() {

	It("keeps schtumm", func() {
		rootCmd := newRootCmd()
		rootCmd.SetArgs([]string{"--silent"})
		Expect(rootCmd.Execute()).To(Succeed())
	})

	FIt("shows help with available commands", func() {
		var out safe.Buffer
		rootCmd := newRootCmd()
		rootCmd.SetArgs([]string{"help"})
		rootCmd.SetOut(io.MultiWriter(&out, GinkgoWriter))
		Expect(rootCmd.Execute()).To(Succeed())
		Eventually(out.String).To(MatchRegexp(
			`(?s)csharg is a .*\n+Usage:\n.*\n+Available Commands:\n  completion.*\n  help.*\n  list.*\n  options.*\n  version.*\n`))
	})

})
