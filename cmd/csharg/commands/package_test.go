// (c) Siemens AG 2026
//
// SPDX-License-Identifier: MIT

package commands

import (
	"io"
	"testing"

	"github.com/siemens/csharg/v2"
	"github.com/siemens/csharg/v2/api"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestCommands(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Csharg cmd/csharg/commands package suite")
}

// fakeShark is a capture client fake that only lists targets.
type fakeShark struct {
	targets    api.Targets
	targetsErr error
}

var _ csharg.SharkTank = (*fakeShark)(nil)

func (f *fakeShark) Capture(io.Writer, *api.Target, *csharg.CaptureOptions) (csharg.CaptureStreamer, error) {
	return nil, nil
}
func (f *fakeShark) CaptureContainer(io.Writer, string, string, *csharg.CaptureOptions) (csharg.CaptureStreamer, error) {
	return nil, nil
}
func (f *fakeShark) CapturePod(io.Writer, string, *csharg.CaptureOptions) (csharg.CaptureStreamer, error) {
	return nil, nil
}
func (f *fakeShark) Clear() {}
func (f *fakeShark) Targets() (api.Targets, error) {
	return f.targets, f.targetsErr
}
