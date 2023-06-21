// (c) Siemens AG 2023
//
// SPDX-License-Identifier: MIT

package cli

import (
	"github.com/siemens/csharg"
	"github.com/spf13/cobra"
)

// SetupCLI defines an exposed plugin symbol type for adding “things” to a
// cobra root command (the csharg root command in particular).
type SetupCLI func(*cobra.Command)

// CommandExamples defines an exposed symbol with CLI examples, indexed by a
// particular (sub) command, namely: “list” and “capture” at this time.
type CommandExamples func() map[string]string

// BeforeCommand defines an exposed plugin symbol type for running checks after
// the command line args have been processed and before running the (choosen)
// command.
type BeforeCommand func(*cobra.Command) error

// NewClient defines an exposed plugin symbol type for returning a suitable
// capture client based on the CLI args. If a registered plugin factory isn't
// responsible, it must return a nil client as well as a nil error. If a factory
// returns a non-nil error, the attempt to find a suitable factory will be
// aborted and the returned error reported to the CLI user.
type NewClient func() (csharg.SharkTank, error)

// SemVer defines an exposed plugin symbol type for returning (overriding) the
// CLI binary's semantic version. The first plugin will win.
type SemVer func() string
