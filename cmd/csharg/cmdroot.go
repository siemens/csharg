// (c) Siemens AG 2026
//
// SPDX-License-Identifier: MIT

// This is the main entry of the csharg CLI tool. There isn't actually much
// here to do except for running the csharg "root" command which will parse
// the CLI args and then hopefully invoke the correct command and sub-command.

package main

import (
	"strings"

	"github.com/spf13/cobra"
	"github.com/thediveo/clippy"
	_ "github.com/thediveo/clippy/debug"
	_ "github.com/thediveo/lxkns/cmd/cli/silent"

	_ "github.com/siemens/csharg/cmd/cli/client"
	_ "github.com/siemens/csharg/cmd/cli/host"
	_ "github.com/siemens/csharg/cmd/csharg/commands"
)

// rootCmd represents the Cobra "root" command thus the charg CLI itself.
func newRootCmd() (rootCmd *cobra.Command) {
	rootCmd = &cobra.Command{
		Use:   "csharg",
		Short: "Capture network traffic in Kubernetes clusters",
		Long: `csharg is a CLI tool for capturing live network traffic from various
capture targets, such as Kubernetes pods, standalone containers (Docker, but also
others), and also container-less network stacks.`,
		// See: https://github.com/spf13/cobra/issues/340
		SilenceUsage:  true,
		SilenceErrors: false,
		// Check mutually exclusive CLI args, ...
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			// Nota bene: some commands like the "help" can be in some weird
			// detached state where they don't have the persistent flags
			// inherited, so we need to "fix in post" (even if this is in "pre",
			// but hey!). To add insult to injury, flag sets have no way to
			// determine if they're empty or not, as PersistentFlags() always
			// returns a flag set (yay!), creating a fresh empty one on-the-fly
			// when necessary.
			if cmd.Use == "help" || strings.HasPrefix(cmd.Use, "help ") {
				cmd.PersistentFlags().AddFlagSet(rootCmd.PersistentFlags())
			}
			return clippy.BeforeCommand(cmd)
		},
	}
	clippy.AddFlags(rootCmd) // ...runs all registered SetupCLI plugins.
	helpCmd, _, _ := rootCmd.Find([]string{"help"})
	helpCmd.PersistentFlags().AddFlagSet(rootCmd.PersistentFlags())

	defaultHelpFn := rootCmd.HelpFunc()
	rootCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		if err := clippy.BeforeCommand(rootCmd); err != nil {
			return
		}
		defaultHelpFn(cmd, args)
	})

	return
}
