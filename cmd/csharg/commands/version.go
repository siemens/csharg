// (c) Siemens AG 2023
//
// SPDX-License-Identifier: MIT

package commands

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/thediveo/clippy/cliplugin"
	"github.com/thediveo/go-plugger/v3"

	"github.com/siemens/csharg/v2"
	"github.com/siemens/csharg/v2/cmd/cli/client"
	"github.com/siemens/csharg/v2/cmd/cli/semver"
)

func init() {
	plugger.Group[cliplugin.SetupCLI]().Register(
		versionSetupCLI, plugger.WithPlugin("version"))
}

// versionSetupCLI adds the “version” command.
func versionSetupCLI(cmd *cobra.Command) {
	cmd.AddCommand(newVersionCmd())
}

// newVersionCmd returns a new “csharg version” command. The semantic version
// is the one defined for the main csharg client package, so there's no
// separate version number for the csharg CLI command. In addition, the version
// command lists the included client types.
func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show version (including integrated capture service clients).",
		Run: func(cmd *cobra.Command, args []string) {
			version := semver.Resolve(csharg.SemVersion)
			clients := strings.Join(plugger.Group[client.New]().Plugins(), ",")
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s %s\ncapture service clients: %s\n",
				cmd.Parent().Name(), version, clients)
		},
	}
}
