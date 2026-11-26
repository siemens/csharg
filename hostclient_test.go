// (c) Siemens AG 2026
//
// SPDX-License-Identifier: MIT

package csharg

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/thediveo/safe"

	"github.com/siemens/csharg/v2/api"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/thediveo/success"
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
		var serr *StatusError
		Expect(errors.As(err, &serr)).To(BeTrue())
		Expect(serr.StatusCode).To(Equal(http.StatusUnauthorized))
		Expect(ts).To(BeEmpty())
		Expect(st.(*hostsharktank).cache.IsEmpty()).To(BeTrue())
	})

	It("reports the status of rejected capture handshakes", func() {
		DeferCleanup(slog.SetDefault, slog.Default())
		slog.SetDefault(slog.New(slog.NewTextHandler(GinkgoWriter, nil)))
		srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			rw.WriteHeader(http.StatusUnauthorized)
		}))
		DeferCleanup(srv.Close)

		st, err := NewSharkTankOnHost(srv.URL, nil)
		Expect(err).NotTo(HaveOccurred())
		_, err = st.Capture(io.Discard, &api.Target{
			Name:              "foo",
			NetworkInterfaces: []string{"eth0"},
		}, nil)
		Expect(err).To(MatchError(ContainSubstring("401 Unauthorized")))
		Expect(err).To(MatchError(websocket.ErrBadHandshake))
		var serr *StatusError
		Expect(errors.As(err, &serr)).To(BeTrue())
		Expect(serr.StatusCode).To(Equal(http.StatusUnauthorized))
	})

	It("doesn't log rejected capture handshakes as errors", func() {
		// Matching the plain text output is sufficient, as only the level of a
		// single fixed message matters. It's also stable: the stdlib text
		// handler always writes the level and message as the "level=..."
		// and "msg=..." keys directly following "time=...", with the level in
		// its canonical upper case form, such as "DEBUG" and "ERROR".
		var logs safe.Buffer
		DeferCleanup(slog.SetDefault, slog.Default())
		slog.SetDefault(slog.New(slog.NewTextHandler(&logs,
			&slog.HandlerOptions{Level: slog.LevelDebug})))
		srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			rw.WriteHeader(http.StatusUnauthorized)
		}))
		DeferCleanup(srv.Close)

		st := Successful(NewSharkTankOnHost(srv.URL, nil))
		_, err := st.Capture(io.Discard, &api.Target{
			Name:              "foo",
			NetworkInterfaces: []string{"eth0"},
		}, nil)
		var serr *StatusError
		Expect(errors.As(err, &serr)).To(BeTrue())
		Expect(serr.StatusCode).To(Equal(http.StatusUnauthorized))

		Expect(logs.String()).To(SatisfyAll(
			ContainSubstring(`level=DEBUG msg="cannot contact capture service via websocket"`),
			Not(ContainSubstring("level=ERROR"))))
	})

	// discoverTargets explicitly discovers the capture targets of st.
	discoverTargets := func(st SharkTank) error {
		_, err := st.Targets()
		return err
	}
	// captureDiscovered captures from a target lacking network interfaces, so
	// that st needs to discover the capture targets first.
	captureDiscovered := func(st SharkTank) error {
		_, err := st.Capture(io.Discard, &api.Target{Name: "foo"}, nil)
		return err
	}
	// captureHandshake captures from a target with network interfaces, so that
	// st immediately attempts the capture WebSocket handshake; any log output
	// goes to the GinkgoWriter.
	captureHandshake := func(st SharkTank) error {
		DeferCleanup(slog.SetDefault, slog.Default())
		slog.SetDefault(slog.New(slog.NewTextHandler(GinkgoWriter, nil)))
		_, err := st.Capture(io.Discard, &api.Target{Name: "foo", NetworkInterfaces: []string{"eth0"}}, nil)
		return err
	}

	DescribeTable("reports rejections together with their response headers",
		func(op func(SharkTank) error, wwwauth string) {
			srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
				rw.Header().Set("X-Rejected-By", "test")
				if wwwauth != "" {
					rw.Header().Set("WWW-Authenticate", wwwauth)
				}
				rw.WriteHeader(http.StatusUnauthorized)
			}))
			DeferCleanup(srv.Close)

			err := op(Successful(NewSharkTankOnHost(srv.URL, nil)))
			var serr *StatusError
			Expect(errors.As(err, &serr)).To(BeTrue())
			Expect(serr.StatusCode).To(Equal(http.StatusUnauthorized))
			Expect(serr.Header).To(HaveKeyWithValue("X-Rejected-By", []string{"test"}))
			if wwwauth == "" {
				Expect(serr.Header).NotTo(HaveKey("Www-Authenticate"))
			} else {
				Expect(serr.Header).To(HaveKeyWithValue("Www-Authenticate", []string{wwwauth}))
			}
		},
		Entry("discovery without WWW-Authenticate", discoverTargets, ""),
		Entry("discovery with WWW-Authenticate", discoverTargets, `Bearer realm="test"`),
		Entry("capture discovery without WWW-Authenticate", captureDiscovered, ""),
		Entry("capture discovery with WWW-Authenticate", captureDiscovered, `Bearer realm="test"`),
		Entry("handshake without WWW-Authenticate", captureHandshake, ""),
		Entry("handshake with WWW-Authenticate", captureHandshake, `Bearer realm="test"`),
	)

	When("sending additional request headers", func() {

		// newHeaderServer returns a capture service URL, as well as channels
		// receiving the request headers of discoveries and capture handshakes.
		newHeaderServer := func() (string, chan http.Header, chan http.Header) {
			discoveredCh := make(chan http.Header, 1)
			capturedCh := make(chan http.Header, 1)
			upgrader := websocket.Upgrader{}
			mux := http.NewServeMux()
			mux.HandleFunc("/discover/mobyshark", func(rw http.ResponseWriter, r *http.Request) {
				discoveredCh <- r.Header.Clone()
				_, _ = io.WriteString(rw,
					`{"containers":[{"name":"foo","type":"docker","network-interfaces":["eth0","lo"]}]}`)
			})
			mux.HandleFunc("/capture", func(rw http.ResponseWriter, r *http.Request) {
				capturedCh <- r.Header.Clone()
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
			return srv.URL, discoveredCh, capturedCh
		}

		It("sends them with discoveries and capture handshakes", func(ctx context.Context) {
			url, discoveredCh, capturedCh := newHeaderServer()
			st := Successful(NewSharkTankOnHost(url, &SharkTankOnHostOptions{
				CommonClientOptions: CommonClientOptions{
					Header: http.Header{"Cookie": {"authToken=x"}},
				},
			}))
			cs := Successful(st.Capture(io.Discard, &api.Target{Name: "foo", NodeName: "127.0.0.1"}, nil))
			DeferCleanup(cs.Stop)

			Eventually(ctx, discoveredCh).Should(Receive(HaveKeyWithValue("Cookie", []string{"authToken=x"})))
			Eventually(ctx, capturedCh).Should(Receive(HaveKeyWithValue("Cookie", []string{"authToken=x"})))
		}, SpecTimeout(10*time.Second))

		It("doesn't let them override the bearer token and capture service headers", func(ctx context.Context) {
			url, discoveredCh, capturedCh := newHeaderServer()
			st := Successful(NewSharkTankOnHost(url, &SharkTankOnHostOptions{
				CommonClientOptions: CommonClientOptions{
					BearerToken: "good",
					Header: http.Header{
						"Authorization":       {"Bearer evil"},
						"clustershark-nif":    {"evil"},
						"Clustershark-Filter": {"evil"},
					},
				},
			}))
			cs := Successful(st.Capture(io.Discard, &api.Target{Name: "foo", NodeName: "127.0.0.1"}, nil))
			DeferCleanup(cs.Stop)

			Eventually(ctx, discoveredCh).Should(Receive(
				HaveKeyWithValue("Authorization", []string{"Bearer good"})))
			Eventually(ctx, capturedCh).Should(Receive(SatisfyAll(
				HaveKeyWithValue("Authorization", []string{"Bearer good"}),
				HaveKeyWithValue("Clustershark-Nif", []string{"eth0/lo"}),
				Not(HaveKey("Clustershark-Filter")),
			)))
		}, SpecTimeout(10*time.Second))

		It("isn't affected by later changes to the caller's headers", func(ctx context.Context) {
			url, discoveredCh, capturedCh := newHeaderServer()
			header := http.Header{"Cookie": {"authToken=x"}}
			st := Successful(NewSharkTankOnHost(url, &SharkTankOnHostOptions{
				CommonClientOptions: CommonClientOptions{Header: header},
			}))
			header["Cookie"][0] = "authToken=y"
			header.Set("X-Foo", "bar")

			cs := Successful(st.Capture(io.Discard, &api.Target{Name: "foo", NodeName: "127.0.0.1"}, nil))
			DeferCleanup(cs.Stop)

			expected := SatisfyAll(
				HaveKeyWithValue("Cookie", []string{"authToken=x"}),
				Not(HaveKey("X-Foo")))
			Eventually(ctx, discoveredCh).Should(Receive(expected))
			Eventually(ctx, capturedCh).Should(Receive(expected))
		}, SpecTimeout(10*time.Second))

		DescribeTable("rejects headers reserved for the WebSocket handshake",
			func(key string) {
				Expect(NewSharkTankOnHost("localhost:1", &SharkTankOnHostOptions{
					CommonClientOptions: CommonClientOptions{
						Header: http.Header{key: {"foo"}},
					},
				})).Error().To(MatchError(ContainSubstring("reserved for the WebSocket handshake")))
			},
			Entry(nil, "Upgrade"),
			Entry(nil, "connection"),
			Entry(nil, "Sec-WebSocket-Key"),
			Entry(nil, "Sec-WebSocket-Protocol"),
		)

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
