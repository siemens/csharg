// (c) Siemens AG 2023
//
// SPDX-License-Identifier: MIT

// Implements the "csharg capture container" subcommand.

package capture

import (
	"github.com/spf13/cobra"
)

func init() {
	captureCmd.AddCommand(ContainerCmd)
}

// ContainerCmd defines the "csharg capture container" command.
var ContainerCmd = &cobra.Command{
	Use:   "container [flags] CONTAINER [NODE]",
	Short: "capture from a stand-alone container on a stand-alone container host or node",
	Example: `# Capture from stand-alone container "mymoby" on host
csharg --host localhost:5001 capture container mycontainer-1 localhost

# Capture from stand-alone container in specific cluster context
csharg --context mycluster container mymoby worker-42`,
	Args: cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		containername := args[0]
		nodename := ""
		if standalonehost, err := cmd.Flags().GetString("host"); err != nil || standalonehost == "" {
			nodename = args[1]
		}
		return capture(cmd, containername, []string{"container"}, nodename)
	},
}
