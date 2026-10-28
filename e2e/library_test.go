// (c) Siemens AG 2026
//
// SPDX-License-Identifier: MIT

package e2e

import (
	"bytes"
	"context"
	"errors"
	"os"
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/thediveo/success"
)

// brokenWriter fails all writes.
type brokenWriter struct{}

var errBrokenWriter = errors.New("broken writer")

func (brokenWriter) Write([]byte) (int, error) { return 0, errBrokenWriter }

var _ = Describe("capturing with the csharg package", func() {

	It("discovers capture targets", func(ctx context.Context) {
		const name = "csharg-e2e-lib-discover"
		newTarget(ctx, name, pingScript)
		Expect(newSharkTank().Targets()).To(ContainElement(And(
			HaveField("Name", name),
			HaveField("Type", "docker"),
			HaveField("NodeName", nodeName),
			HaveField("NetworkInterfaces", ConsistOf("lo", "eth0")))))
	})

	It("captures until stopped after a duration", func(ctx context.Context) {
		const name = "csharg-e2e-lib-stopafter"
		newTarget(ctx, name, pingScript)
		out := tempPath("capture.pcapng")
		f := Successful(os.Create(out))
		capt := Successful(newSharkTank().CaptureContainer(f, nodeName, name, nil))
		Expect(capt.StopAfter(3 * time.Second)).To(Succeed())
		Expect(f.Close()).To(Succeed())
		Expect(capinfos(out)).To(ContainSubstring("container-name: " + name))
		Expect(fields(out, "icmp", "frame.number")).NotTo(BeEmpty())
	})

	It("runs concurrent captures using the same SharkTank", func(ctx context.Context) {
		names := []string{"csharg-e2e-lib-concurrent-1", "csharg-e2e-lib-concurrent-2"}
		for _, name := range names {
			newTarget(ctx, name, pingScript)
		}
		st := newSharkTank()
		outs := make([]string, len(names))
		errs := make([]error, len(names))
		var wg sync.WaitGroup
		for idx, name := range names {
			outs[idx] = tempPath("capture.pcapng")
			f := Successful(os.Create(outs[idx]))
			capt := Successful(st.CaptureContainer(f, nodeName, name, nil))
			wg.Go(func() {
				errs[idx] = errors.Join(capt.StopAfter(3*time.Second), f.Close())
			})
		}
		wg.Wait()
		Expect(errs).To(HaveEach(BeNil()))
		for idx, name := range names {
			Expect(capinfos(outs[idx])).To(ContainSubstring("container-name: " + name))
			Expect(fields(outs[idx], "icmp", "frame.number")).NotTo(BeEmpty())
		}
	})

	It("reports a normal end when the target container terminates", func(ctx context.Context) {
		const name = "csharg-e2e-lib-wait"
		newTarget(ctx, name, "sleep 5; ping -c 3 -i 0.2 127.0.0.1")
		var buf bytes.Buffer
		capt := Successful(newSharkTank().CaptureContainer(&buf, nodeName, name, nil))
		done := make(chan error, 1)
		go func() { done <- capt.Wait() }()
		Eventually(done).WithTimeout(30 * time.Second).Should(Receive(BeNil()))
		Expect(buf.Len()).To(BeNumerically(">", 0))
	})

	It("reports a broken writer", func(ctx context.Context) {
		const name = "csharg-e2e-lib-brokenwriter"
		newTarget(ctx, name, pingScript)
		capt := Successful(newSharkTank().CaptureContainer(brokenWriter{}, nodeName, name, nil))
		done := make(chan error, 1)
		go func() { done <- capt.Wait() }()
		Eventually(done).WithTimeout(30 * time.Second).Should(Receive(MatchError(errBrokenWriter)))
	})

})
