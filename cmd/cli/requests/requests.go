// (c) Siemens AG 2026
//
// SPDX-License-Identifier: MIT

package requests

import (
	"time"

	"github.com/spf13/cobra"
	"github.com/thediveo/clippy/cliplugin"
	"github.com/thediveo/go-plugger/v3"
)

const (
	// RequestTimeoutFlag is the name of the global request timeout CLI flag.
	RequestTimeoutFlag = "request-timeout"
	// RequestTimeoutDefault is the default request timeout, meaning no timeout.
	RequestTimeoutDefault = time.Duration(0)
)

func init() {
	plugger.Group[cliplugin.SetupCLI]().Register(
		reqSetupCLI, plugger.WithPlugin("req"))
}

// reqSetupCLI adds the global (persistent) request timeout flag.
func reqSetupCLI(cmd *cobra.Command) {
	cmd.PersistentFlags().Duration(RequestTimeoutFlag, RequestTimeoutDefault,
		`The length of time to wait before giving up on a single server request.
Non-zero values should contain a corresponding time unit (e.g. 1s, 2m, 3h).
A value of zero means don't timeout requests.`)
}

// GetTimeout returns the request timeout specified for the passed command,
// including when inherited from a parent command. It returns zero if the flag
// isn't present.
func GetTimeout(cmd *cobra.Command) time.Duration {
	timeout, _ := cmd.Flags().GetDuration(RequestTimeoutFlag)
	return timeout
}
