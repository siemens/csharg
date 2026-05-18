// (c) Siemens AG 2026
//
// SPDX-License-Identifier: MIT

// Implements the csharg "root" command with its global CLI flags.
// Additionally runs some checks on some of those global CLI flags, where
// necessary, so individual commands do not need to check them themselves.

package token

import (
	"github.com/spf13/cobra"
	"github.com/thediveo/clippy/cliplugin"
	"github.com/thediveo/go-plugger/v3"
)

const (
	TokenFlag = "token"
)

func init() {
	plugger.Group[cliplugin.SetupCLI]().Register(
		tokenSetupCLI, plugger.WithPlugin("token"))
}

func tokenSetupCLI(cmd *cobra.Command) {
	cmd.PersistentFlags().String(TokenFlag, "",
		"Bearer token for authentication to the API server or URL")
}

func Get(cmd *cobra.Command) string {
	token, _ := cmd.PersistentFlags().GetString(TokenFlag)
	return token
}
