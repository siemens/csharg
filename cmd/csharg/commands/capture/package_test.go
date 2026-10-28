// (c) Siemens AG 2026
//
// SPDX-License-Identifier: MIT

package capture

import (
	"io"
	"sync/atomic"
	"testing"
	"time"

	"github.com/siemens/csharg/v2"
	"github.com/siemens/csharg/v2/api"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestCaptureCmd(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Csharg cmd/csharg/commands/capture package suite")
}

// fakeStreamer is a capture stream fake whose end the test controls by closing
// ended; it records whether Stop was called but otherwise doesn't stop. It
// never streams any packet data.
type fakeStreamer struct {
	ended   chan struct{} // close to simulate the stream ending by itself.
	err     error         // returned by Wait after the stream ended.
	stopped atomic.Bool   // set when Stop was called.
}

var _ csharg.CaptureStreamer = (*fakeStreamer)(nil)

func (s *fakeStreamer) Stop() error                     { s.stopped.Store(true); return nil }
func (s *fakeStreamer) Wait() error                     { <-s.ended; return s.err }
func (s *fakeStreamer) StopAfter(d time.Duration) error { return nil }

// fakeShark is a capture client fake offering a single "foo" target unless
// told otherwise; its captures write "packets" to the writer and return the
// same streamer.
type fakeShark struct {
	streamer   *fakeStreamer
	targets    api.Targets
	targetsErr error
	captureErr error
	target     *api.Target            // target of the last successful Capture.
	opts       *csharg.CaptureOptions // options of the last successful Capture.
}

var _ csharg.SharkTank = (*fakeShark)(nil)

func (f *fakeShark) Capture(w io.Writer, t *api.Target, opts *csharg.CaptureOptions) (csharg.CaptureStreamer, error) {
	if f.captureErr != nil {
		return nil, f.captureErr
	}
	f.target, f.opts = t, opts
	_, _ = io.WriteString(w, "packets")
	return f.streamer, nil
}
func (f *fakeShark) CaptureContainer(w io.Writer, nodename, name string, opts *csharg.CaptureOptions) (csharg.CaptureStreamer, error) {
	return f.streamer, nil
}
func (f *fakeShark) CapturePod(w io.Writer, podname string, opts *csharg.CaptureOptions) (csharg.CaptureStreamer, error) {
	return f.streamer, nil
}
func (f *fakeShark) Clear() {}
func (f *fakeShark) Targets() (api.Targets, error) {
	if f.targetsErr != nil {
		return nil, f.targetsErr
	}
	if f.targets != nil {
		return f.targets, nil
	}
	return api.Targets{{Name: "foo", Type: "docker"}}, nil
}
