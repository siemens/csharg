// (c) Siemens AG 2023
//
// SPDX-License-Identifier: MIT

package capture

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/siemens/csharg"
	"github.com/siemens/csharg/api"
	"github.com/siemens/csharg/cli"
	"github.com/siemens/csharg/cli/command"
	"github.com/thediveo/go-plugger/v3"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

const AvoidPromModeArg = "avoid-promiscuous"

// CaptureCmd defines the "csharg capture" command. Sub-commands will be
// automatically registered with this command by the other sibling .go files
// in this package.
var captureCmd = &cobra.Command{
	Use:   "capture [flags] TARGET",
	Short: "Capture and then live stream network traffic.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return capture(cmd, args[0], []string{}, "")
	},
}

func init() {
	plugger.Group[cli.SetupCLI]().Register(CaptureSetupCLI, plugger.WithPlugin("capture"))
}

// CaptureSetupCLI adds the "capture" command.
func CaptureSetupCLI(cmd *cobra.Command) {
	cmd.AddCommand(captureCmd)
	pf := captureCmd.PersistentFlags()
	pf.StringArrayP("interface", "i", []string{},
		"Name of interface to capture from. Can be specified multiple times.")
	pf.StringP("filter", "f", "",
		"Set the capture filter expression. It applies to all network interfaces included in a capture.")
	pf.BoolP(AvoidPromModeArg, "p", false,
		"Don't put network interfaces into promiscuous mode")
	pf.StringP("write", "w", "-",
		"Write captured network packets to file. Use \"-\" for stdout.")
}

// Capture network traffic from the specified named target and start streaming
// it. Optionally, the required type of target can be specified ("pod", et
// cetera), as well as the host/node name in order to give an unambiguous target
// match.
func capture(cmd *cobra.Command, targetname string, targettypes []string, nodename string) error {
	// Retrieve the list of capture targets from the container/cluster capture
	// service.
	st, err := command.NewSharkTank()
	if err != nil {
		return fmt.Errorf("invalid --context: %s", err)
	}
	// Final parameter sanity check.
	if targetname == "" {
		return fmt.Errorf("invalid empty capture target name")
	}
	log.Debugf("looking up capture target %q of type(s) %q on node %q",
		targetname, targettypes, nodename)
	// Try to find the named target and check for its type and/or nodename, if
	// additionally specified, too.
	matches := []*api.Target{}
	for _, t := range st.Targets() {
		log.Debugf("?target %+v", t)
		var typematch bool
		if len(targettypes) != 0 {
			// See if the type of this target is, erm, contained in the list of
			// target types...
			for _, tt := range targettypes {
				if t.Type == tt {
					typematch = true
					break
				} else if tt == "container" &&
					t.Type != "bindmount" && t.Type != "proc" && t.Type != "pod" {
					typematch = true
					break
				}
			}
		} else {
			// If no specific target type(s) has (have) been specified, then we
			// will always match any target type.
			typematch = true
		}
		if t.Name == targetname && typematch &&
			(nodename == "" || t.NodeName == nodename) {
			matches = append(matches, t)
		}
	}
	if len(matches) == 0 {
		if nodename == "" {
			return fmt.Errorf("capture target %q not found", targetname)
		}
		return fmt.Errorf("capture target %q on node %q not found", targetname, nodename)
	}
	if len(matches) > 1 {
		return fmt.Errorf("ambiguous capture target %q matches %d targets", targetname, len(matches))
	}
	// Open a new output file to dump the captured network packets into, or use
	// stdout, if "-" was specified.
	out := os.Stdout
	if wname, _ := cmd.Flags().GetString("write"); wname != "-" {
		var err error // ...oh, the joy of shady variable shadowing when misusing ":="!
		out, err = os.OpenFile(wname, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0640)
		if err != nil {
			return fmt.Errorf("cannot create packet capture file: %s", err.Error())
		}
		defer out.Close()
	}
	// Get any supported capture options, such as the list of network interfaces.
	captureopts := &csharg.CaptureOptions{}
	if nifs, err := cmd.Flags().GetStringArray("interface"); err == nil && len(nifs) > 0 {
		log.Debugf("capturing from network interfaces: %s", strings.Join(nifs, ", "))
		captureopts.Nifs = nifs
	}
	captureopts.AvoidPromiscuousMode, _ = cmd.Flags().GetBool(AvoidPromModeArg)
	if filter, err := cmd.Flags().GetString("filter"); err != nil && filter != "" {
		log.Debugf("capture filter expression: %q", filter)
		captureopts.Filter = filter
	}
	// Start the capture stream and keep streaming until we drop ... because
	// this CLI tool was SIGINT'ed or SIGTERM'ed.
	target := matches[0]
	capture, err := st.Capture(out, target, captureopts)
	if err != nil {
		return fmt.Errorf("cannot start capture: %s", err.Error())
	}
	done := make(chan os.Signal)
	signal.Notify(done, os.Interrupt)
	signal.Notify(done, syscall.SIGTERM)
	// ...zzzzzzzzzz...
	<-done
	// We're done, stop the packet capture stream in an orderly manner, so that
	// we won't stream half-broken captures, but instead get a clean end.
	// Stopping a capture will block until the capture has orderly terminated.
	log.Debugf("closing live network packet capture stream from target %q...", target.Name)
	capture.Stop()
	log.Debugf("network packet capture stream from target %q finished", target.Name)
	return nil
}
