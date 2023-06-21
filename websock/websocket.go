// (c) Siemens AG 2023
//
// SPDX-License-Identifier: MIT

package websock

import (
	"fmt"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
)

// ReadingClientWebsocket represents a websocket for reading, with
// graceful handling of the closing procedure.
type ReadingClientWebsocket struct {
	*websocket.Conn
	Closing bool       // Are we in the process of gracefully closing?
	m       sync.Mutex // Synchronize access to this websocket's state.
	// Signals that the websocket is closed, by closing (sic!)
	// this channel.
	closed chan bool
}

// New returns an enhanced gorilla websocket that does graceful close handling.
func New(ws *websocket.Conn) *ReadingClientWebsocket {
	return &ReadingClientWebsocket{
		Conn:   ws,
		closed: make(chan bool),
	}
}

// Read reads more (binary) data from a websocket. It correctly handles
// gracefully closing the websocket when the peer (server) signals to do
// so. The client can trigger a close itself using the Close() method. When
// the websocket has been gracefully closed, this Read() returns a
// websocket.CloseError with the peer's (server's) close code and text.
func (ws *ReadingClientWebsocket) Read() (data []byte, err error) {
	for {
		msgType, data, err := ws.Conn.ReadMessage()
		if err == nil {
			if msgType == websocket.BinaryMessage {
				return data, err
			}
			return nil, fmt.Errorf("unexpected websocket text message received")
		}
		// Check if we got a close "error" or some other error: all non-close error
		// get reported immediately, otherwise, for close errors we need to do some
		// checks and handling to correctly carry out the graceful close procedure.
		cerr, ok := err.(*websocket.CloseError)
		if !ok {
			return nil, err
		}
		// So we got a websocket close control message. If the peer sent it in
		// response to us sending it a close control message beforehand, then we
		// need to respond with our close control message to acknowledge the
		// close gracefully. Otherwise, we started the close war, so we can now
		// finally close the connection, because both sides are done.
		ws.m.Lock()
		defer ws.m.Unlock()
		if !ws.Closing {
			// The peer (server) is closing first, so we need to ack, and then
			// are done with this connection either.
			ws.Closing = true
			log.Debug("server closes websocket, acknowledging close")
			ws.Conn.WriteMessage(
				websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseNormalClosure, "ciao"))
		} else {
			log.Debug("server acknowledged websocket close")
		}
		ws.Conn.Close()
		close(ws.closed) // sic(k)!
		return nil, cerr
	}
}

// Close gracefully closes this client websocket and waits for the close
// to complete. The waiting is time limited, though, so a non-responsive
// websocket peer (server) won't block us here forever: instead, after
// a "graceful" timeout, we will close the underlaying transport connection
// in any case. So, this Close() operation has an upper bound on its
// execution time -- which is set to 10s.
func (ws *ReadingClientWebsocket) Close() {
	ws.m.Lock()
	func() { // locked section
		defer ws.m.Unlock()
		// We should not send a close control message when we're already gracefully closing
		// the connection; regardless of whether already we or the peer (server) started
		// the close (in progress or already done).
		if !ws.Closing {
			ws.Closing = true
			log.Debug("initiating graceful websocket close")
			ws.Conn.WriteMessage(
				websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseNormalClosure, "ciao"))
		}
	}()
	log.Debug("waiting for graceful close to be finished...")
	select {
	case <-time.After(10 * time.Second):
		// Force the underlaying transport connection to close anyway in
		// case the peer (server) hangs, not proceeding in the graceful
		// websocket close.
		log.Debug("graceful websocket close timeout; forced closed")
		ws.Conn.Close()
		close(ws.closed)
	case <-ws.closed:
		// Done: either just gracefully closed or already closed.
		break
	}
	log.Debug("websocket gracefully closed.")
}
