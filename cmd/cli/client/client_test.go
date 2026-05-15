// (c) Siemens AG 2026
//
// SPDX-License-Identifier: MIT

package client

import (
	"errors"
	"io"

	"github.com/spf13/cobra"
	"github.com/thediveo/go-plugger/v3"

	"github.com/siemens/csharg"
	"github.com/siemens/csharg/api"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("SharkTank clients", func() {

	BeforeEach(func() {
		grp := plugger.Group[New]()
		DeferCleanup(grp.Restore, grp.Backup())
		grp.Clear()
	})

	It("returns available clients when there's no match", func() {
		Expect(NewSharkTank(nil)).Error().To(MatchError(ContainSubstring("(none)")))

		plugger.Group[New]().Register(func(c *cobra.Command) (csharg.SharkTank, error) {
			return nil, nil
		}, plugger.WithPlugin("foo"))
		plugger.Group[New]().Register(func(c *cobra.Command) (csharg.SharkTank, error) {
			return nil, nil
		}, plugger.WithPlugin("bar"))
		Expect(NewSharkTank(nil)).Error().To(MatchError(ContainSubstring("bar, foo")))
	})

	It("returns plugin errors", func() {
		plugger.Group[New]().Register(func(c *cobra.Command) (csharg.SharkTank, error) {
			return nil, errors.New("JKL305")
		}, plugger.WithPlugin("foo"))
		plugger.Group[New]().Register(func(c *cobra.Command) (csharg.SharkTank, error) {
			return nil, nil
		}, plugger.WithPlugin("bar"))
		Expect(NewSharkTank(nil)).Error().To(MatchError(ContainSubstring("JKL305")))
	})

	It("returns a first client", func() {
		plugger.Group[New]().Register(func(c *cobra.Command) (csharg.SharkTank, error) {
			return &fakeshark{}, nil
		}, plugger.WithPlugin("foo"))
		plugger.Group[New]().Register(func(c *cobra.Command) (csharg.SharkTank, error) {
			return nil, nil
		}, plugger.WithPlugin("bar"))
		Expect(NewSharkTank(nil)).NotTo(BeNil())
	})

})

type fakeshark struct{}

var _ csharg.SharkTank = (*fakeshark)(nil)

func (fakeshark) Capture(w io.Writer, t *api.Target, opts *csharg.CaptureOptions) (cs csharg.CaptureStreamer, err error) {
	return nil, nil
}
func (fakeshark) CaptureContainer(w io.Writer, nodename, name string, opts *csharg.CaptureOptions) (cs csharg.CaptureStreamer, err error) {
	return nil, nil
}
func (fakeshark) CapturePod(w io.Writer, podname string, opts *csharg.CaptureOptions) (cs csharg.CaptureStreamer, err error) {
	return nil, nil
}
func (fakeshark) Clear()                    {}
func (fakeshark) Targets() (ts api.Targets) { return nil }
