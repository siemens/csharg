// (c) Siemens AG 2026
//
// SPDX-License-Identifier: MIT

package csharg

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"syscall"
	"time"

	"github.com/gorilla/websocket"

	"github.com/siemens/csharg/v2/api"

	"github.com/onsi/gomega/types"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// dialServer connects to a test websocket server serving each connection using
// serve.
func dialServer(ctx context.Context, serve func(conn *websocket.Conn)) *websocket.Conn {
	GinkgoHelper()
	upgrader := websocket.Upgrader{}
	srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(rw, r, nil)
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		serve(conn)
	}))
	DeferCleanup(srv.Close)
	conn, _, err := websocket.DefaultDialer.DialContext(ctx,
		"ws"+strings.TrimPrefix(srv.URL, "http"), nil)
	Expect(err).NotTo(HaveOccurred())
	return conn
}

// drain reads from the websocket until it fails, so that it answers close
// handshakes.
func drain(conn *websocket.Conn) {
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			return
		}
	}
}

// dialCapture connects to a test websocket server that sends a single packet
// data message and then keeps reading, so that it answers close handshakes.
func dialCapture(ctx context.Context) *websocket.Conn {
	GinkgoHelper()
	return dialServer(ctx, func(conn *websocket.Conn) {
		// Any payload works: the pcapng stream editor buffers this incomplete
		// SHB, yet still calls the writer, merely with zero octets.
		_ = conn.WriteMessage(websocket.BinaryMessage, []byte("packet"))
		drain(conn)
	})
}

// startCapture starts a capture stream from a test websocket server.
func startCapture(ctx context.Context, w func([]byte) (int, error), opts *CaptureOptions) CaptureStreamer {
	GinkgoHelper()
	cs, err := StartCaptureStream(writerFunc(w), dialCapture(ctx), &api.Target{}, opts)
	Expect(err).NotTo(HaveOccurred())
	return cs
}

type writerFunc func([]byte) (int, error)

func (f writerFunc) Write(b []byte) (int, error) { return f(b) }

var _ = Describe("capture streamer", func() {

	It("stops only after the stream writer is done", func(ctx context.Context) {
		writing := make(chan struct{}, 1)
		release := make(chan struct{})
		DeferCleanup(func() {
			select {
			case <-release:
			default:
				close(release)
			}
		})
		cs := startCapture(ctx, func(b []byte) (int, error) {
			select {
			case writing <- struct{}{}:
			default:
			}
			<-release
			return len(b), nil
		}, &CaptureOptions{})
		Eventually(writing).Should(Receive())

		stopped := make(chan struct{})
		var stopErr error
		go func() {
			defer close(stopped)
			stopErr = cs.Stop()
		}()
		Consistently(stopped).WithTimeout(250 * time.Millisecond).ShouldNot(BeClosed())
		close(release)
		Eventually(stopped).WithTimeout(2 * time.Second).Should(BeClosed())
		Expect(stopErr).To(Succeed())

		Expect(cs.Stop()).To(Succeed())
	}, SpecTimeout(10*time.Second))

	It("closes the websocket promptly after a writer failure", func(ctx context.Context) {
		cs := startCapture(ctx, func([]byte) (int, error) {
			return 0, syscall.EPIPE
		}, &CaptureOptions{})

		ended := make(chan struct{})
		var waitErr error
		go func() {
			defer close(ended)
			waitErr = cs.Wait()
		}()
		Eventually(ended).WithTimeout(2 * time.Second).Should(BeClosed())
		Expect(waitErr).To(MatchError(syscall.EPIPE))

		start := time.Now()
		Expect(cs.Stop()).To(MatchError(syscall.EPIPE))
		Expect(time.Since(start)).To(BeNumerically("<", time.Second))
	}, SpecTimeout(10*time.Second))

	It("streams with nil options", func(ctx context.Context) {
		writing := make(chan struct{}, 1)
		cs := startCapture(ctx, func(b []byte) (int, error) {
			select {
			case writing <- struct{}{}:
			default:
			}
			return len(b), nil
		}, nil)
		Eventually(writing).Should(Receive())
		Expect(cs.Stop()).To(Succeed())
	}, SpecTimeout(10*time.Second))

	It("stops after a duration without error", func(ctx context.Context) {
		cs := startCapture(ctx, func(b []byte) (int, error) { return len(b), nil }, nil)
		Expect(cs.StopAfter(100 * time.Millisecond)).To(Succeed())
	}, SpecTimeout(10*time.Second))

	DescribeTable("reports why a capture stream ended by itself",
		func(ctx context.Context, serve func(conn *websocket.Conn), expected types.GomegaMatcher) {
			cs, err := StartCaptureStream(io.Discard, dialServer(ctx, serve), &api.Target{}, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(cs.Wait()).To(expected)
			Expect(cs.StopAfter(time.Hour)).To(expected)
		},
		Entry("normal close", func(conn *websocket.Conn) {
			_ = conn.WriteMessage(websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			drain(conn)
		}, Succeed()),
		Entry("going away", func(conn *websocket.Conn) {
			_ = conn.WriteMessage(websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseGoingAway, ""))
			drain(conn)
		}, Succeed()),
		Entry("error close", func(conn *websocket.Conn) {
			_ = conn.WriteMessage(websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseInternalServerErr, "dumpcap failed"))
			drain(conn)
		}, MatchError(ContainSubstring("dumpcap failed"))),
		Entry("dropped connection", func(conn *websocket.Conn) {
			_ = conn.UnderlyingConn().Close()
		}, MatchError(ContainSubstring("capture stream broken"))),
	)

	It("rejects a nil websocket", func() {
		Expect(StartCaptureStream(io.Discard, nil, &api.Target{}, nil)).Error().To(
			MatchError(ContainSubstring("no websocket")))
	})

	DescribeTable("rejects nil arguments and closes the websocket",
		func(ctx context.Context, w io.Writer, t *api.Target, errtext string) {
			conn := dialCapture(ctx)
			Expect(StartCaptureStream(w, conn, t, nil)).Error().To(
				MatchError(ContainSubstring(errtext)))
			Expect(conn.WriteMessage(websocket.TextMessage, nil)).NotTo(Succeed())
		},
		Entry("nil writer", nil, &api.Target{}, "no capture writer"),
		Entry("nil target", io.Discard, nil, "no capture target"),
	)

})

