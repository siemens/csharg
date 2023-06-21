// (c) Siemens AG 2023
//
// SPDX-License-Identifier: MIT

package command

import (
	"fmt"
	"strings"

	"github.com/siemens/csharg"
	"github.com/siemens/csharg/cli"
	"github.com/spf13/cobra"
	"github.com/thediveo/go-plugger/v3"
)

// Provides the “csharg version” command. The semantic version is the one
// defined for the main csharg client package, so there's no separate version
// number for the csharg CLI command. In addition, the version command lists the
// included client types.
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show version (with integrated capture service clients).",
	Run: func(cmd *cobra.Command, args []string) {
		semver := csharg.SemVersion
		for _, pluginsemver := range plugger.Group[cli.SemVer]().Symbols() {
			semver = pluginsemver()
			break
		}
		fmt.Printf("%s version %s (capture service clients: %s)\n",
			cmd.Parent().Name(),
			semver,
			strings.Join(plugger.Group[cli.NewClient]().Plugins(), ", "))
	},
}

func init() {
	plugger.Group[cli.SetupCLI]().Register(
		VersionSetupCLI, plugger.WithPlugin("version"))
}

// VersionSetupCLI adds the “version” command.
func VersionSetupCLI(cmd *cobra.Command) {
	cmd.AddCommand(versionCmd)
}
