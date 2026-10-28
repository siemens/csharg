// (c) Siemens AG 2023
//
// SPDX-License-Identifier: MIT

// Implements the capture client to access the capture service on a standalone
// Docker container host. For convenience, the capture service acts as a
// simplified "frontend" in this scenario.

package csharg

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/gorilla/websocket"
	"github.com/thediveo/nonstd/xslog"

	"github.com/siemens/csharg/v2/api"
)

// SharkTankOnHostOptions allows some degree of control over how to use a
// (SharkTank) Packetflix service reachable at a given address and port.
type SharkTankOnHostOptions struct {
	CommonClientOptions
	InsecureSkipVerify bool
}

// maxDiscoveryResponseSize limits the size of target discovery responses.
const maxDiscoveryResponseSize = 16 << 20

// NewSharkTankOnHost returns a new host capturer object to capture directly
// from host targets using a Packetflix service, and accessing it via host, an
// optional port, and an optional service path.
func NewSharkTankOnHost(hosturl string, opts *SharkTankOnHostOptions) (st SharkTank, err error) {
	// First checkpoint: if it doesn't start with the http/s scheme, then go for http.
	if !strings.HasPrefix(hosturl, "http://") && !strings.HasPrefix(hosturl, "https://") {
		hosturl = "http://" + hosturl
	}
	surl, err := url.Parse(hosturl)
	if err != nil {
		return
	}
	// Don't accept fragments and query elements.
	if surl.User != nil || surl.Opaque != "" ||
		surl.RawQuery != "" || surl.Fragment != "" {
		return nil, errors.New("only host name and optional port number allowed")
	}
	uc := &hostsharktank{
		hosturl: surl,
		opts: SharkTankOnHostOptions{
			CommonClientOptions: CommonClientOptions{
				Timeout: DefaultServiceTimeout,
			},
		},
	}
	if opts != nil {
		uc.opts = *opts
	}
	return uc, nil
}

// hostsharktank implements the UrlCapturer interface for a standalone host,
// where the Packetflix capture service can be "directly" reached via
// host+port-only URL.
type hostsharktank struct {
	// Host+Port (+ optional path) URL of the Packetflix service REST API.
	hosturl *url.URL
	// Options
	opts SharkTankOnHostOptions
	// Cached capture targets
	cache TargetCache
	// Discovery transport; nil uses http.DefaultTransport.
	transport http.RoundTripper
}

// Captures network traffic from a specific pod and send the captured packet
// stream to the writer w. The capture optionally can be restricted to only a
// subset of the pod's network interfaces. The pod name can be prefixed by a
// namespace in form of "namespace/podname"; if the namespace is left out it
// defaults to the aptly-named "default" namespace.
//
// In principle, a standalone Docker host won't have Kubernetes pods, but then,
// we don't block this function, because we cannot deny any pod existence. Talk
// about KinD...
func (hc *hostsharktank) CapturePod(w io.Writer, pod string, opts *CaptureOptions) (cs CaptureStreamer, err error) {
	p := strings.Split(pod, "/")
	switch len(p) {
	case 1:
		p = []string{"default", p[0]}
	case 2:
		// ...already has a namespace, so we're done here.
	default:
		return nil, fmt.Errorf("invalid pod namespace/name: %q", pod)
	}
	t := &api.Target{
		Name: strings.Join(p, "/"),
		Type: "pod",
	}
	return hc.Capture(w, t, opts)
}

// CaptureContainer captures the network traffic from a specific container on a
// specific cluster node and then sends the captured packet stream to the writer
// w. The capture optionally can be restricted to only a subset of the
// containers/pod's network interfaces.
func (hc *hostsharktank) CaptureContainer(w io.Writer, nodename, name string, opts *CaptureOptions) (cs CaptureStreamer, err error) {
	t := &api.Target{
		Name:     name,
		NodeName: nodename,
	}
	return hc.Capture(w, t, opts)
}

// needsTargetDiscovery, given a capture target description, returns true if the
// caller should run a full (and slightly expensive) target discovery. This
// allows a performance optimization for the standalone container host case
// where no capture service forwarding information is required, but the list of
// network interfaces, if the caller cannot supply it.
func needsTargetDiscovery(t *api.Target) bool {
	// Do we use the cluster capture service and thus do we need forwarding
	// information AND the forwarding information is yet missing?
	return len(t.NetworkInterfaces) == 0
}

