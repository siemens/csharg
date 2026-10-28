// (c) Siemens AG 2023
//
// SPDX-License-Identifier: MIT

// Declares the interfaces to the cluster capture service as well as the
// individual captures.

package csharg

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/thediveo/nonstd/xslog"

	"github.com/siemens/csharg/v2/api"
	"github.com/siemens/csharg/v2/pcapng"
	"github.com/siemens/csharg/v2/websock"
)

// CaptureOptions describe a set of options giving more detailed control over
// how to capture network traffic from a capture target and an optional specific
// set of network interfaces to capture from.
type CaptureOptions struct {
	// The list of network interfaces (names) to capture from; defaults to all
	// network interfaces of the capture target (that is, AllNifs) if left zero.
	Nifs Nifs
	// Packet capture filter expression, defaults to no filtering. For its
	// syntax, please refer to:
	// https://www.tcpdump.org/manpages/pcap-filter.7.html
	Filter string
	// If true, avoid switching into promiscuous mode if possible. Please note
	// that it is not possible to disable promiscuous mode: other parallel
	// captures might already have switched on promiscuous mode so we can never
	// force it off. This zero setting defaults to switching promiscuous mode
	// ON.
	AvoidPromiscuousMode bool
}

// Nifs is a list of network interface names.
type Nifs []string

// AllNifs will capture from all available network interfaces of a capture
// target, regardless of how many and which ones it will have. This is just a
// convenience to those situations where a programmer wants to emphasize
// explicitly capturing from all network interfaces of a capture target, instead
// of the implicit zero default.
var AllNifs = Nifs{}

// SharkTank gives access to network captures in clusters via the
// SharkTank cluster capture service.
type SharkTank interface {
	// Lists the available capture targets in this cluster, or returns an error
	// if discovery fails.
	Targets() (ts api.Targets, err error)
	// Captures network traffic from a specific pod and send the captured packet
	// stream to the writer w. The capture optionally can be restricted to only
	// a subset of the pod's network interfaces. The pod name can be prefixed by
	// a namespace in form of "namespace/podname"; if the namespace is left out
	// it defaults to the aptly-named "default" namespace.
	CapturePod(w io.Writer, podname string, opts *CaptureOptions) (cs CaptureStreamer, err error)
	// Captures network traffic from a specific container on a specific
	// kubernetes node. Of course, the capture can be restricted to only a
	// subset of the pod's network interfaces.
	CaptureContainer(w io.Writer, nodename, name string, opts *CaptureOptions) (cs CaptureStreamer, err error)
	// Captures network traffic from a capture target, such as a pod, a
	// stand-alone container, a process-less IP stack, et cetera, optionally
	// limited to a specific (set of) network interface(s) for this target. The
	// captured packets are then send to the given Writer.
	Capture(w io.Writer, t *api.Target, opts *CaptureOptions) (cs CaptureStreamer, err error)
	// Clears the cached set of capture targets: a SharkTank will fetch the set
	// of capture targets anew when it needs them, and will then cache them
	// because typically there will be multiple lookups into the cached set
	// necessary in order to start a capture.
	Clear()
}

// CaptureStreamer gives control over an individual network packet capture.
//
// Stop, Wait, and StopAfter return why the capture ended: nil if it was stopped
// or the capture service ended it normally, otherwise the error that broke the
// capture stream or its writer.
type CaptureStreamer interface {
	// Stop this capture in an orderly manner. This operation will block until
	// the capture has finally terminated. It is also idempotent.
	Stop() error
	// Wait for the capture to terminate, but do not initiate the termination.
	Wait() error
	// StopAfter waits the specified duration for the capture to terminate, and
	// terminates it after the duration if necessary.
	StopAfter(d time.Duration) error
}

// captureStreamer is the implementation of the CaptureStreamer interface.
type captureStreamer struct {
	// The (wrapped) websocket for the network packet stream.
	cws *websock.ReadingClientWebsocket
	// Signals that the capture (and the capture stream) finally has ended.
	done chan bool
	// Set when the capture is being stopped on request.
	stopping atomic.Bool
	// Why the capture ended; only valid after done has been closed.
	err error
}

// Stop the packet capture and waits for the capture to gracefully terminate.
// See also Wait() for the usecase where a go routine needs to wait for the
// capture to terminate, but will not initiate the termination itself.
func (cs *captureStreamer) Stop() error {
	cs.stopping.Store(true)
	cs.cws.Close()
	<-cs.done
	return cs.err
}

