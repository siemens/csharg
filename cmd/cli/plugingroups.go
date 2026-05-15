// (c) Siemens AG 2023
//
// SPDX-License-Identifier: MIT

package cli

// SemVer defines an exposed plugin symbol type for returning (overriding) the
// CLI binary's semantic version. The first plugin will win.
type SemVer func() string
