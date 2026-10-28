// (c) Siemens AG 2026
//
// SPDX-License-Identifier: MIT

package silent

import (
	"context"
	"log/slog"

	"github.com/spf13/cobra"
	"github.com/thediveo/clippy/cliplugin"
	"github.com/thediveo/clippy/debug"
	"github.com/thediveo/go-plugger/v3"
)

// Names of the CLI flags provided in this package.
const (
	SilentFlagName = "silent"
)

func init() {
	plugger.Group[cliplugin.SetupCLI]().Register(
		SetupCLI, plugger.WithPlugin("silent"),
		plugger.WithPlacement(">clippy/debug"))
	plugger.Group[cliplugin.BeforeCommand]().Register(
		BeforeCommand, plugger.WithPlugin("silent"),
		plugger.WithPlacement("<clippy/debug"))
}

// SetupCLI runs after(!) the debug flag's SetupCLI so that we can add our
// "--silent" flag and make it mutually exclusive to the "--debug" flag.
func SetupCLI(cmd *cobra.Command) {
	cmd.PersistentFlags().Bool(SilentFlagName, false, "only log errors")
	cmd.MarkFlagsMutuallyExclusive(SilentFlagName, debug.DebugFlagName)
}

// BeforeCommand runs before(!) the debug flag's BeforeCommand and raises the
// logging bar when the "--silent" flag has been specified with the command. It
// does so by attaching the forced level to the context of the command.
func BeforeCommand(cmd *cobra.Command) error {
	silence, _ := cmd.PersistentFlags().GetBool(SilentFlagName)
	if !cmd.PersistentFlags().Lookup(SilentFlagName).Changed {
		if ets, ok := cmd.Context().Value(ctxEnjoyTheSilence).(bool); ok {
			silence = ets
		}
	}
	if silence {
		debug.SetLevel(cmd, slog.LevelError)
	}
	return nil
}

type ctxKey int

const (
	ctxEnjoyTheSilence ctxKey = iota
)

// PreferSilence configures the command to default to --silence instead of
// --silence=false; CLI users can still override this by specifying
// “--silence=false”. PreferSilence needs to be called before the BeforeCommand
// plugins chain runs, that is, before clippy.BeforeCommand.
func PreferSilence(cmd *cobra.Command) {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	cmd.SetContext(context.WithValue(ctx, ctxEnjoyTheSilence, true))
}
