// (c) Siemens AG 2026
//
// SPDX-License-Identifier: MIT

package capture

import (
	"bytes"
	"errors"
	"fmt"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/thediveo/go-plugger/v3"

	"github.com/siemens/csharg/v2"
	"github.com/siemens/csharg/v2/cmd/cli/client"

	"github.com/onsi/gomega/types"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("capture command", func() {

	BeforeEach(func() {
		grp := plugger.Group[client.New]()
		DeferCleanup(grp.Restore, grp.Backup())
		grp.Clear()
	})

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