// Wait for the packet capture to terminate, without initiating it. See also
// Stop().
func (cs *captureStreamer) Wait() error {
	<-cs.done
	return cs.err
}

// StopAfter waits for the packet capture to terminate and terminates it after
// the specified duration if necessary.
func (cs *captureStreamer) StopAfter(d time.Duration) error {
	select {
	case <-cs.done:
		return cs.err
	case <-time.After(d):
		return cs.Stop()
	}
}

// endedBy returns the error a capture ended by, if any; requested stops and
// normal closes by the capture service don't count as errors.
func (cs *captureStreamer) endedBy(err error) error {
	if cs.stopping.Load() ||
		websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
		return nil
	}
	return err
}

// CompleteTarget completes the capture target description to the point that the
// SharkTank service can be successfully contacted on the service application
// level. If the target description needs to be modified, then CompleteTarget
// will return a shallow copy of the passed target description, with the
// necessary additional data filled in. Otherwise, if the target description is
// already sufficient to start the capture, then it will be returned as-is
// instead.
//
// Please note that any list of network interfaces in the capture target
// description will be replaced by either the list specified in the capture
// options (which takes precedence) or the discovered complete list of network
// interfaces for this target.
func CompleteTarget(t *api.Target, opts *CaptureOptions, ts *TargetCache) (*api.Target, error) {
	// No Nilihists beyond this point.
	if t == nil {
		return nil, errors.New("no capture target specified")
	}
	// If the capture target description lacks the capture service pod instance
	// information, we need to first look it up. That's because the developer of
	// this crap is slightly dumb.
	if t.CaptureService == "" {
		if ts == nil {
			return nil, errors.New("no capture target cache specified")
		}
		var tcached *api.Target
		var ok bool
		if t.Type == "pod" {
			tcached, ok = ts.Pod(t.Prefix, t.Name)
		} else {
			tcached, ok = ts.OnNode(t.NodeName, t.Prefix, t.Name)
		}
		if !ok {
			return nil, fmt.Errorf("non-existing target %+v", *t)
		}
		// The cache hands out copies, so we're free to update it.
		t = tcached
	}
	if opts != nil && len(opts.Nifs) > 0 {
		tshallow := *t
		tshallow.NetworkInterfaces = slices.Clone(opts.Nifs)
		t = &tshallow
	}
	// By now we will have the required information about the particular capture
	// service instance responsible for our capture target.
	if strings.ContainsAny(t.CaptureService, "/?%") {
		return nil, fmt.Errorf("missing or invalid capture service routing for target %+v", t)
	}
	return t, nil
}

// StartCaptureStream is a low-level function almost all cshark package users
// WON'T use. Instead, csharg package users typically want to use the high-level
// SharkTank methods CapturePod, CaptureContainer, and Capture instead. Please
// see the package examples for how to use the high-level capture functions.
//
// The low-level StartCaptureStream which needs to be given an already
// successfully connected websocket, a capture target specification, and capture
// options. It then starts the capture by issuing a capture service request via
// the websocket and then in the background streams the incomming network packet
// data into the given Writer. A nil opts uses the default capture options. On
// error, StartCaptureStream closes the websocket.
func StartCaptureStream(w io.Writer, ws *websocket.Conn, t *api.Target, opts *CaptureOptions) (cs CaptureStreamer, err error) {
	switch {
	case ws == nil:
		return nil, errors.New("no websocket specified")
	case w == nil:
		_ = ws.Close()
		return nil, errors.New("no capture writer specified")
	case t == nil:
		_ = ws.Close()
		return nil, errors.New("no capture target specified")
	}
	if opts == nil {
		opts = &CaptureOptions{}
	}
	slog.Debug("starting capture",
		slog.Group("target",
			slog.String("type", t.Type),
			slog.String("name", t.Name),
		),
		xslog.Lazy("interfaces", func() string {
			return strings.Join(t.NetworkInterfaces, ",")
		}))

	csimpl := &captureStreamer{
		// Wrap the websocket connection into something more "graceful" when it
		// comes to websocket closing.
		cws:  websock.New(ws),
		done: make(chan bool),
	}
	cs = csimpl
	// Sending the incomming packet capture data from the websocket to the
	// writer is done in a separate go routine. Whenever either the websocket or
	// the writer breaks, the websocket gets closed before signalling the end.
	go func() {
		defer close(csimpl.done)
		defer csimpl.closeAndDrain()
		pcapedit := pcapng.NewStreamEditor(
			w, t, opts.Filter, opts.AvoidPromiscuousMode)
		for {
			// Wait for more packet data to arrive, or the websocket becoming
			// closed/broken.
			data, err := csimpl.cws.Read()
			if err != nil {
				slog.Debug("websocket packet data stream failure",
					xslog.Error(err))
				if err = csimpl.endedBy(err); err != nil {
					csimpl.err = fmt.Errorf("capture stream broken: %w", err)
				}
				return
			}
			// Now forward the packet data into the Wireshark pipe. But pass it
			// through our pcapng stream editor.
			if _, err := pcapedit.Write(data); err != nil {
				slog.Debug("capture stream writer failure", xslog.Error(err))
				if err = csimpl.endedBy(err); err != nil {
					csimpl.err = fmt.Errorf("capture stream writer failure: %w", err)
				}
				return
			}
		}
	}()
	return cs, nil
}

