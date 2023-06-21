// (c) Siemens AG 2023
//
// SPDX-License-Identifier: MIT

package command

import (
	"github.com/siemens/csharg/cli"
	"github.com/spf13/cobra"
	"github.com/thediveo/go-plugger/v3"
)

// Provides the "csharg options" command which gives information about the
// available global CLI flags/options. This is modelled after what kubectl, etc.
// have on offer.
var optionsCmd = &cobra.Command{
	Use:   "options",
	Short: "List of global command-line options which apply to all commands.",
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Usage()
	},
}

// optionsUsageTemplate replaces cobra's builtin usage template which
// doesn't quite fit in this special usecase for listing only the global
// options.
var optionsUsageTemplate = `{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}
`

func init() {
	plugger.Group[cli.SetupCLI]().Register(OptionsSetupCLI, plugger.WithPlugin("options"))
}

// OptionsSetupCLI adds the "option" command.
func OptionsSetupCLI(cmd *cobra.Command) {
	cmd.AddCommand(optionsCmd)
	optionsCmd.SetUsageTemplate(optionsUsageTemplate)
}
