// (c) Siemens AG 2023
//
// SPDX-License-Identifier: MIT

package capture

import (
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/thediveo/clippy/cliplugin"
	"github.com/thediveo/go-plugger/v3"

	"github.com/siemens/csharg/v2"
	"github.com/siemens/csharg/v2/api"
	"github.com/siemens/csharg/v2/cmd/cli/client"
)

const AvoidPromModeArg = "avoid-promiscuous"

func init() {
	plugger.Group[cliplugin.SetupCLI]().Register(captureSetupCLI, plugger.WithPlugin("capture"))
}

// captureSetupCLI adds the "capture" command.
func captureSetupCLI(cmd *cobra.Command) {
	cmd.AddCommand(newCaptureCmd())
}

// newCaptureCmd returns a new "csharg capture" command, including its
// sub-commands.
func newCaptureCmd() *cobra.Command {
	captureCmd := &cobra.Command{
		Use:   "capture [flags] TARGET",
		Short: "Capture and then live stream network traffic.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return capture(cmd, args[0], []string{}, "")
		},
	}
	captureCmd.AddCommand(
		newContainerCmd(),
		newNetworkCmd(),
		newPodCmd(),
	)
	pf := captureCmd.PersistentFlags()
	pf.StringArrayP("interface", "i", []string{},
		"Name of interface to capture from. Can be specified multiple times.")
	pf.StringP("filter", "f", "",
		"Set the capture filter expression. It applies to all network interfaces included in a capture.")
	pf.BoolP(AvoidPromModeArg, "p", false,
		"Don't put network interfaces into promiscuous mode")
	pf.StringP("write", "w", "-",
		"Write captured network packets to file. Use \"-\" for stdout.")
	return captureCmd
}

// Capture network traffic from the specified named target and start streaming
// it. Optionally, the required type of target can be specified ("pod", et
// cetera), as well as the host/node name in order to give an unambiguous target
// match.
func capture(cmd *cobra.Command, targetname string, targettypes []string, nodename string) error {
	// Retrieve the list of capture targets from the container/cluster capture
	// service.
	st, err := client.NewSharkTank(cmd)
	if err != nil {
		return err
	}
	// Final parameter sanity check.
	if targetname == "" {
		return fmt.Errorf("invalid empty capture target name")
	}
	slog.Debug("looking up capture target",
		slog.String("name", targetname),
		slog.Any("types", targettypes),
		slog.String("node", nodename))
	// Try to find the named target and check for its type and/or nodename, if
	// additionally specified, too.
	targets, err := st.Targets()
	if err != nil {
		return err
	}
	matches := []*api.Target{}
	for _, t := range targets {
		slog.Debug("checking target", slog.Any("target", t))
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
	out := cmd.OutOrStdout()
	if wname, _ := cmd.Flags().GetString("write"); wname != "-" {
		f, err := os.OpenFile(wname, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0640)
		if err != nil {
			return fmt.Errorf("cannot create packet capture file: %s", err.Error())
		}
		defer func() { _ = f.Close() }()
		out = f
	}
	// Get any supported capture options, such as the list of network interfaces.
	captureopts := &csharg.CaptureOptions{}
	if nifs, err := cmd.Flags().GetStringArray("interface"); err == nil && len(nifs) > 0 {
		slog.Debug("capturing from network interfaces", slog.Any("interfaces", nifs))
		captureopts.Nifs = nifs
	}
	captureopts.AvoidPromiscuousMode, _ = cmd.Flags().GetBool(AvoidPromModeArg)
	if filter, err := cmd.Flags().GetString("filter"); err == nil && filter != "" {
		slog.Debug("capture filter expression", slog.String("filter", filter))
		captureopts.Filter = filter
	}
	// Without a SIGPIPE notification, Go kills us on a broken stdout pipe
	// instead of failing the write with EPIPE; the notifications go unread.
	brokenPipes := make(chan os.Signal, 1)
	signal.Notify(brokenPipes, syscall.SIGPIPE)
	defer signal.Stop(brokenPipes)
	// Start the capture stream and keep streaming until either we drop ...
	// because this CLI tool was SIGINT'ed or SIGTERM'ed, or the stream ends by
	// itself.
	target := matches[0]
	capture, err := st.Capture(out, target, captureopts)
	if err != nil {
		return fmt.Errorf("cannot start capture: %w", err)
	}
	sigs := make(chan os.Signal, 2)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigs)
	ended := make(chan struct{})
	var captureErr error
	go func() {
		captureErr = capture.Wait()
		close(ended)
	}()
	// ...zzzzzzzzzz...
	select {
	case <-ended:
		slog.Debug("network packet capture stream ended by itself", slog.String("target", target.Name))
		// The packet consumer, such as Wireshark, closing its end is a normal end.
		if isClosedPipe(captureErr) {
			return nil
		}
		return captureErr
	case <-sigs:
		// Let a second signal kill us should stopping hang on a stuck writer.
		signal.Stop(sigs)
	}
	// We're done, stop the packet capture stream in an orderly manner, so that
	// we won't stream half-broken captures, but instead get a clean end.
	// Stopping a capture will block until the capture has orderly terminated.
	slog.Debug("closing live network packet capture stream...", slog.String("target", target.Name))
	err = capture.Stop()
	slog.Debug("network packet capture stream finished", slog.String("target", target.Name))
	return err
}
