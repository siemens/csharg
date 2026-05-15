// (c) Siemens AG 2026
//
// SPDX-License-Identifier: MIT

// Sets up the test suite for unit testing our ClusterShark external capture
// plugin.

package cli

import (
	"github.com/thediveo/go-plugger/v3"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("command examples", func() {

	BeforeEach(func() {
		grp := plugger.Group[CommandExamples]()
		DeferCleanup(grp.Restore, grp.Backup())
		grp.Clear()
	})

	It("ignores empty examples", func() {
		plugger.Group[CommandExamples]().Register(func() (command string, example string) {
			return "foo", "\n"
		}, plugger.WithPlugin("foo"))
		Expect(Examples("foo")).To(BeEmpty())
	})

	It("ignores other examples", func() {
		plugger.Group[CommandExamples]().Register(func() (command string, example string) {
			return "foo", "\n"
		}, plugger.WithPlugin("foo"))
		Expect(Examples("bar")).To(BeEmpty())
	})

	It("augments examples", func() {
		plugger.Group[CommandExamples]().Register(func() (command string, example string) {
			return "foo", "example-1"
		}, plugger.WithPlugin("foo1"))
		plugger.Group[CommandExamples]().Register(func() (command string, example string) {
			return "foo", "example-2"
		}, plugger.WithPlugin("foo2"))
		Expect(Examples("foo")).To(Equal("example-1\n\nexample-2"))
	})

})
