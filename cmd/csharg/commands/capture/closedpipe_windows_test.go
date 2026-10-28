// (c) Siemens AG 2026
//
// SPDX-License-Identifier: MIT

//go:build windows

package capture

import (
	"errors"
	"fmt"
	"os"
	"syscall"

	"golang.org/x/sys/windows"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("closed pipes on Windows", func() {

	DescribeTable("are recognized",
		func(errno syscall.Errno) {
			err := fmt.Errorf("capture stream writer failure: %w",
				&os.PathError{Op: "write", Path: "/dev/stdout", Err: errno})
			Expect(isClosedPipe(err)).To(BeTrue())
		},
		Entry("ERROR_BROKEN_PIPE", windows.ERROR_BROKEN_PIPE),
		Entry("ERROR_NO_DATA", windows.ERROR_NO_DATA),
	)

	It("doesn't mistake other errors", func() {
		Expect(isClosedPipe(errors.New("fooled"))).To(BeFalse())
		Expect(isClosedPipe(windows.ERROR_ACCESS_DENIED)).To(BeFalse())
	})

})
