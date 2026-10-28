// (c) Siemens AG 2026
//
// SPDX-License-Identifier: MIT

package capture

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/thediveo/go-plugger/v3"

	"github.com/siemens/csharg/v2"
	"github.com/siemens/csharg/v2/api"
	"github.com/siemens/csharg/v2/cmd/cli/client"

	"github.com/onsi/gomega/types"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// execute runs "csharg" with the specified args against the fake capture
// client st, whose capture stream ends immediately, returning stdout.
func execute(st *fakeShark, args ...string) (string, error) {
	GinkgoHelper()
	if st.streamer == nil {
		st.streamer = &fakeStreamer{ended: make(chan struct{})}
		close(st.streamer.ended)
	}
	plugger.Group[client.New]().Register(func(*cobra.Command) (csharg.SharkTank, error) {
		return st, nil
	}, plugger.WithPlugin("fake"))
	return executeRoot(args...)
}

// executeRoot runs "csharg" with the specified args, returning stdout.
func executeRoot(args ...string) (string, error) {
	root := &cobra.Command{Use: "csharg", SilenceUsage: true}
	root.PersistentFlags().String("host", "", "")
	captureSetupCLI(root)
	out := &bytes.Buffer{}
	root.SetArgs(args)
	root.SetOut(out)
	root.SetErr(GinkgoWriter)
	err := root.Execute()
	return out.String(), err
}

var _ = Describe("capture command", func() {

	BeforeEach(func() {
		grp := plugger.Group[client.New]()
		DeferCleanup(grp.Restore, grp.Backup())
		grp.Clear()
	})

	It("reports a missing capture client", func() {
		Expect(executeRoot("capture", "foo")).Error().To(
			MatchError(ContainSubstring("no suitable capture API client found")))
	})

	It("reports a failing capture client", func() {
		plugger.Group[client.New]().Register(func(*cobra.Command) (csharg.SharkTank, error) {
			return nil, errors.New("no shark")
		}, plugger.WithPlugin("fake"))
		Expect(executeRoot("capture", "foo")).Error().To(MatchError("no shark"))
	})

	DescribeTable("rejects",
		func(st *fakeShark, args []string, expectedErr string) {
			Expect(execute(st, args...)).Error().To(MatchError(ContainSubstring(expectedErr)))
			Expect(st.target).To(BeNil())
		},
		Entry("an empty target name", &fakeShark{},
			[]string{"capture", ""}, "invalid empty capture target name"),
		Entry("when targets can't be discovered", &fakeShark{targetsErr: errors.New("no targets")},
			[]string{"capture", "foo"}, "no targets"),
		Entry("an unknown target", &fakeShark{},
			[]string{"capture", "bar"}, `capture target "bar" not found`),
		Entry("an unknown target on a node", &fakeShark{},
			[]string{"capture", "container", "foo", "node-1"}, `capture target "foo" on node "node-1" not found`),
		Entry("an ambiguous target", &fakeShark{targets: api.Targets{
			{Name: "foo", Type: "docker", NodeName: "node-1"},
			{Name: "foo", Type: "docker", NodeName: "node-2"},
		}}, []string{"capture", "foo"}, `ambiguous capture target "foo" matches 2 targets`),
		Entry("an uncreatable output file", &fakeShark{},
			[]string{"capture", "-w", "/nonexisting/dir/cap.pcapng", "foo"}, "cannot create packet capture file"),
		Entry("when the capture can't start", &fakeShark{captureErr: errors.New("no capture")},
			[]string{"capture", "foo"}, "cannot start capture: no capture"),
	)

	It("streams to stdout", func() {
		st := &fakeShark{}
		Expect(execute(st, "capture", "foo")).To(Equal("packets"))
		Expect(st.opts).To(Equal(&csharg.CaptureOptions{}))
	})

	It("streams to a file", func() {
		path := filepath.Join(GinkgoT().TempDir(), "cap.pcapng")
		Expect(execute(&fakeShark{}, "capture", "-w", path, "foo")).To(BeEmpty())
		Expect(os.ReadFile(path)).To(Equal([]byte("packets")))
	})

	It("passes capture options", func() {
		st := &fakeShark{}
		Expect(execute(st, "capture", "-i", "eth0", "-i", "eth1", "-f", "tcp port 80", "-p", "foo")).
			Error().NotTo(HaveOccurred())
		Expect(st.opts).To(Equal(&csharg.CaptureOptions{
			Nifs:                 csharg.Nifs{"eth0", "eth1"},
			Filter:               "tcp port 80",
			AvoidPromiscuousMode: true,
		}))
	})

	DescribeTable("selects the target",
		func(args []string, expectedIdx int) {
			targets := api.Targets{
				{Name: "foo", Type: "docker", NodeName: "node-1"},
				{Name: "foo", Type: "docker", NodeName: "node-2"},
				{Name: "foo", Type: "proc", NodeName: "node-1"},
				{Name: "bar", Type: "bindmount", NodeName: "node-1"},
				{Name: "default/foo", Type: "pod"},
				{Name: "kube/foo", Type: "pod"},
				{Name: "moby", Type: "docker", NodeName: "localhost"},
			}
			st := &fakeShark{targets: targets}
			Expect(execute(st, args...)).Error().NotTo(HaveOccurred())
			Expect(st.target).To(BeIdenticalTo(targets[expectedIdx]))
		},
		Entry("of any type", []string{"capture", "moby"}, 6),
		Entry("container on node", []string{"capture", "container", "foo", "node-1"}, 0),
		Entry("container on other node", []string{"capture", "container", "foo", "node-2"}, 1),
		Entry("container on stand-alone host, ignoring node",
			[]string{"--host", "localhost:5001", "capture", "container", "moby", "node-1"}, 6),
		Entry("process network", []string{"capture", "network", "foo", "node-1"}, 2),
		Entry("bind-mounted network", []string{"capture", "network", "bar", "node-1"}, 3),
		Entry("pod in default namespace", []string{"capture", "pod", "foo"}, 4),
		Entry("pod in namespace flag", []string{"capture", "pod", "-n", "kube", "foo"}, 5),
		Entry("pod in namespaced name", []string{"capture", "pod", "kube/foo"}, 5),
	)

	DescribeTable("returns when the capture stream ends by itself",
		func(streamErr error, expected types.GomegaMatcher) {
			streamer := &fakeStreamer{ended: make(chan struct{}), err: streamErr}
			plugger.Group[client.New]().Register(func(*cobra.Command) (csharg.SharkTank, error) {
				return &fakeShark{streamer: streamer}, nil
			}, plugger.WithPlugin("fake"))

			cmd := newCaptureCmd()
			cmd.SetArgs([]string{"foo"})
			cmd.SetOut(&bytes.Buffer{})
			cmd.SetErr(GinkgoWriter)
			result := make(chan error, 1)
			go func() {
				defer GinkgoRecover()
				result <- cmd.Execute()
			}()

			Consistently(result).WithTimeout(100*time.Millisecond).ShouldNot(Receive(),
				"returned while capture stream still running")
			close(streamer.ended)
			Eventually(result).WithTimeout(2 * time.Second).Should(Receive(expected))
			Expect(streamer.stopped.Load()).To(BeFalse())
		},
		Entry("normally", nil, BeNil()),
		Entry("on a closed consumer pipe",
			fmt.Errorf("capture stream writer failure: %w", syscall.EPIPE), BeNil()),
		Entry("on a broken stream",
			errors.New("capture stream broken: fooled"), MatchError(ContainSubstring("fooled"))),
	)

})
