// (c) Siemens AG 2023
//
// SPDX-License-Identifier: MIT

//go:build !windows

package pipe

import (
	"fmt"
	"log/slog"
	"time"

	"golang.org/x/sys/unix"

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

	DescribeTable("waits until the pipe breaks",
		func(minfd int) {
			r, w := pipeWithReadFdAtLeast(minfd)
			DeferCleanup(r.Close)
			// ensure the monitoring goroutine terminates even if a spec fails.
			DeferCleanup(func() { _ = w.Close() })

			done := make(chan struct{})
			go func() {
				defer GinkgoRecover()
				defer close(done)
				WaitTillBreak(r)
			}()

			Consistently(done).WithTimeout(250*time.Millisecond).
				ShouldNot(BeClosed(), "fd %d: returned while pipe still intact", r.Fd())
			Expect(w.Close()).To(Succeed())
			Eventually(done).WithTimeout(2*time.Second).
				Should(BeClosed(), "fd %d: did not notice broken pipe", r.Fd())
		},
		Entry("low fd", 0),
		Entry("fd beyond first byte", 8),
		Entry("fd beyond first 32 bit word", 32),
		Entry("fd beyond first 64 bit word", 64),
		Entry("high fd", 200),
	)

	When("the fd doesn't fit into select()'s fd set", func() {

		BeforeEach(func() {
			var lim unix.Rlimit
			Expect(unix.Getrlimit(unix.RLIMIT_NOFILE, &lim)).To(Succeed())
			// Rlimit.Cur is int64 on some BSDs, uint64 elsewhere.
			if uint64(lim.Cur) <= uint64(fdSetSize) {
				Skip(fmt.Sprintf("RLIMIT_NOFILE %d too low for fds >= FD_SETSIZE", lim.Cur))
			}
		})

		It("still accepts the highest fd within FD_SETSIZE", func() {
			r, w := pipeWithReadFdAtLeast(fdSetSize - 1)
			DeferCleanup(r.Close)
			Expect(int(r.Fd())).To(Equal(fdSetSize - 1))
			Expect(w.Close()).To(Succeed())
			Expect(func() { WaitTillBreak(r) }).NotTo(Panic())
		})

		It("panics with a useful description", func() {
			r, w := pipeWithReadFdAtLeast(fdSetSize)
			DeferCleanup(r.Close)
			DeferCleanup(w.Close)
			Expect(func() { WaitTillBreak(r) }).To(PanicWith(
				MatchRegexp(`fd \d+ outside select\(\) range 0\.\.%d \(FD_SETSIZE\)`, fdSetSize-1)))
		})

	})

	It("panics on a closed fifo", func() {
		r, w := pipeWithReadFdAtLeast(0)
		Expect(r.Close()).To(Succeed())
		DeferCleanup(w.Close)
		Expect(func() { WaitTillBreak(r) }).To(PanicWith(
			ContainSubstring("fd -1 outside select() range")))
	})

})
