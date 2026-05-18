// (c) Siemens AG 2023
//
// SPDX-License-Identifier: MIT

package commands

import (
	"log/slog"
	"strings"

	"github.com/spf13/cobra"
	"github.com/thediveo/clippy/cliplugin"
	"github.com/thediveo/go-plugger/v3"

	"github.com/siemens/csharg"
	"github.com/siemens/csharg/cmd/cli/client"
	"github.com/siemens/csharg/cmd/cli/sem"
)

// Provides the “csharg version” command. The semantic version is the one
// defined for the main csharg client package, so there's no separate version
// number for the csharg CLI command. In addition, the version command lists the
// included client types.
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show version (including integrated capture service clients).",
	Run: func(cmd *cobra.Command, args []string) {
		semver := sem.Version(csharg.SemVersion)
		slog.Info(cmd.Parent().Name(),
			slog.String("semver", semver),
			slog.String("capture-service-clients", strings.Join(plugger.Group[client.New]().Plugins(), ",")))
	},
}

func init() {
	plugger.Group[cliplugin.SetupCLI]().Register(
		versionSetupCLI, plugger.WithPlugin("version"))
}

// versionSetupCLI adds the “version” command.
func versionSetupCLI(cmd *cobra.Command) {
	cmd.AddCommand(versionCmd)
}
