// (c) Siemens AG 2026
//
// SPDX-License-Identifier: MIT

package semver

import (
	"fmt"

	"github.com/thediveo/go-plugger/v3"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("semantic version", func() {

	BeforeEach(func() {
		grp := plugger.Group[Override]()
		DeferCleanup(grp.Restore, grp.Backup())
		grp.Clear()
	})

	DescribeTable("resolving the version",
		func(expected string, overrides ...string) {
			for idx, override := range overrides {
				plugger.Group[Override]().Register(func() string {
					return override
				}, plugger.WithPlugin(fmt.Sprintf("plugin-%d", idx)))
			}
			Expect(Resolve("1.0.0")).To(Equal(expected))
		},
		Entry("preset without overrides", "1.0.0"),
		Entry("preset with only empty overrides", "1.0.0", "", ""),
		Entry("first non-empty override", "2.0.0", "", "2.0.0", "3.0.0"),
	)

})
