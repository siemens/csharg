// (c) Siemens AG 2026
//
// SPDX-License-Identifier: MIT

package e2e

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/thediveo/morbyd"
	"github.com/thediveo/morbyd/run"
	"github.com/thediveo/morbyd/session"

	"github.com/siemens/csharg/v2"
	"github.com/siemens/csharg/v2/api"

	"github.com/onsi/gomega/gexec"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/thediveo/success"
)

func TestE2E(t *testing.T) {
	// Filters also matching unlabelled specs, such as none at all or "!foo", don't opt in.
	if f := GinkgoLabelFilter(); !Label("e2e").MatchesLabelFilter(f) || Label().MatchesLabelFilter(f) {
		t.Skip("end-to-end tests are opt-in, use -ginkgo.label-filter=e2e")
	}
	RegisterFailHandler(Fail)
	RunSpecs(t, "Csharg end-to-end suite", Label("e2e"))
}

const (
	ghostwireImage  = "ghcr.io/siemens/ghostwire:latest"
	packetflixImage = "ghcr.io/siemens/packetflix:latest"
	targetImage     = "busybox:latest"

	// csharg derives the node name of standalone host targets from the service URL.
	nodeName = "127.0.0.1"

	// pingScript generates ICMP traffic on the loopback interface.
	pingScript = "ping -i 0.2 127.0.0.1"
	// chattyScript generates ICMP traffic on lo and eth0, as well as UDP traffic on lo.
	chattyScript = `gw=$(ip route | awk '/default/ {print $3}')
while true; do
	ping -c 1 127.0.0.1
	ping -c 1 "$gw"
	echo hello | nc -u -w 1 127.0.0.1 4242
done`
)

var (
	sess        *morbyd.Session
	serviceAddr string // host:port of the Packetflix service
	cshargBin   string
)

var _ = BeforeSuite(func(ctx context.Context) {
	for _, tool := range []string{"tshark", "capinfos"} {
		Expect(exec.LookPath(tool)).Error().NotTo(HaveOccurred(),
			"end-to-end tests require %s", tool)
	}

	By("building the csharg CLI")
	cshargBin = Successful(gexec.Build("github.com/siemens/csharg/v2/cmd/csharg"))
	DeferCleanup(gexec.CleanupBuildArtifacts)

	By("deploying Ghostwire and Packetflix")
	sess = Successful(morbyd.NewSession(ctx, session.WithAutoCleaning("test.csharg.e2e=")))
	DeferCleanup(sess.Close)
	netw := Successful(sess.CreateNetwork(ctx, "csharg-e2e"))
	Expect(sess.Run(ctx, ghostwireImage, serviceOpts(
		run.WithName("csharg-e2e-ghostwire"),
		run.WithCommand("--http=[::]:5000"),
		run.WithNetwork("name="+netw.ID+",alias=gostwire"),
		run.WithCgroupnsMode("host"),
		run.WithCapAdd("DAC_READ_SEARCH"),
		run.WithCapAdd("DAC_OVERRIDE"),
	)...)).Error().NotTo(HaveOccurred())
	packetflix := Successful(sess.Run(ctx, packetflixImage, serviceOpts(
		run.WithName("csharg-e2e-packetflix"),
		run.WithCommand("--port=5001", "--discovery-service=gostwire", "--gw-port=5000", "--proxy-discovery"),
		run.WithNetwork(netw.ID),
		run.WithPublishedPort(nodeName+":5001"),
	)...))
	serviceAddr = packetflix.PublishedPort("5001").Any().String()
	Expect(serviceAddr).To(HavePrefix(nodeName + ":"))

	Eventually(func() (api.Targets, error) { return newSharkTank().Targets() }).
		WithContext(ctx).Within(60 * time.Second).ProbeEvery(time.Second).
		ShouldNot(BeEmpty())
}, NodeTimeout(5*time.Minute))

