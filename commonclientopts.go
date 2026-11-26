// (c) Siemens AG 2023
//
// SPDX-License-Identifier: MIT

// Defines the options common to all cluster capture client types -- not that
// there are that many, but this way we make explicit which options are common
// to URL-based and remote-API-based clients.

package csharg

import (
	"net/http"
	"time"
)

// CommonClientOptions defines options common to all cluster capture client
// types.
type CommonClientOptions struct {
	// BearerToken optionally specifies the bearer token to use when talking to
	// the cluster capture service, regardless of how we reach the service.
	BearerToken string
	// Header optionally specifies additional HTTP request headers to send to the
	// capture service, both for target discovery and the capture WebSocket
	// handshake; for instance, "Cookie: authToken=...".
	//
	// The client keeps its own copy of Header, so later changes to the caller's
	// map have no effect; to use different headers, such as after a new login,
	// create a new client. Headers managed by the WebSocket handshake itself
	// ("Upgrade", "Connection", and "Sec-Websocket-*") are rejected when
	// creating the client. The capture service headers ("Clustershark-*") and
	// the "Authorization" header derived from a non-empty BearerToken always
	// take precedence over the same headers in Header.
	//
	// Header values are never logged, as they usually carry credentials.
	Header http.Header
	// Timeout specifies a time limit for requests made to the SharkTank cluster
	// capture service. For discovery it limits the time allowed to complete a
	// discovery request and response. For capturing it limits just the
	// connection establishing phase, including the web socket handshake phase.
	Timeout time.Duration
}
