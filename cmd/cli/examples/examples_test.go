// (c) Siemens AG 2026
//
// SPDX-License-Identifier: MIT

package examples

import (
	"github.com/thediveo/go-plugger/v3"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("command examples", func() {

	BeforeEach(func() {
		grp := plugger.Group[Illustrate]()
		DeferCleanup(grp.Restore, grp.Backup())
		grp.Clear()
	})

	It("ignores empty examples", func() {
		plugger.Group[Illustrate]().Register(func() ForCommands {
			return ForCommands{
				"foo": "\n",
				"bar": "gnampf",
			}
		}, plugger.WithPlugin("foo"))
		Expect(For("foo")).To(BeEmpty())
	})

	It("ignores other examples", func() {
		plugger.Group[Illustrate]().Register(func() ForCommands {
			return ForCommands{"foo": "\n"}
		}, plugger.WithPlugin("foo"))
		Expect(For("bar")).To(BeEmpty())
	})

	It("augments examples", func() {
		plugger.Group[Illustrate]().Register(func() ForCommands {
			return ForCommands{"foo": "example-1"}
		}, plugger.WithPlugin("foo1"))
		plugger.Group[Illustrate]().Register(func() ForCommands {
			return ForCommands{"foo": "example-2"}
		}, plugger.WithPlugin("foo2"))
		Expect(For("foo")).To(Equal("example-1\n\nexample-2"))
	})

})
