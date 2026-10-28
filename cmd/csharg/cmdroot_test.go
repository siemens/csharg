// (c) Siemens AG 2026
//
// SPDX-License-Identifier: MIT

package main

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"

	"github.com/thediveo/clippy/debug"
	"github.com/thediveo/safe"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("csharg commands", func() {

	It("keeps schtumm", func() {
		rootCmd := newRootCmd()
		rootCmd.SetArgs([]string{"--silent"})
		rootCmd.SetOut(GinkgoWriter)
		rootCmd.SetErr(GinkgoWriter)
		Expect(rootCmd.Execute()).To(Succeed())
	})

	It("shows help with available commands", func() {
		var out safe.Buffer
		rootCmd := newRootCmd()
		rootCmd.SetArgs([]string{"help"})
		rootCmd.SetOut(io.MultiWriter(&out, GinkgoWriter))
		Expect(rootCmd.Execute()).To(Succeed())
		Eventually(out.String).To(MatchRegexp(
			`(?s)csharg is a .*\n+Usage:\n.*\n+Available Commands:\n  capture.*\n  completion.*\n  help.*\n  list.*\n  options.*\n  version.*\n`))
	})

	It("shows version", func() {
		var out safe.Buffer
		rootCmd := newRootCmd()
		rootCmd.SetArgs([]string{"--silent", "version"})
		rootCmd.SetOut(io.MultiWriter(&out, GinkgoWriter))
		Expect(rootCmd.Execute()).To(Succeed())
		Expect(out.String()).To(MatchRegexp(
			`^csharg \S+\ncapture service clients: .*host.*\n$`))
	})

	It("prints version only once when not silent", func() {
		DeferCleanup(slog.SetDefault, slog.Default())
		var out, log safe.Buffer
		rootCmd := newRootCmd()
		debug.SetWriter(rootCmd, io.MultiWriter(&log, GinkgoWriter))
		rootCmd.SetArgs([]string{"version"})
		rootCmd.SetOut(io.MultiWriter(&out, GinkgoWriter))
		Expect(rootCmd.Execute()).To(Succeed())
		Expect(out.String()).To(MatchRegexp(
			`^csharg \S+\ncapture service clients: .*host.*\n$`))
		Expect(log.String()).NotTo(ContainSubstring("capture-service-clients"))
	})

	It("rejects an invalid list --sort-by", func() {
		rootCmd := newRootCmd()
		rootCmd.SetArgs([]string{"--silent", "list", "--sort-by", "{.["})
		rootCmd.SetOut(GinkgoWriter)
		rootCmd.SetErr(GinkgoWriter)
		Expect(rootCmd.Execute()).To(MatchError(ContainSubstring("invalid --sort-by")))
	})

	It("reports a missing capture service client without mentioning --context", func() {
		rootCmd := newRootCmd()
		rootCmd.SetArgs([]string{"--silent", "list"})
		rootCmd.SetOut(GinkgoWriter)
		rootCmd.SetErr(GinkgoWriter)
		err := rootCmd.Execute()
		Expect(err).To(MatchError(ContainSubstring("no suitable capture API client found")))
		Expect(err).NotTo(MatchError(ContainSubstring("--context")))
	})

	It("fails listing from an unreachable host", func() {
		srv := httptest.NewServer(http.NotFoundHandler())
		srv.Close()
		rootCmd := newRootCmd()
		rootCmd.SetArgs([]string{"--silent", "--host", srv.URL, "list"})
		rootCmd.SetOut(GinkgoWriter)
		rootCmd.SetErr(GinkgoWriter)
		Expect(rootCmd.Execute()).To(MatchError(ContainSubstring("connection refused")))
	})

})
