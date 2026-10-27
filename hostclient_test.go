// (c) Siemens AG 2026
//
// SPDX-License-Identifier: MIT

package csharg

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"

	"github.com/siemens/csharg/v2/api"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("host client", func() {

	It("completes targets from an already filled cache", func(ctx context.Context) {
		var discoveries atomic.Int32
		nifs := make(chan string, 1)
		upgrader := websocket.Upgrader{}
		mux := http.NewServeMux()
		mux.HandleFunc("/discover/mobyshark", func(rw http.ResponseWriter, r *http.Request) {
			discoveries.Add(1)
			_, _ = io.WriteString(rw,
				`{"containers":[{"name":"foo","type":"docker","network-interfaces":["eth0","lo"]}]}`)
		})
		mux.HandleFunc("/capture", func(rw http.ResponseWriter, r *http.Request) {
			nifs <- r.Header.Get("Clustershark-Nif")
			conn, err := upgrader.Upgrade(rw, r, nil)
			if err != nil {
				return
			}
			defer func() { _ = conn.Close() }()
			_ = conn.WriteMessage(websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			drain(conn)
		})
		srv := httptest.NewServer(mux)
		DeferCleanup(srv.Close)

		st, err := NewSharkTankOnHost(srv.URL, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(st.Targets()).To(HaveLen(1))

		cs, err := st.Capture(io.Discard, &api.Target{Name: "foo", NodeName: "127.0.0.1"}, nil)
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(cs.Stop)

		Eventually(ctx, nifs).Should(Receive(Equal("eth0/lo")))
		Expect(discoveries.Load()).To(Equal(int32(1)))
	}, SpecTimeout(10*time.Second))

	DescribeTable("sets the node name from the host URL",
		func(hosturl, nodename string) {
			srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
				_, _ = io.WriteString(rw, `{"containers":[{"name":"foo","type":"docker"}]}`)
			}))
			DeferCleanup(srv.Close)
			// Route any host:port to the test server, as the URL may lack a port.
			trans := http.DefaultTransport.(*http.Transport).Clone()
			trans.Proxy = nil
			trans.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
				return (&net.Dialer{}).DialContext(ctx, network, srv.Listener.Addr().String())
			}
			DeferCleanup(trans.CloseIdleConnections)

			st, err := NewSharkTankOnHost(hosturl, nil)
			Expect(err).NotTo(HaveOccurred())
			st.(*hostsharktank).transport = trans
			ts, err := st.Targets()
			Expect(err).NotTo(HaveOccurred())
			Expect(ts).To(HaveLen(1))
			Expect(ts[0].NodeName).To(Equal(nodename))
		},
		Entry("without port", "http://foo.example", "foo.example"),
		Entry("with port", "http://foo.example:5001", "foo.example"),
		Entry("IPv6 without port", "http://[fe80::1]", "fe80::1"),
		Entry("IPv6 with port", "http://[fe80::1]:5001", "fe80::1"),
	)

	It("rejects targets from failed discovery responses", func() {
		srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			rw.WriteHeader(http.StatusUnauthorized)
			_, _ = io.WriteString(rw, `{"containers":[{"name":"foo","type":"docker"}]}`)
		}))
		DeferCleanup(srv.Close)

		st, err := NewSharkTankOnHost(srv.URL, nil)
		Expect(err).NotTo(HaveOccurred())
		ts, err := st.Targets()
		Expect(err).To(MatchError(ContainSubstring("401 Unauthorized")))
		Expect(ts).To(BeEmpty())
		Expect(st.(*hostsharktank).cache.IsEmpty()).To(BeTrue())
	})

	It("rejects oversized discovery responses", func() {
		srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(rw, `{"containers":[],"padding":"`+
				strings.Repeat("A", maxDiscoveryResponseSize)+`"}`)
		}))
		DeferCleanup(srv.Close)

		st, err := NewSharkTankOnHost(srv.URL, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(st.Targets()).Error().To(MatchError(ContainSubstring("exceed")))
	})

	It("reports unreachable capture services", func() {
		srv := httptest.NewServer(http.NotFoundHandler())
		srv.Close()

		st, err := NewSharkTankOnHost(srv.URL, nil)
		Expect(err).NotTo(HaveOccurred())
		_, err = st.Targets()
		Expect(err).To(MatchError(ContainSubstring("connection refused")))
		_, err = st.Capture(io.Discard, &api.Target{Name: "foo"}, nil)
		Expect(err).To(MatchError(ContainSubstring("connection refused")))
	})

	It("rejects nil targets and writers", func() {
		st, err := NewSharkTankOnHost("localhost:1", nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(st.Capture(io.Discard, nil, nil)).Error().To(
			MatchError(ContainSubstring("no capture target")))
		Expect(st.Capture(nil, &api.Target{Name: "foo"}, nil)).Error().To(
			MatchError(ContainSubstring("no capture writer")))
	})

	DescribeTable("defaults to the http scheme",
		func(hosturl, expected string) {
			st, err := NewSharkTankOnHost(hosturl, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(st.(*hostsharktank).hosturl.String()).To(Equal(expected))
		},
		Entry("host only", "foo", "http://foo"),
		Entry("host and port", "foo:5001", "http://foo:5001"),
		Entry("host named http", "http:5001", "http://http:5001"),
		Entry("http", "http://foo:5001", "http://foo:5001"),
		Entry("https", "https://foo:5001", "https://foo:5001"),
	)

})
