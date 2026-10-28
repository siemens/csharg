// (c) Siemens AG 2023
//
// SPDX-License-Identifier: MIT

package commands

import (
	"github.com/spf13/cobra"
	"github.com/thediveo/clippy/cliplugin"
	"github.com/thediveo/go-plugger/v3"
)

// optionsUsageTemplate replaces cobra's builtin usage template which
// doesn't quite fit in this special usecase for listing only the global
// options.
const optionsUsageTemplate = `{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}
`

func init() {
	plugger.Group[cliplugin.SetupCLI]().Register(
		optionsSetupCLI, plugger.WithPlugin("options"))
}

// optionsSetupCLI adds the "option" command.
func optionsSetupCLI(cmd *cobra.Command) {
	cmd.AddCommand(newOptionsCmd())
}

// newOptionsCmd returns a new "csharg options" command which gives information
// about the available global CLI flags/options. This is modelled after what
// kubectl, etc. have on offer.
func newOptionsCmd() *cobra.Command {
	optionsCmd := &cobra.Command{
		Use:   "options",
		Short: "List of global command-line options which apply to all commands.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Usage()
		},
	}
	optionsCmd.SetUsageTemplate(optionsUsageTemplate)
	return optionsCmd
}
