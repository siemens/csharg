// (c) Siemens AG 2026
//
// SPDX-License-Identifier: MIT

// Implements the csharg "root" command with its global CLI flags.
// Additionally runs some checks on some of those global CLI flags, where
// necessary, so individual commands do not need to check them themselves.

package req

import (
	"time"

	"github.com/spf13/cobra"
	"github.com/thediveo/clippy/cliplugin"
	"github.com/thediveo/go-plugger/v3"
)

const (
	RequestTimeoutFlag    = "request-timeout"
	RequestTimeoutDefault = time.Duration(0)
)

func init() {
	plugger.Group[cliplugin.SetupCLI]().Register(
		reqSetupCLI, plugger.WithPlugin("req"))
}

func reqSetupCLI(cmd *cobra.Command) {
	cmd.PersistentFlags().Duration(RequestTimeoutFlag, RequestTimeoutDefault,
		`The length of time to wait before giving up on a single server request.
Non-zero values should contain a corresponding time unit (e.g. 1s, 2m, 3h).
A value of zero means don't timeout requests.`)
}

func GetTimeout(cmd *cobra.Command) time.Duration {
	timeout, _ := cmd.PersistentFlags().GetDuration(RequestTimeoutFlag)
	return timeout
}