// Captures network traffic from a capture target, such as a pod, a stand-alone
// container, a process-less IP stack, et cetera, optionally limited to a
// specific (set of) network interface(s) for this target. The captured packets
// are then send to the given Writer. This implementation hides the details how
// to connect to the discovery/capture service.
//
// If t lacks network interfaces, Capture completes it from the cached capture
// targets, discovering them first only if the cache is empty.
func (hc *hostsharktank) Capture(w io.Writer, t *api.Target, opts *CaptureOptions) (cs CaptureStreamer, err error) {
	if t == nil {
		return nil, errors.New("no capture target specified")
	}
	if w == nil {
		return nil, errors.New("no capture writer specified")
	}
	if opts == nil {
		opts = &CaptureOptions{}
	}
	// Discover (or reuse cached) targets only if the caller didn't supply the
	// network interfaces to capture from.
	if needsTargetDiscovery(t) {
		if _, err = hc.Targets(); err != nil {
			return
		}
		if t, err = CompleteTarget(t, opts, &hc.cache); err != nil {
			return
		}
	} else {
		slog.Debug("skipping unneeded target discovery")
	}
	// Prepare the necessary URL query parameters and request headers in order
	// to suckcessfully start a capture...
	wsheaders, err := CaptureServiceHeaders(t, opts)
	if err != nil {
		slog.Error("service request header failure", xslog.Error(err))
		return
	}
	if hc.opts.BearerToken != "" {
		wsheaders.Set("Authorization", "Bearer "+hc.opts.BearerToken)
	}
	query, err := CaptureServiceQueryParams(t, opts)
	if err != nil {
		slog.Error("service request query parameter failure", xslog.Error(err))
		return
	}
	apiurl := *hc.hosturl
	if apiurl.Scheme == "https" {
		apiurl.Scheme = "wss"
	} else {
		apiurl.Scheme = "ws"
	}
	apiurl.Path = path.Join(apiurl.Path, "capture")
	apiurl.RawQuery = query.Encode()

	// Finally: off to capture...
	slog.Debug("connecting to capture service",
		slog.String("api", apiurl.String()),
		slog.Any("timeout", hc.opts.Timeout))
	wsd := &websocket.Dialer{
		Proxy:            http.ProxyFromEnvironment,
		HandshakeTimeout: hc.opts.Timeout,
	}
	if hc.opts.InsecureSkipVerify && apiurl.Scheme == "wss" {
		wsd.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
	wscon, resp, err := wsd.Dial(apiurl.String(), *wsheaders)
	if err != nil {
		slog.Error("cannot contact capture service via websocket", xslog.Error(err))
		return
	}
	slog.Debug("capture service initial HTTP response",
		xslog.Lazy("response", func() string { return fmt.Sprint(*resp) }))
	return StartCaptureStream(w, wscon, t, opts)
}

// Targets discovers the available capture targets on this host. The targets'
// node name is the host name of the service URL, without any port.
func (hc *hostsharktank) Targets() (ts api.Targets, err error) {
	return hc.discover()
}

// Clear the internally cached set of capture targets: this will cause the next
// discover and capture operation to automatically get a fresh set.
func (hc *hostsharktank) Clear() {
	hc.cache.Clear()
}

// Discovers the available capture targets on a standalone Docker host from the
// capture service,  sending an HTTP(S) GET request to the given service URL.
func (hc *hostsharktank) discover() (api.Targets, error) {
	// If we already have a cached set of capture targets, then avoid the
	// roundtrip to the cluster capture service and instead quickly return the
	// cached set.
	if !hc.cache.IsEmpty() {
		return hc.cache.Targets(), nil
	}
	// Derive the discovery service API URL from the base URL for the SharkTank
	// cluster capture service. Then issue a simple HTTP/S GET request and hope
	// that the result does make sense in that it can be decoded.
	apiurl := *hc.hosturl
	apiurl.Path = path.Join(apiurl.Path, "discover/mobyshark")
	slog.Debug("querying targets from GhostWire-on-Packetflix service",
		slog.String("api", apiurl.String()),
		slog.Any("timeout", hc.opts.Timeout))
	httpclient := &http.Client{
		Timeout:   hc.opts.Timeout,
		Transport: hc.transport,
	}
	if hc.opts.InsecureSkipVerify && apiurl.Scheme == "https" {
		// Clone so as to never weaken TLS for other users of the default transport.
		httptrans := http.DefaultTransport.(*http.Transport).Clone()
		httptrans.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
		httpclient.Transport = httptrans
		defer httptrans.CloseIdleConnections()
	}
	req, err := http.NewRequest("GET", apiurl.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("cannot create target discovery request: %w", err)
	}
	if hc.opts.BearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+hc.opts.BearerToken)
	}
	res, err := httpclient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("querying targets from GhostWire-on-Packetflix service failed: %w", err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("querying targets from GhostWire-on-Packetflix service failed: %s", res.Status)
	}
	var td api.GwTargetList
	body := &io.LimitedReader{R: res.Body, N: maxDiscoveryResponseSize + 1}
	err = json.NewDecoder(body).Decode(&td)
	if err != nil {
		if body.N <= 0 {
			return nil, fmt.Errorf("targets from GhostWire-on-Packetflix service exceed %d octets",
				maxDiscoveryResponseSize)
		}
		return nil, fmt.Errorf("cannot decode targets from GhostWire-on-Packetflix service: %w", err)
	}
	// Since we don't have the cluster capture frontend service, we need to fill
	// in some missing data to get a target list consistent with what a cluster
	// capture service would return.
	hostn := hc.hosturl.Hostname()
	for _, t := range td.Targets {
		t.NodeName = hostn
	}
	// Cache the capture target descriptions for further quick reference.
	hc.cache.Set(td.Targets)
	return td.Targets, nil
}
