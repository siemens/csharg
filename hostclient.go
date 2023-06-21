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
	"net"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/siemens/csharg/api"

	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
)

// SharkTankOnHostOptions allows some degree of control over how to use a
// (SharkTank) Packetflix service reachable at a given address and port.
type SharkTankOnHostOptions struct {
	CommonClientOptions
	InsecureSkipVerify bool
}

// NewSharkTankOnHost returns a new host capturer object to capture directly
// from host targets using a Packetflix service, and accessing it via host+port
// and an optional service path.
func NewSharkTankOnHost(hosturl string, opts *SharkTankOnHostOptions) (st SharkTank, err error) {
	// First checkpoint: if it doesn't start with the http/s scheme, then go for http.
	if !strings.HasPrefix(hosturl, "http:") && !strings.HasPrefix(hosturl, "https://") {
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
func (hc *hostsharktank) Capture(w io.Writer, t *api.Target, opts *CaptureOptions) (cs CaptureStreamer, err error) {
	if opts == nil {
		opts = &CaptureOptions{}
	}
	// Fill the cache only if we don't have to necessary information we might
	// want to fill in...
	if hc.cache.IsEmpty() && needsTargetDiscovery(t) {
		hc.Targets()
		if t, err = CompleteTarget(t, opts, &hc.cache); err != nil {
			return
		}
	} else {
		log.Debug("skipping unneeded target discovery")
	}
	// Prepare the necessary URL query parameters and request headers in order
	// to suckcessfully start a capture...
	wsheaders, err := CaptureServiceHeaders(t, opts)
	if err != nil {
		log.Errorf("service request header failure: %q", err.Error())
		return
	}
	if hc.opts.BearerToken != "" {
		wsheaders.Set("Authorization", "Bearer "+hc.opts.BearerToken)
	}
	query, err := CaptureServiceQueryParams(t, opts)
	if err != nil {
		log.Errorf("service request query parameter failure: %q", err.Error())
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
	log.Debugf("connecting to capture service %q, time limit %s", apiurl.String(), hc.opts.Timeout)
	wsd := &websocket.Dialer{
		Proxy:            http.ProxyFromEnvironment,
		HandshakeTimeout: hc.opts.Timeout,
	}
	if hc.opts.InsecureSkipVerify && apiurl.Scheme == "wss" {
		wsd.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
	wscon, resp, err := wsd.Dial(apiurl.String(), *wsheaders)
	if err != nil {
		log.Errorf("cannot contact capture service via websocket: %s", err.Error())
		return
	}
	log.Debugf("capture service initial HTTP response: %+v", *resp)
	return StartCaptureStream(w, wscon, t, opts)
}

// Targets discovers the available capture targets in this cluster.
func (hc *hostsharktank) Targets() (ts api.Targets) {
	return hc.discover()
}

// Clear the internally cached set of capture targets: this will cause the next
// discover and capture operation to automatically get a fresh set.
func (hc *hostsharktank) Clear() {
	hc.cache.Clear()
}

// Discovers the available capture targets on a standalone Docker host from the
// capture service,  sending an HTTP(S) GET request to the given service URL.
func (hc *hostsharktank) discover() (ts api.Targets) {
	// If we already have a cached set of capture targets, then avoid the
	// roundtrip to the cluster capture service and instead quickly return the
	// cached set.
	if !hc.cache.IsEmpty() {
		return hc.cache.Targets()
	}
	// Derive the discovery service API URL from the base URL for the SharkTank
	// cluster capture service. Then issue a simple HTTP/S GET request and hope
	// that the result does make sense in that it can be decoded.
	apiurl := *hc.hosturl
	apiurl.Path = path.Join(apiurl.Path, "discover/mobyshark")
	log.Debugf("querying targets from GhostWire-on-Packetflix service %q, time limit %s", apiurl.String(), hc.opts.Timeout)
	httptrans := http.DefaultTransport.(*http.Transport)
	if hc.opts.InsecureSkipVerify && apiurl.Scheme == "https" {
		httptrans.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
	httpclient := &http.Client{
		Timeout:   hc.opts.Timeout,
		Transport: httptrans,
	}
	req, err := http.NewRequest("GET", apiurl.String(), nil)
	if err != nil {
		log.Errorf("cannot create new HTTP request: %s", err.Error())
		return api.Targets{}
	}
	if hc.opts.BearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+hc.opts.BearerToken)
	}
	res, err := httpclient.Do(req)
	if err != nil {
		log.Errorf("querying targets from GhostWire-on-Packetflix service failed: %s", err.Error())
		return api.Targets{}
	}
	defer res.Body.Close()
	var td api.GwTargetList
	err = json.NewDecoder(res.Body).Decode(&td)
	if err != nil {
		log.Errorf("cannot decode targets from GhostWire-on-Packetflix service: %s", err.Error())
		return api.Targets{}
	}
	// Since we don't have the cluster capture frontend service, we need to fill
	// in some missing data to get a target list consistent with what a cluster
	// capture service would return.
	hostn, _, _ := net.SplitHostPort(hc.hosturl.Host)
	for _, t := range td.Targets {
		t.NodeName = hostn
	}
	// Cache the capture target descriptions for further quick reference.
	hc.cache.Set(td.Targets)
	return td.Targets
}
