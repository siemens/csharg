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
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/siemens/csharg/api"
	"github.com/siemens/csharg/pcapng"
	"github.com/siemens/csharg/websock"
	log "github.com/sirupsen/logrus"
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
	// Lists the available capture targets in this cluster.
	Targets() (ts api.Targets)
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
type CaptureStreamer interface {
	// Stop this capture in an orderly manner. This operation will block until
	// the capture has finally terminated. It is also idempotent.
	Stop()
	// Wait for the capture to terminate, but do not initiate the termination.
	Wait()
	// StopAfter waits the specified duration for the capture to terminate, and
	// terminates it after the duration if necessary.
	StopAfter(d time.Duration)
}

// captureStreamer is the implementation of the CaptureStreamer interface.
type captureStreamer struct {
	// The (wrapped) websocket for the network packet stream.
	cws *websock.ReadingClientWebsocket
	// Signals that the capture (and the capture stream) finally has ended.
	done chan bool
}

// Stop the packet capture and waits for the capture to gracefully terminate.
// See also Wait() for the usecase where a go routine needs to wait for the
// capture to terminate, but will not initiate the termination itself.
func (cs *captureStreamer) Stop() {
	cs.cws.Close()
}

// Wait for the packet capture to terminate, without initiating it. See also
// Stop().
func (cs *captureStreamer) Wait() {
	<-cs.done
}

// StopAfter waits for the packet capture to terminate and terminates it after
// the specified duration if necessary.
func (cs *captureStreamer) StopAfter(d time.Duration) {
	select {
	case <-cs.done:
		// We're toast.
	case <-time.After(d):
		cs.Stop()
	}
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
		if t.Type == "pod" {
			tcached, ok := ts.Pod(t.Name)
			if !ok {
				return nil, fmt.Errorf("non-existing target %+v", *t)
			}
			// Since we're going to update the capture target description, we
			// make a shallow copy first
			tshallow := *tcached
			t = &tshallow
		} else {
			tcached, ok := ts.OnNode(t.NodeName, t.Prefix, t.Name)
			if !ok {
				return nil, fmt.Errorf("non-existing target %+v", *t)
			}
			tshallow := *tcached
			t = &tshallow
		}
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
// data into the given Writer.
func StartCaptureStream(w io.Writer, ws *websocket.Conn, t *api.Target, opts *CaptureOptions) (cs CaptureStreamer, err error) {
	log.Debugf("capturing from: %s %s", t.Type, t.Name)
	log.Debugf("capturing from network interfaces: %s", strings.Join(t.NetworkInterfaces, ", "))

	csimpl := &captureStreamer{
		// Wrap the websocket connection into something more "graceful" when it
		// comes to websocket closing.
		cws:  websock.New(ws),
		done: make(chan bool),
	}
	cs = csimpl
	// Sending the incomming packet capture data from the websocket to the
	// writer is done in a separate go routine. Beyond "just" connecting the
	// websocket stream to the writer, we need to handle either the websocket or
	// the writer to break
	go func() {
		defer close(csimpl.done)
		pcapedit := pcapng.NewStreamEditor(
			w, t, opts.Filter, opts.AvoidPromiscuousMode)
		for {
			// Wait for more packet data to arrive, or the websocket becoming
			// closed/broken.
			data, err := csimpl.cws.Read()
			if err != nil {
				log.Debugf("websocket packet data stream error: %s", err.Error())
				return
			}
			// Now forward the packet data into the Wireshark pipe. But pass it
			// through our pcapng stream editor.
			_, err = pcapedit.Write(data)
			perr, ok := err.(*os.PathError)
			if ok && (perr.Err == os.ErrClosed) {
				log.Errorf("capture stream writer is fed up and does not accpet any more packets.")
				go func() {
					// We need to read further from the websocket in order to
					// keep the control message interaction going during the
					// graceful close. It's just that we're throwing away any
					// packet capture data that might still arrive because it
					// was already in flight.
					log.Debug("draining websocket...")
					for {
						_, err := csimpl.cws.Read()
						if err != nil {
							break
						}
					}
					log.Debug("...drained")
				}()
				return
			} else if err != nil {
				log.Errorf("capture stream writer failed: %s", err.Error())
				return
			}
		}
	}()
	return cs, nil
}

// CaptureServiceHeaders is a convenience function that builds the set of
// capture service HTTP/WS headers required in order to successfully connect via
// the Kubernetes remote API proxy to the capture service -- where the WS
// service request unfortunately looses the URL parameters, so we need to resort
// to HTTP headers. This bug is documented, but doesn't seem to get any
// attention to fix it, which really is unfortunate. Anyway, we use the capture
// service headers also when not passing broken Kubernetes remote API servers,
// to keep things more uniform.
func CaptureServiceHeaders(t *api.Target, opts *CaptureOptions) (header *http.Header, err error) {
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
// we'll take.
func CaptureServiceQueryParams(t *api.Target, opts *CaptureOptions) (values *url.Values, err error) {
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
