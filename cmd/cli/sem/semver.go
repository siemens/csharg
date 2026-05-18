// (c) Siemens AG 2023
//
// SPDX-License-Identifier: MIT

package sem

import (
	"github.com/thediveo/go-plugger/v3"
)

// SemVer defines an exposed plugin symbol type for returning (overriding) the
// CLI binary's semantic version. The first plugin returning a non-empty string
// will win.
type SemVer func() string

// Version returns the semantic version string returned by the first registered
// plugin that is not empty, otherwise the specified preset.
func Version(preset string) string {
	for _, pluginsemver := range plugger.Group[SemVer]().Symbols() {
		if sv := pluginsemver(); sv != "" {
			return sv
		}
	}
	return preset
}
