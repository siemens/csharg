// (c) Siemens AG 2023
//
// SPDX-License-Identifier: MIT

package command

import (
	"errors"
	"strings"

	"github.com/siemens/csharg"
	"github.com/siemens/csharg/cli"
	"github.com/thediveo/go-plugger/v3"
)

// NewSharkTank returns a suitable packetflix capture service client by asking
// the registered client factories one after another until the first one returns
// a client or an error.
func NewSharkTank() (csharg.SharkTank, error) {
	for _, newClient := range plugger.Group[cli.NewClient]().Symbols() {
		st, err := newClient()
		if err != nil {
			return nil, err
		}
		if st != nil {
			return st, nil
		}
	}
	plugins := strings.Join(plugger.Group[cli.NewClient]().Plugins(), ", ")
	if plugins == "" {
		plugins = "(none)"
	}
	return nil, errors.New("no suitable capture API client; available clients: " + plugins)
}
