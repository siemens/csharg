// (c) Siemens AG 2023
//
// SPDX-License-Identifier: MIT

package csharg

import (
	"runtime/debug"
	"strings"
)

const modulePath = "github.com/siemens/csharg/v2"

// SemVersion is the semantic version string of the csharg module. Release
// builds set it using “-ldflags -X”, otherwise it is taken from the Go build
// information.
var SemVersion string

func init() {
	if SemVersion == "" {
		SemVersion = buildInfoVersion()
	}
}

// buildInfoVersion returns the csharg module version from the binary's build
// information, regardless of csharg being the main module or a dependency.
func buildInfoVersion() string {
	const devel = "(devel)"
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return devel
	}
	for _, mod := range append([]*debug.Module{&bi.Main}, bi.Deps...) {
		if mod.Path != modulePath {
			continue
		}
		if mod.Replace != nil {
			mod = mod.Replace
		}
		if mod.Version == "" || mod.Version == devel {
			return devel
		}
		return strings.TrimPrefix(mod.Version, "v")
	}
	return devel
}
