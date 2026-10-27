// (c) Siemens AG 2026
//
// SPDX-License-Identifier: MIT

//go:build !windows

package capture

import (
	"errors"
	"syscall"
)

// isClosedPipe reports whether err stems from the packet consumer having
// closed its end of the pipe.
func isClosedPipe(err error) bool {
	return errors.Is(err, syscall.EPIPE)
}
