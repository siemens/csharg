// (c) Siemens AG 2026
//
// SPDX-License-Identifier: MIT

package examples

import (
	"fmt"

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

	DescribeTable("collecting examples for a command",
		func(command string, expected string, plugins ...ForCommands) {
			for idx, examples := range plugins {
				plugger.Group[Illustrate]().Register(func() ForCommands {
					return examples
				}, plugger.WithPlugin(fmt.Sprintf("plugin-%d", idx)))
			}
			Expect(For(command)).To(Equal(expected))
		},
		Entry("ignores empty examples", "foo", "",
			ForCommands{"foo": "\n", "bar": "gnampf"}),
		Entry("ignores other examples", "bar", "",
			ForCommands{"foo": "\n"}),
		Entry("augments examples", "foo", "example-1\n\nexample-2",
			ForCommands{"foo": "example-1"},
			ForCommands{"foo": "example-2"}),
	)

})
