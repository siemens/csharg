// (c) Siemens AG 2026
//
// SPDX-License-Identifier: MIT

package websock

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"github.com/gorilla/websocket"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// newServer returns a test websocket server running serve on each upgraded
// connection, together with the matching ws:// URL.
func newServer(serve func(*websocket.Conn)) (*httptest.Server, string) {
	upgrader := websocket.Upgrader{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		serve(conn)
	}))
	DeferCleanup(srv.Close)
	return srv, "ws" + strings.TrimPrefix(srv.URL, "http")
}

func dial(ctx context.Context, url string) *ReadingClientWebsocket {
	GinkgoHelper()
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, url, nil)
	Expect(err).NotTo(HaveOccurred())
	return New(conn)
}

var _ = Describe("reading client websocket", func() {

	It("handles a server-initiated close and repeated reads", func(ctx context.Context) {
		_, url := newServer(func(conn *websocket.Conn) {
			_ = conn.WriteMessage(websocket.BinaryMessage, []byte("foo"))
			_ = conn.WriteMessage(websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseNormalClosure, "bye"))
			for {
				if _, _, err := conn.ReadMessage(); err != nil {
					return
				}
			}
		})
		ws := dial(ctx, url)

		Expect(ws.Read()).To(Equal([]byte("foo")))

		_, err := ws.Read()
		var cerr *websocket.CloseError
		Expect(err).To(BeAssignableToTypeOf(cerr))
		Expect(err.(*websocket.CloseError).Code).To(Equal(websocket.CloseNormalClosure))

		Expect(func() { _, _ = ws.Read() }).NotTo(Panic())

		start := time.Now()
		ws.Close()
		Expect(time.Since(start)).To(BeNumerically("<", time.Second))
	}, SpecTimeout(10*time.Second))

	It("force-closes after the timeout and then tolerates further reads and closes", func(ctx context.Context) {
		DeferCleanup(func(old time.Duration) { closeTimeout = old }, closeTimeout)
		closeTimeout = 100 * time.Millisecond

		release := make(chan struct{})
		_, url := newServer(func(*websocket.Conn) { <-release })
		DeferCleanup(func() { close(release) })
		ws := dial(ctx, url)

		start := time.Now()
		ws.Close()
		Expect(time.Since(start)).To(BeNumerically(">=", closeTimeout))

		var err error
		Expect(func() { _, err = ws.Read() }).NotTo(Panic())
		Expect(err).To(HaveOccurred())

		start = time.Now()
		Expect(ws.Close).NotTo(Panic())
		Expect(time.Since(start)).To(BeNumerically("<", closeTimeout))
	}, SpecTimeout(10*time.Second))

	It("closes promptly after a broken connection", func(ctx context.Context) {
		release := make(chan struct{})
		_, url := newServer(func(*websocket.Conn) { <-release })
		DeferCleanup(func() { close(release) })
		ws := dial(ctx, url)

		Expect(ws.SetReadDeadline(time.Now())).To(Succeed())
		_, err := ws.Read()
		Expect(err).To(HaveOccurred())
		Expect(err).NotTo(BeAssignableToTypeOf(&websocket.CloseError{}))

		start := time.Now()
		ws.Close()
		Expect(time.Since(start)).To(BeNumerically("<", time.Second))
	}, SpecTimeout(10*time.Second))

})
