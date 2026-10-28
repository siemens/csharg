// (c) Siemens AG 2023
//
// SPDX-License-Identifier: MIT

//go:build !windows

package pipe

import (
	"fmt"
	"log/slog"
	"os"
	"unsafe"

	"golang.org/x/sys/unix"
)

// fdSetSize is FD_SETSIZE, the number of fds an [unix.FdSet] can hold.
const fdSetSize = int(unsafe.Sizeof(unix.FdSet{})) * 8

// WaitTillBreak continuously checks a fifo/pipe to see when it breaks. When
// called, WaitTillBreak blocks until the fifo/pipe finally has broken.
//
// This implementation leverages [unix.Select] and thus panics if the fifo's fd
// is invalid or not below FD_SETSIZE. As a standalone capture client command
// only has very few fds open, the fifo's fd can safely be assumed to be well
// below FD_SETSIZE.
func WaitTillBreak(fifo *os.File) {
	slog.Debug("constantly monitoring packet capture fifo status...")
	fd := int(fifo.Fd())
	if fd < 0 || fd >= fdSetSize {
		panic(fmt.Sprintf("cannot monitor fifo %q: fd %d outside select() range 0..%d (FD_SETSIZE)",
			fifo.Name(), fd, fdSetSize-1))
	}
	for {
		// Check the fifo becoming readable, which signals that it has been
		// closed. In this case, ex-termi-nate ;) Oh, and remember to correctly
		// initialize the fdset each time before calling select() ... well, just
		// because that's a good idea to do. :(
		fds := unix.FdSet{}
		fds.Set(fd)
		n, err := unix.Select(
			fd+1,           // highest fd is our file descriptor.
			&fds, nil, nil, // only watch readable.
			nil, // no timeout, ever.
		)
		if err == unix.EINTR {
			// Interrupted by a signal, so keep waiting.
			continue
		}
		if n != 0 || err != nil {
			// Either the pipe was broken by Wireshark, or we did break it on
			// purpose in the piping process. Anyway, we're done.
			slog.Debug("capture fifo broken, stopped monitoring.")
			return
		}
	}
}
