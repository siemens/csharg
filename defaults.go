// (c) Siemens AG 2023
//
// SPDX-License-Identifier: MIT

package csharg

import "time"

const (
	// DefaultServiceTimeout specifies the time limit for completing discovery
	// service calls and for establishing a stream connection to the capture
	// service.
	DefaultServiceTimeout = 30 * time.Second
)