// closeAndDrain gracefully closes the websocket, discarding any packet data
// still in flight, so that the close handshake can complete.
func (cs *captureStreamer) closeAndDrain() {
	go cs.cws.Close()
	for {
		if _, err := cs.cws.Read(); err != nil {
			return
		}
	}
}

// CaptureServiceHeaders is a convenience function that builds the set of
// capture service HTTP/WS headers required in order to successfully connect via
// the Kubernetes remote API proxy to the capture service -- where the WS
// service request unfortunately looses the URL parameters, so we need to resort
// to HTTP headers. This bug is documented, but doesn't seem to get any
// attention to fix it, which really is unfortunate. Anyway, we use the capture
// service headers also when not passing broken Kubernetes remote API servers,
// to keep things more uniform. A nil opts uses the default capture options.
func CaptureServiceHeaders(t *api.Target, opts *CaptureOptions) (header *http.Header, err error) {
	if t == nil {
		return nil, errors.New("no capture target specified")
	}
	if opts == nil {
		opts = &CaptureOptions{}
	}
	ctext, err := json.Marshal(t)
	if err != nil {
		return
	}
	// If the options specify the network interfaces to capture from, then take
	// this options set. If this is set to AllNifs, then try to figure the exact
	// set of network interfaces from the target description. And if that
	// doesn't give us a clue, then fall back to "all" as the last resort.
	nifs := opts.Nifs
	if len(nifs) == 0 {
		nifs = t.NetworkInterfaces
	}
	if len(nifs) == 0 {
		nifs = []string{"all"}
	}
	// Create the necessary headers...
	header = &http.Header{
		"Clustershark-Container": {string(ctext)},
		"Clustershark-Nif":       {strings.Join(nifs, "/")},
	}
	if opts.AvoidPromiscuousMode {
		header.Set("Clustershark-Chaste", "")
	}
	if len(opts.Filter) > 0 {
		header.Set("Clustershark-Filter", opts.Filter)
	}
	return
}

// CaptureServiceQueryParams is a convenience function that builds the HTTP/WS
// URL query parameters necessary to connect successfully to the capture service
// -- unless there's a broken Kubernetes remote API proxy in between where we
// query params get lost in transit, but we uniformly use the query params
// whenever we contact the SharkTank capture service, regardless of the path
// we'll take. A nil opts uses the default capture options.
func CaptureServiceQueryParams(t *api.Target, opts *CaptureOptions) (values *url.Values, err error) {
	if t == nil {
		return nil, errors.New("no capture target specified")
	}
	if opts == nil {
		opts = &CaptureOptions{}
	}
	ctext, err := json.Marshal(t)
	if err != nil {
		return
	}
	// If the options specify the network interfaces to capture from, then take
	// this options set. If this is set to AllNifs, then try to figure the exact
	// set of network interfaces from the target description. And if that
	// doesn't give us a clue, then fall back to "all" as the last resort.
	nifs := opts.Nifs
	if len(nifs) == 0 {
		nifs = t.NetworkInterfaces
	}
	if len(nifs) == 0 {
		nifs = []string{"all"}
	}
	// Create the necessary query params...
	values = &url.Values{}
	values.Set("container", string(ctext))
	values.Set("nif", strings.Join(nifs, "/"))
	if opts.AvoidPromiscuousMode {
		values.Set("chaste", "")
	}
	if len(opts.Filter) > 0 {
		values.Set("filter", opts.Filter)
	}
	return
}
