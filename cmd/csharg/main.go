// (c) Siemens AG 2023
//
// SPDX-License-Identifier: MIT

// This is the main entry of the csharg CLI tool. There isn't actually much
// here to do except for running the csharg "root" command which will parse
// the CLI args and then hopefully invoke the correct command and sub-command.

package main

import (
	"os"

	// Pull in all command packages which define sub-commands: they will
	// register themselves as needed, but we need the packages to get included,
	// as otherwise there are no references in the code which could pull them
	// in anyway.
	"github.com/siemens/csharg/cli/command"
	_ "github.com/siemens/csharg/cli/command/capture"

	_ "github.com/siemens/csharg/cli/sharktank" // stand-alone host

	log "github.com/sirupsen/logrus"
	prefixed "github.com/x-cray/logrus-prefixed-formatter"
)

func main() {
	// Establish logger output format in case we're hitting errors, et cetera.
	f := new(prefixed.TextFormatter)
	f.DisableColors = true
	f.ForceFormatting = true
	f.FullTimestamp = true
	f.TimestampFormat = "15:04:05"
	log.SetFormatter(f)

	// This is cobra boilerplate documentation, except for the missing call to
	// fmt.Println(err) which in the original boilerplate is just plain wrong:
	// it renders the error message twice, see also:
	// https://github.com/spf13/cobra/issues/304
	if err := command.SetupCLI().Execute(); err != nil {
		os.Exit(1)
	}
}
