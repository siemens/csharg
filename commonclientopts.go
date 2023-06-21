// (c) Siemens AG 2023
//
// SPDX-License-Identifier: MIT

// Defines the options common to all cluster capture client types -- not that
// there are that many, but this way we make explicit which options are common
// to URL-based and remote-API-based clients.

package csharg

import "time"

// CommonClientOptions defines options common to all cluster capture client
// types.
type CommonClientOptions struct {
	// BearerToken optionally specifies the bearer token to use when talking to
	// the cluster capture service, regardless of how we reach the service.
	BearerToken string
	// Timeout specifies a time limit for requests made to the SharkTank cluster
	// capture service. For discovery it limits the time allowed to complete a
	// discovery request and response. For capturing it limits just the
	// connection establishing phase, including the web socket handshake phase.
	Timeout time.Duration
}
