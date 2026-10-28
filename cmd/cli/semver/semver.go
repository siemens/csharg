// (c) Siemens AG 2023
//
// SPDX-License-Identifier: MIT

package semver

import (
	"github.com/thediveo/go-plugger/v3"
)

// Override defines an exposed plugin symbol type for returning (overriding) the
// CLI binary's semantic version. The first plugin returning a non-empty string
// will win.
type Override func() string

// Resolve returns the semantic version string returned by the first registered
// plugin that is not empty, otherwise the specified preset.
func Resolve(preset string) string {
	for _, override := range plugger.Group[Override]().Symbols() {
		if sv := override(); sv != "" {
			return sv
		}
	}
	return preset
}
