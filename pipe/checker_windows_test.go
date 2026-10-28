// (c) Siemens AG 2023
//
// SPDX-License-Identifier: MIT

//go:build windows

package pipe

import (
	"log/slog"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("pipe breakage", func() {

	BeforeEach(func() {
		DeferCleanup(slog.SetDefault, slog.Default())
		slog.SetDefault(slog.New(slog.NewTextHandler(GinkgoWriter, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		})))
	})

	It("waits until the reading end goes away", func() {
		r, w, err := os.Pipe()
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(w.Close)
		// ensure the monitoring goroutine terminates even if a spec fails.
		DeferCleanup(func() { _ = r.Close() })

		done := make(chan struct{})
		go func() {
			defer GinkgoRecover()
			defer close(done)
			WaitTillBreak(w)
		}()

		// WaitTillBreak probes only once per second, so cover at least one probe.
		Consistently(done).WithTimeout(1500*time.Millisecond).
			ShouldNot(BeClosed(), "returned while pipe still intact")
		Expect(r.Close()).To(Succeed())
		Eventually(done).WithTimeout(3*time.Second).
			Should(BeClosed(), "did not notice broken pipe")
	})

})
