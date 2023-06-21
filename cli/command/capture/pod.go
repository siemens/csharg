// (c) Siemens AG 2023
//
// SPDX-License-Identifier: MIT

// Implements the "csharg capture pod" subcommand.

package capture

import (
	"strings"

	"github.com/spf13/cobra"
)

func init() {
	captureCmd.AddCommand(PodCmd)
	PodCmd.Flags().StringP("namespace", "n", "default",
		"Namespace of pod, unless explicitly specified in pod name itself. Defaults to \"default\" namespace.")
}

// PodCmd defines the "csharg capture pod" command.
var PodCmd = &cobra.Command{
	Use:   "pod [flags] POD",
	Short: "capture from a Kubernetes pod",
	Example: `# Capture from pod in default namespace in the host KinD deployment and pipe the captured packets into Wireshark.
csharg --host ... capture pod mikroservice | wireshark -k -i -`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		podnamespace, _ := cmd.Flags().GetString("namespace")
		podname := args[0] // index safe, was already checked via ExactArgs(1).
		if !strings.ContainsRune(podname, '/') {
			podname = podnamespace + "/" + podname
		}
		return capture(cmd, podname, []string{"pod"}, "")
	},
}
