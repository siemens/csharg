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
		grp := plugger.Group[CommandExamples]()
		DeferCleanup(grp.Restore, grp.Backup())
		grp.Clear()
	})

	It("ignores empty examples", func() {
		plugger.Group[CommandExamples]().Register(func() Examples {
			return Examples{
				"foo": "\n",
				"bar": "gnampf",
			}
		}, plugger.WithPlugin("foo"))
		Expect(ExamplesFor("foo")).To(BeEmpty())
	})

	It("ignores other examples", func() {
		plugger.Group[CommandExamples]().Register(func() Examples {
			return Examples{"foo": "\n"}
		}, plugger.WithPlugin("foo"))
		Expect(ExamplesFor("bar")).To(BeEmpty())
	})

	It("augments examples", func() {
		plugger.Group[CommandExamples]().Register(func() Examples {
			return Examples{"foo": "example-1"}
		}, plugger.WithPlugin("foo1"))
		plugger.Group[CommandExamples]().Register(func() Examples {
			return Examples{"foo": "example-2"}
		}, plugger.WithPlugin("foo2"))
		Expect(ExamplesFor("foo")).To(Equal("example-1\n\nexample-2"))
	})

})
