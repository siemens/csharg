// (c) Siemens AG 2026
//
// SPDX-License-Identifier: MIT

package token

import (
	"github.com/spf13/cobra"
	"github.com/thediveo/clippy/cliplugin"
	"github.com/thediveo/go-plugger/v3"
)

// TokenFlag is the name of the global bearer token CLI flag.
const TokenFlag = "token"

func init() {
	plugger.Group[cliplugin.SetupCLI]().Register(
		tokenSetupCLI, plugger.WithPlugin("token"))
}

// tokenSetupCLI adds the global (persistent) bearer token flag.
func tokenSetupCLI(cmd *cobra.Command) {
	cmd.PersistentFlags().String(TokenFlag, "",
		"Bearer token for authentication to the API server or URL")
}

// Get returns the bearer token specified for the passed command, including
// when inherited from a parent command. It returns an empty string if the flag
// isn't present.
func Get(cmd *cobra.Command) string {
	token, _ := cmd.Flags().GetString(TokenFlag)
	return token
}
