// (c) Siemens AG 2026
//
// SPDX-License-Identifier: MIT

package host

import (
	"github.com/spf13/cobra"
	"github.com/thediveo/clippy/cliplugin"
	"github.com/thediveo/go-plugger/v3"

	"github.com/siemens/csharg"
	"github.com/siemens/csharg/cmd/cli/client"
	"github.com/siemens/csharg/cmd/cli/examples"
	"github.com/siemens/csharg/cmd/cli/mutual"
	"github.com/siemens/csharg/cmd/csharg/commands"
)

const (
	StandaloneHostFlag    = "host"
	InsecureFlag          = "insecure"
	InsecureShorthandFlag = "k"
)

func init() {
	plugger.Group[cliplugin.SetupCLI]().Register(
		setupCLI, plugger.WithPlugin("host"))
	plugger.Group[client.New]().Register(
		newClient, plugger.WithPlugin("host"))
	plugger.Group[examples.Illustrate]().Register(
		func() examples.ForCommands {
			return examples.ForCommands{
				"list": `# List only (stand-alone) containers on the local host.
csharg --host localhost:5001 list containers

# List all capture targets on a remote container host.
csharg --host dns-or-ip:5001 list

# List pods in the local KinD deployment.
csharg --host localhost:5001 list pods`,

				"capture": `# Capture from (stand-alone) container on the local host and pipe the captured packets into Wireshark.
csharg --host localhost:5001 capture fools-mikroserviz | wireshark -k -i -`,
			}
		},
		plugger.WithPlugin("host"), plugger.WithPlacement("<"))
}

func setupCLI(cmd *cobra.Command) {
	pf := cmd.PersistentFlags()
	pf.String(StandaloneHostFlag, "",
		`[http://|https://]hostname[:port][/path] of a Packetflix capture service
on a standalone container host`)
	mutual.ExclusiveInGroup(pf, StandaloneHostFlag, client.MutuallyExclusiveGroup)
	pf.BoolP(InsecureFlag, InsecureShorthandFlag, false,
		"Danger: skip invalid server certificates when connecting to a standalone container host")
}

// newClient returns a standalone host client only if the “--host” CLI flag
// was specified.
func newClient(cmd *cobra.Command) (csharg.SharkTank, error) {
	pf := cmd.PersistentFlags()
	// --host for a standalone container host capture...
	host, _ := pf.GetString(StandaloneHostFlag)
	if host == "" {
		return nil, nil
	}
	insecure, _ := pf.GetBool(InsecureFlag)
	opts := &csharg.SharkTankOnHostOptions{
		CommonClientOptions: csharg.CommonClientOptions{
			BearerToken: commands.BearerToken,
			Timeout:     commands.ReqTimeout,
		},
		InsecureSkipVerify: insecure,
	}
	return csharg.NewSharkTankOnHost(host, opts)
}
