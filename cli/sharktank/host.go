// (c) Siemens AG 2023
//
// SPDX-License-Identifier: MIT

package sharktank

import (
	"github.com/siemens/csharg"
	"github.com/siemens/csharg/cli"
	"github.com/siemens/csharg/cli/command"
	"github.com/spf13/cobra"
	"github.com/thediveo/go-plugger/v3"
)

// StandaloneHost specifies the hostname and port number of a discovery+capture
// service on a standalone container host.
var StandaloneHost string

// Insecure skips invalid server certificates.
var Insecure bool

func init() {
	plugger.Group[cli.SetupCLI]().Register(
		HostSetupCLI, plugger.WithPlugin("host"))
	plugger.Group[cli.NewClient]().Register(
		NewHostClient, plugger.WithPlugin("host"))
	plugger.Group[cli.CommandExamples]().Register(
		func() map[string]string {
			return map[string]string{
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

func HostSetupCLI(cmd *cobra.Command) {
	pf := cmd.PersistentFlags()
	pf.StringVar(&StandaloneHost, "host", "",
		`[http://|https://]hostname[:port][/path] of a Packetflix capture service
on a standalone container host`)
	command.Annotate(pf, "host", command.MutualFlagGroupAnnotation, command.ClientGroup)
	pf.BoolVarP(&Insecure, "insecure", "k", false,
		"Danger: skip invalid server certificates when connecting to a standalone container host")
}

func NewHostClient() (csharg.SharkTank, error) {
	// --host for a standalone container host capture...
	if StandaloneHost != "" {
		opts := &csharg.SharkTankOnHostOptions{
			CommonClientOptions: csharg.CommonClientOptions{
				BearerToken: command.BearerToken,
				Timeout:     command.ReqTimeout,
			},
			InsecureSkipVerify: Insecure,
		}
		return csharg.NewSharkTankOnHost(StandaloneHost, opts)
	}
	return nil, nil
}
