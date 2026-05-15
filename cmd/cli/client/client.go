// (c) Siemens AG 2026
//
// SPDX-License-Identifier: MIT

package client

import (
	"errors"
	"strings"

	"github.com/spf13/cobra"
	"github.com/thediveo/go-plugger/v3"

	"github.com/siemens/csharg"
)

// MutuallyExclusiveGroup is the name of an annotation value for flags that should be
// mutually exclusive for specifying capture service client endpoint
// information.
const MutuallyExclusiveGroup = "sharktank"

// New defines an exposed plugin symbol type for returning a suitable capture
// client based on the CLI args. If a registered plugin factory isn't
// responsible, it must return a nil client as well as a nil error. If a factory
// returns a non-nil error, the attempt to find a suitable factory will be
// aborted and the returned error reported to the CLI user.
type New func(*cobra.Command) (csharg.SharkTank, error)

// NewSharkTank returns a suitable packetflix capture service client by asking
// the registered client factories one after another until the first one returns
// a non-nil client or an error.
func NewSharkTank(cmd *cobra.Command) (csharg.SharkTank, error) {
	for _, newClient := range plugger.Group[New]().Symbols() {
		st, err := newClient(cmd)
		if err != nil {
			return nil, err
		}
		if st != nil {
			return st, nil
		}
	}
	plugins := strings.Join(plugger.Group[New]().Plugins(), ", ")
	if plugins == "" {
		plugins = "(none)"
	}
	return nil, errors.New("no suitable capture API client found; available clients: " + plugins)
}
