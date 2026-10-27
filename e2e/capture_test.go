// (c) Siemens AG 2026
//
// SPDX-License-Identifier: MIT

package e2e

import (
	"context"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/onsi/gomega/gexec"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/thediveo/success"
)

var _ = Describe("capturing with the CLI", func() {

	DescribeTable("writes a complete capture file when stopped by a signal",
		func(ctx context.Context, name string, sig os.Signal) {
			newTarget(ctx, name, pingScript)
			out := tempPath("capture.pcapng")
			cli := runCsharg("capture", "container", name, "-w", out)
			awaitPackets(ctx, out)
			cli.Signal(sig)
			Eventually(cli).WithTimeout(10 * time.Second).Should(gexec.Exit(0))
			Expect(capinfos(out)).To(And(
				ContainSubstring("container-name: "+name),
				ContainSubstring("node-name: "+nodeName)))
			Expect(fields(out, "icmp", "frame.number")).NotTo(BeEmpty())
		},
		Entry("SIGINT", "csharg-e2e-sigint", os.Interrupt),
		Entry("SIGTERM", "csharg-e2e-sigterm", syscall.SIGTERM),
	)

	DescribeTable("applies a capture filter",
		func(ctx context.Context, name, filter string, wantUDP bool) {
			newTarget(ctx, name, chattyScript)
			out := tempPath("capture.pcapng")
			// Packetflix's dumpcap applies a filter only to the last network interface.
			args := []string{"capture", "container", name, "-i", "lo", "-w", out}
			if filter != "" {
				args = append(args, "-f", filter)
			}
			cli := runCsharg(args...)
			awaitPackets(ctx, out)
			cli.Interrupt()
			Eventually(cli).WithTimeout(10 * time.Second).Should(gexec.Exit(0))
			Expect(fields(out, "icmp", "frame.number")).NotTo(BeEmpty())
			// ICMP port unreachable errors quote UDP, so they don't count as UDP.
			udp := fields(out, "udp && !icmp", "frame.number")
			if wantUDP {
				Expect(udp).NotTo(BeEmpty())
			} else {
				Expect(udp).To(BeEmpty())
			}
		},
		Entry("without a filter", "csharg-e2e-nofilter", "", true),
		Entry("with an ICMP-only filter", "csharg-e2e-icmpfilter", "icmp", false),
	)

	DescribeTable("captures from the selected network interfaces",
		func(ctx context.Context, name string, nifs ...string) {
			newTarget(ctx, name, chattyScript)
			out := tempPath("capture.pcapng")
			args := []string{"capture", "container", name, "-w", out}
			for _, nif := range nifs {
				args = append(args, "-i", nif)
			}
			cli := runCsharg(args...)
			awaitPackets(ctx, out)
			cli.Interrupt()
			Eventually(cli).WithTimeout(10 * time.Second).Should(gexec.Exit(0))
			if len(nifs) == 0 {
				nifs = []string{"lo", "eth0"}
			}
			Expect(fields(out, "", "frame.interface_name")).To(And(
				HaveEach(BeElementOf(nifs)),
				ContainElements(nifs)))
		},
		Entry("all by default", "csharg-e2e-nif-all"),
		Entry("only loopback", "csharg-e2e-nif-lo", "lo"),
		Entry("only ethernet", "csharg-e2e-nif-eth0", "eth0"),
	)

	It("ends by itself when the target container terminates", func(ctx context.Context) {
		const name = "csharg-e2e-shortlived"
		newTarget(ctx, name, "sleep 5; ping -c 3 -i 0.2 127.0.0.1")
		out := tempPath("capture.pcapng")
		cli := runCsharg("capture", "container", name, "-w", out)
		Eventually(cli).WithTimeout(30 * time.Second).Should(gexec.Exit(0))
		Expect(fields(out, "icmp", "frame.number")).NotTo(BeEmpty())
	})

	It("ends cleanly when the packet consumer closes its pipe", func(ctx context.Context) {
		const name = "csharg-e2e-closedpipe"
		newTarget(ctx, name, pingScript)
		r, w := Successful2R(os.Pipe())
		done := startCsharg(w, "capture", "container", name)
		Expect(w.Close()).To(Succeed())
		Expect(io.ReadFull(r, make([]byte, 2048))).Error().NotTo(HaveOccurred())
		Expect(r.Close()).To(Succeed())
		Eventually(done).WithTimeout(10 * time.Second).Should(Receive(BeNil()))
	})

	It("streams live into tshark", func(ctx context.Context) {
		const name = "csharg-e2e-tshark"
		newTarget(ctx, name, pingScript)
		r, w := Successful2R(os.Pipe())
		done := startCsharg(w, "capture", "container", name)
		Expect(w.Close()).To(Succeed())
		tshark := exec.Command("tshark", "-i", "-", "-c", "5", "-T", "fields", "-e", "frame.number")
		tshark.Stdin = r
		tsharkSess := Successful(gexec.Start(tshark, GinkgoWriter, GinkgoWriter))
		DeferCleanup(func() { tsharkSess.Kill().Wait() })
		Expect(r.Close()).To(Succeed())
		Eventually(tsharkSess).WithTimeout(30 * time.Second).Should(gexec.Exit(0))
		Expect(strings.Fields(string(tsharkSess.Out.Contents()))).To(Equal([]string{"1", "2", "3", "4", "5"}))
		Eventually(done).WithTimeout(10 * time.Second).Should(Receive(BeNil()))
	})

	It("reports an unknown capture target", func() {
		cli := runCsharg("capture", "container", "csharg-e2e-nonexisting", "-w", tempPath("capture.pcapng"))
		Eventually(cli).WithTimeout(30 * time.Second).Should(gexec.Exit(1))
		Expect(cli.Err.Contents()).To(ContainSubstring(`capture target "csharg-e2e-nonexisting" not found`))
	})

})
