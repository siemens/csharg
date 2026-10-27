// (c) Siemens AG 2023
//
// SPDX-License-Identifier: MIT

package pipe

import (
	"runtime"
	"time"

	"golang.org/x/sys/unix"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("pipe breakage on Linux", func() {

	It("keeps waiting when interrupted by signals", func() {
		r, w := pipeWithReadFdAtLeast(0)
		DeferCleanup(r.Close)
		DeferCleanup(func() { _ = w.Close() })

		tids := make(chan int, 1)
		done := make(chan struct{})
		go func() {
			defer GinkgoRecover()
			defer close(done)
			// Pin to a single thread so we can signal exactly the thread
			// blocking in select().
			runtime.LockOSThread()
			defer runtime.UnlockOSThread()
			tids <- unix.Gettid()
			WaitTillBreak(r)
		}()
		tid := <-tids

		stop := make(chan struct{})
		signaled := make(chan struct{})
		go func() {
			defer close(signaled)
			ticker := time.NewTicker(10 * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-stop:
					return
				case <-ticker.C:
					// SIGURG is harmless as the Go runtime already handles it.
					_ = unix.Tgkill(unix.Getpid(), tid, unix.SIGURG)
				}
			}
		}()

		Consistently(done).WithTimeout(250*time.Millisecond).
			ShouldNot(BeClosed(), "returned on signal while pipe still intact")
		close(stop)
		<-signaled
		Expect(w.Close()).To(Succeed())
		Eventually(done).WithTimeout(2*time.Second).
			Should(BeClosed(), "did not notice broken pipe")
	})

})