var _ = Describe("capture service request parameters", func() {

	It("rejects nil targets", func() {
		Expect(CaptureServiceHeaders(nil, nil)).Error().To(HaveOccurred())
		Expect(CaptureServiceQueryParams(nil, nil)).Error().To(HaveOccurred())
	})

	It("defaults nil options", func() {
		t := &api.Target{Name: "foo", NetworkInterfaces: []string{"eth0"}}
		h, err := CaptureServiceHeaders(t, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(h.Get("Clustershark-Nif")).To(Equal("eth0"))
		q, err := CaptureServiceQueryParams(t, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(q.Get("nif")).To(Equal("eth0"))
	})

})

var _ = Describe("completing capture targets", func() {

	var tc *TargetCache

	BeforeEach(func() {
		tc = &TargetCache{}
		tc.Set(api.Targets{{
			Name: "foo", Type: "docker", NodeName: "node1",
			NetworkInterfaces: []string{"eth0", "lo"},
		}})
	})

	It("rejects nil targets and caches", func() {
		Expect(CompleteTarget(nil, nil, tc)).Error().To(HaveOccurred())
		Expect(CompleteTarget(&api.Target{Name: "foo", NodeName: "node1"}, nil, nil)).Error().To(
			MatchError(ContainSubstring("no capture target cache")))
	})

	It("uses the discovered network interfaces", func() {
		t, err := CompleteTarget(&api.Target{Name: "foo", NodeName: "node1"}, nil, tc)
		Expect(err).NotTo(HaveOccurred())
		Expect(t.NetworkInterfaces).To(ConsistOf("eth0", "lo"))
	})

	It("prefers the network interfaces from the options", func() {
		opts := &CaptureOptions{Nifs: Nifs{"eth0"}}
		t, err := CompleteTarget(&api.Target{Name: "foo", NodeName: "node1"}, opts, tc)
		Expect(err).NotTo(HaveOccurred())
		Expect(t.NetworkInterfaces).To(ConsistOf("eth0"))
		opts.Nifs[0] = "fooled"
		Expect(t.NetworkInterfaces).To(ConsistOf("eth0"))
	})

	It("doesn't modify an already complete target", func() {
		orig := &api.Target{Name: "foo", CaptureService: "svc", NetworkInterfaces: []string{"lo"}}
		t, err := CompleteTarget(orig, &CaptureOptions{Nifs: Nifs{"eth0"}}, tc)
		Expect(err).NotTo(HaveOccurred())
		Expect(t.NetworkInterfaces).To(ConsistOf("eth0"))
		Expect(orig.NetworkInterfaces).To(ConsistOf("lo"))

		t, err = CompleteTarget(orig, &CaptureOptions{}, tc)
		Expect(err).NotTo(HaveOccurred())
		Expect(t).To(BeIdenticalTo(orig))
	})

})
