// (c) Siemens AG 2023
//
// SPDX-License-Identifier: MIT

// Implements the "csharg capture network" subcommand.

package capture

import (
	"github.com/spf13/cobra"
)

// newNetworkCmd returns a new "csharg capture network" command.
func newNetworkCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "network [flags] NETWORK NODE",
		Short: "capture from a process/process-less network stack on a node",
		Example: `# Capture from host network stack on specific node "worker-42"
csharg capture network "init (1)" worker-42

# Capture from bind-mounted and process-less network stack
csharg capture network foobarnet worker-42
	
# Capture from stand-alone container in specific cluster context
csharg --context mycluster network "init (1)" worker-42`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			containername := args[0]
			nodename := args[1]
			return capture(cmd, containername, []string{"bindmount", "proc"}, nodename)
		},
	}
}