// serviceOpts returns the privileges Ghostwire and Packetflix need for
// discovering and capturing, followed by the specified opts.
func serviceOpts(opts ...run.Opt) []run.Opt {
	return append([]run.Opt{
		run.WithPIDMode("host"),
		run.WithSecurityOpt("apparmor=unconfined"),
		run.WithUser(65534),
		run.WithReadOnlyRootfs(),
		run.WithCapDropAll(),
		run.WithCapAdd("SYS_ADMIN"),
		run.WithCapAdd("SYS_CHROOT"),
		run.WithCapAdd("SYS_PTRACE"),
		run.WithCapAdd("NET_ADMIN"),
		run.WithCapAdd("NET_RAW"),
	}, opts...)
}

// newSharkTank returns a new client with an empty target cache for the
// Packetflix service.
func newSharkTank() csharg.SharkTank {
	GinkgoHelper()
	return Successful(csharg.NewSharkTankOnHost(serviceAddr, nil))
}

// newTarget runs a busybox container with the specified name executing script,
// and waits for the Packetflix service to discover it. The container gets
// removed automatically when it exits or, at the latest, when the test session
// closes.
func newTarget(ctx context.Context, name, script string) {
	GinkgoHelper()
	Expect(sess.Run(ctx, targetImage,
		run.WithName(name),
		run.WithCommand("/bin/sh", "-c", script),
		run.WithAutoRemove())).Error().NotTo(HaveOccurred())
	Eventually(func() (api.Targets, error) { return newSharkTank().Targets() }).
		WithContext(ctx).Within(30 * time.Second).ProbeEvery(500 * time.Millisecond).
		Should(ContainElement(HaveField("Name", name)))
}

// runCshargOn starts the csharg CLI against the capture service at host.
func runCshargOn(host string, args ...string) *gexec.Session {
	GinkgoHelper()
	cli := Successful(gexec.Start(
		exec.Command(cshargBin, append([]string{"--host", host}, args...)...),
		GinkgoWriter, GinkgoWriter))
	DeferCleanup(func() { cli.Kill().Wait() })
	return cli
}

// runCsharg starts the csharg CLI against the Packetflix service.
func runCsharg(args ...string) *gexec.Session {
	GinkgoHelper()
	return runCshargOn(serviceAddr, args...)
}

// startCsharg starts the csharg CLI against the Packetflix service, sending the
// captured packets to stdout, and returns a channel receiving the CLI's exit.
func startCsharg(stdout *os.File, args ...string) <-chan error {
	GinkgoHelper()
	cmd := exec.Command(cshargBin, append([]string{"--host", serviceAddr}, args...)...)
	cmd.Stdout = stdout
	cmd.Stderr = GinkgoWriter
	Expect(cmd.Start()).To(Succeed())
	DeferCleanup(func() { _ = cmd.Process.Kill() })
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	return done
}

// tempPath returns the path of a file with the specified name inside a new
// temporary directory that gets removed after the current spec.
func tempPath(name string) string {
	GinkgoHelper()
	tmpdir := Successful(os.MkdirTemp("", "csharg-e2e-"))
	DeferCleanup(os.RemoveAll, tmpdir)
	return filepath.Join(tmpdir, name)
}

// awaitPackets waits for the capture file at path to grow well beyond its
// section and interface headers.
func awaitPackets(ctx context.Context, path string) {
	GinkgoHelper()
	Eventually(func() (int64, error) {
		fi, err := os.Stat(path)
		if err != nil {
			return 0, err
		}
		return fi.Size(), nil
	}).WithContext(ctx).Within(30 * time.Second).ProbeEvery(250 * time.Millisecond).
		Should(BeNumerically(">", 2048))
}

// fields returns the values of field for the frames in the capture file at
// path matching the (optional) display filter. It fails the current spec if
// tshark cannot fully read the capture file.
func fields(path, displayFilter, field string) []string {
	GinkgoHelper()
	args := []string{"-r", path, "-T", "fields", "-e", field}
	if displayFilter != "" {
		args = append(args, "-Y", displayFilter)
	}
	return strings.Fields(string(Successful(exec.Command("tshark", args...).Output())))
}

// capinfos returns the capture file information including the section comment
// for the capture file at path, failing the current spec if the capture file
// is invalid.
func capinfos(path string) string {
	GinkgoHelper()
	return string(Successful(exec.Command("capinfos", "-k", path).Output()))
}
