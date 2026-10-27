// (c) Siemens AG 2023
//
// SPDX-License-Identifier: MIT

//go:build !windows

package pipe

import (
	"os"

	"golang.org/x/sys/unix"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// pipeWithReadFdAtLeast returns a pipe whose read end has a file descriptor
// number of at least minfd.
func pipeWithReadFdAtLeast(minfd int) (*os.File, *os.File) {
	GinkgoHelper()
	lowr, w, err := os.Pipe()
	Expect(err).NotTo(HaveOccurred())
	defer func() { _ = lowr.Close() }()
	// F_DUPFD picks the lowest free fd >= minfd, so unlike dup2 it never
	// clobbers an fd already in use.
	highfd, err := unix.FcntlInt(lowr.Fd(), unix.F_DUPFD_CLOEXEC, minfd)
	Expect(err).NotTo(HaveOccurred())
	return os.NewFile(uintptr(highfd), "pipe-read"), w
}
