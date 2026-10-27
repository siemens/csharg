// (c) Siemens AG 2023
//
// SPDX-License-Identifier: MIT

// Provides the "csharg list" command for listing available capture network
// traffic from targets served by a Packetflix service.

package commands

import (
	"fmt"
	"log/slog"

	"github.com/spf13/cobra"
	"github.com/thediveo/clippy/cliplugin"
	"github.com/thediveo/go-plugger/v3"
	"github.com/thediveo/klo"

	"github.com/siemens/csharg/v2/api"
	"github.com/siemens/csharg/v2/cmd/cli/client"
)

// Builtin custom-columns templates
const (
	// PodListTemplate defines the custom columns when listing only pods.
	PodListTemplate = "POD:{.Name}"
	// PodWideListTemplate defines the custom columns when listing only pods in
	// --wide mode.
	PodWideListTemplate = "POD:{.Name},NODE:{.NodeName}"

	// TargetListTemplate defines the custom columns when listing all types of
	// capture targets.
	TargetListTemplate = "TARGET:{.Name},TYPE:{.Type},NODE:{.NodeName}"
	// TargetWideListTemplate is like TargetListTemplate, but additionally tacks
	// on a column listing the capture service pod names.
	TargetWideListTemplate = "TARGET:{.Name},TYPE:{.Type},NODE:{.NodeName},SERVICE:{.CaptureService}"

	// NameListTemplate for handling "-o name" and only showing a custom "name"
	// column; this template should be used with no headers shown, as kubectl
	// and others do.
	NameListTemplate = "NAME:{.Name}"
)

func init() {
	plugger.Group[cliplugin.SetupCLI]().Register(
		listSetupCLI, plugger.WithPlugin("list"))
}

// listSetupCLI adds the “list” command.
func listSetupCLI(cmd *cobra.Command) {
	cmd.AddCommand(newListCmd())
}

// newListCmd returns a new "csharg list" command.
func newListCmd() *cobra.Command {
	listCmd := &cobra.Command{
		Use:     "list [flags] [pods|containers|networks...]",
		Aliases: []string{"ps"},
		Short:   "List network capture targets in a Docker host or Kubernetes cluster",
		Args:    cobra.OnlyValidArgs,
		ValidArgs: []string{
			"pod", "pods",
			"container", "containers",
			"network", "networks",
		},
		RunE: filteredlist,
	}
	listCmd.Flags().StringP("output", "o", "",
		"Output format. One of: json|yaml|wide|custom-columns=...|custom-columns-file=...|jsonpath=...|jsonpath-file=...")
	listCmd.Flags().Bool("no-headers", false, "When using the default or custom-column output format, don't print headers (default print headers).")
	listCmd.Flags().String("sort-by", "{.Name}{'/'}{.NodeName}",
		"If non-empty, sort custom-columns using this field specification. The field specification is expressed as a JSONPath expression (e.g. '{.Name}').")
	return listCmd
}

// filteredlist fetches the list of available capture targets and optionally
// filters by target type(s) for output using a template.
func filteredlist(cmd *cobra.Command, args []string) error {
	// Get the capture type filter settings; args have already been validated,
	// so we can simply go by their initials.
	var showPods, showContainers, showNetworks bool
	if len(args) == 0 {
		showPods, showContainers, showNetworks = true, true, true
	}
	for _, arg := range args {
		switch arg[0] {
		case 'p':
			showPods = true
		case 'c':
			showContainers = true
		case 'n':
			showNetworks = true
		}
	}
	slog.Debug("target type filter",
		slog.Bool("pods", showPods),
		slog.Bool("containers", showContainers),
		slog.Bool("networks", showNetworks))
	// If the user did not specify any output format or did just select the wide
	// output format then select a suitable builtin format based on the filter
	// settings...
	if outfmt, err := cmd.LocalFlags().GetString("output"); err == nil && (outfmt == "" || outfmt == "wide") {
		// If only pods are to be shown, then go for the simpler pod targets
		// template. Otherwise don't touch the output format and let the custom
		// columns default to the built-in all-targets template.
		if showPods && !showContainers && !showNetworks {
			var ccfmt string
			if outfmt == "wide" {
				ccfmt = PodWideListTemplate
			} else {
				ccfmt = PodListTemplate
			}
			if err := cmd.LocalFlags().Set("output", "custom-columns="+ccfmt); err != nil {
				panic(err)
			}
		}
	}
	// Get the output CLI flag and prepare a suitable object printer.
	prn, err := getPrinter(cmd)
	if err != nil {
		return err
	}
	// ...throwing in sorting, if not explicitly forbidden. It depends on the
	// object printer if it will honor the sorted data or will just impose its
	// own order anyway.
	if sortby, err := cmd.LocalFlags().GetString("sort-by"); err == nil && sortby != "" {
		var err error
		prn, err = klo.NewSortingPrinter(sortby, prn)
		if err != nil {
			return fmt.Errorf("invalid --sort-by: %w", err)
		}
	}
	// Retrieve the list of capture targets from the container/cluster capture
	// service.
	st, err := client.NewSharkTank(cmd)
	if err != nil {
		return err
	}
	targets, err := st.Targets()
	if err != nil {
		return err
	}
	// Filter the target list and then print it.
	ft := make([]*api.Target, 0, len(targets))
	for _, t := range targets {
		slog.Debug("found target",
			slog.String("name", t.Name),
			slog.String("type", t.Type),
			slog.String("node", t.NodeName),
			slog.String("service", t.CaptureService))
		switch t.Type {
		case "pod":
			if !showPods {
				continue
			}
		case "bindmount", "proc":
			if !showNetworks {
				continue
			}
		default:
			if !showContainers {
				continue
			}
		}
		ft = append(ft, t)
	}
	return prn.Fprint(cmd.OutOrStdout(), ft)
}

// getPrinter returns a value printer configured according to the output format
// chosen by the user, and some more optional output configuration flags.
func getPrinter(cmd *cobra.Command) (prn klo.ValuePrinter, err error) {
	outfmt, err := cmd.LocalFlags().GetString("output")
	if err != nil {
		return
	}
	if outfmt == "name" {
		// Support "-o name" output format which uses our builtin custom-columns
		// template to only show capture target names, and hide the column
		// header. Please note that this is most useful for listing pod capture
		// targets, but less useful for other types of capture targets, where a
		// user might need to know the node name also, in order to select the
		// correct capture target from a set of targets with the same name, such
		// as "init (1)".
		prn, err = klo.PrinterFromFlag("custom-columns="+NameListTemplate, nil)
		if err != nil {
			panic(err)
		}
		prn.(*klo.CustomColumnsPrinter).HideHeaders = true
		return
	}
	// For the other output format option, let the kubectl-like output
	// package handle the details and give us just the printer suitable for
	// dumping the target list onto our users.
	prn, err = klo.PrinterFromFlag(outfmt, &klo.Specs{
		DefaultColumnSpec: TargetListTemplate,
		WideColumnSpec:    TargetWideListTemplate,
	})
	if err != nil {
		return
	}
	if ccprn, ok := prn.(*klo.CustomColumnsPrinter); ok {
		ccprn.Padding = 3
		if noheaders, err := cmd.LocalFlags().GetBool("no-headers"); err == nil {
			ccprn.HideHeaders = noheaders
		}
	}
	return
}
