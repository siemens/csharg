// (c) Siemens AG 2026
//
// SPDX-License-Identifier: MIT

//go:build windows

package capture

import (
	"errors"
	"syscall"

	"golang.org/x/sys/windows"
)

// isClosedPipe reports whether err stems from the packet consumer having
// closed its end of the pipe.
func isClosedPipe(err error) bool {
	return errors.Is(err, syscall.EPIPE) ||
		errors.Is(err, windows.ERROR_BROKEN_PIPE) ||
		errors.Is(err, windows.ERROR_NO_DATA)
}
