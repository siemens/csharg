// (c) Siemens AG 2023
//
// SPDX-License-Identifier: MIT

package command

import (
	"github.com/siemens/csharg"
	"github.com/siemens/csharg/cli"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/thediveo/go-plugger/v3"
)

// enable debug log output.
var enable bool

func init() {
	plugger.Group[cli.SetupCLI]().Register(DebugSetupCLI, plugger.WithPlugin("debug"))
	plugger.Group[cli.BeforeCommand]().Register(DebugBeforeCommand, plugger.WithPlugin("debug"))
}

// DebugSetupCLI registers the “--debug” CLI flag.
func DebugSetupCLI(cmd *cobra.Command) {
	pf := cmd.PersistentFlags()
	pf.BoolVarP(&enable, "debug", "d", false, "Enable debug output")
}

// DebugBeforeCommand enables debug logging when requested via the “--debug” flag.
func DebugBeforeCommand(*cobra.Command) error {
	// When asked for, enable debug logging.
	if enable {
		log.SetLevel(log.DebugLevel)
		log.Debugf("csharg version %s", csharg.SemVersion)
	}
	return nil
}
