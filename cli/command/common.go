// (c) Siemens AG 2023
//
// SPDX-License-Identifier: MIT

// Implements the csharg "root" command with its global CLI flags.
// Additionally runs some checks on some of those global CLI flags, where
// necessary, so individual commands do not need to check them themselves.

package command

import (
	"time"

	"github.com/siemens/csharg/cli"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/thediveo/go-plugger/v3"
	"golang.org/x/exp/slices"
)

// Flag annotation for grouping mutually exclusive flags. Due to the open-ended
// plugin architecture of csharg we cannot directly use cobra's
// MarkFlagsMutuallyExclusive in plugins, but instead plugin need to annotate
// their flags and we then gather the groups with their flag members in order to
// issue MarkFlagsMutuallyExclusive as necessary.
const MutualFlagGroupAnnotation = "mutually-exclusive-group"

// ClientGroup is the name of an annotation value for flags that should be
// mutually exclusive for specifying capture service client endpoint
// information.
const ClientGroup = "sharktank"

// BearerToken specifies an optional user-supplied bearer token for
// authentication to be used with either the service URL.
var BearerToken string

// ReqTimeout specifies the length of time to wait before giving up on a single
// server request.
var ReqTimeout time.Duration

// rootCmd represents the Cobra "root" command thus the charg CLI itself.
var rootCmd = &cobra.Command{
	Use:   "csharg",
	Short: "Capture network traffic in Kubernetes clusters",
	Long: `csharg is a CLI tool for capturing live network traffic from various
capture targets, such as Kubernetes pods, standalone containers (Docker, but also
others), and also container-less network stacks.`,
	// See: https://github.com/spf13/cobra/issues/340
	SilenceUsage:  true,
	SilenceErrors: false,
	// Check mutually exclusive CLI args, ...
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Run the registered before-the-command plugins
		for _, beforeCmd := range plugger.Group[cli.BeforeCommand]().Symbols() {
			if err := beforeCmd(cmd); err != nil {
				return err
			}
		}
		return nil
	},
}

// SetupCLI registers the global ("persistent") CLI flags, as well as the
// (sub)commands. The individual commands are registered via a plugin-mechanism.
func SetupCLI() *cobra.Command {
	pf := rootCmd.PersistentFlags()

	pf.StringVar(&BearerToken, "token", "",
		"Bearer token for authentication to the API server or URL")
	pf.DurationVar(&ReqTimeout, "request-timeout", 0,
		`The length of time to wait before giving up on a single server request.
Non-zero values should contain a corresponding time unit (e.g. 1s, 2m, 3h).
A value of zero means don't timeout requests.`)

	// Call registered plugins in order to add further CLI args as well as
	// commands to the root command (or below).
	for _, setupCLI := range plugger.Group[cli.SetupCLI]().Symbols() {
		setupCLI(rootCmd)
	}
	// Set groups of mutually exclusive flags as annotated.
	mutuallyExclusives(rootCmd)
	// Fill in/expand command example sections, where additional command
	// examples are available.
	for _, cmd := range rootCmd.Commands() {
		examples := cli.Examples(cmd.Name())
		if examples == "" {
			continue
		}
		cmd.Example = examples
	}

	return rootCmd
}

// Annotate annotates the flag identified by name with the key=ann.
func Annotate(fs *pflag.FlagSet, flagname, key, ann string) {
	fs.SetAnnotation(flagname, key, []string{ann})
}

// exclusivesMap maps an "exclusive" group (name) to its mutually exclusive
// flags (names).
type exclusivesMap map[string][]string

// mutuallyExclusives starts with the specified command and collects mutually
// exclusive flags as identified by their annotations. It then configures them
// into their groups. This process then recursively repeats with each child
// command.
func mutuallyExclusives(cmd *cobra.Command) {
	exclusives := exclusivesMap{}
	cmd.MarkFlagsMutuallyExclusive() // hack: trigger merging if not already happened
	cmd.Flags().VisitAll(func(flag *pflag.Flag) {
		group := flag.Annotations[MutualFlagGroupAnnotation]
		if len(group) != 1 {
			return
		}
		name := flag.Name
		members := exclusives[group[0]]
		if slices.Contains(members, name) {
			return
		}
		exclusives[group[0]] = append(exclusives[group[0]], name)
	})
	for _, members := range exclusives {
		cmd.MarkFlagsMutuallyExclusive(members...)
	}
	for _, subcmd := range cmd.Commands() {
		mutuallyExclusives(subcmd)
	}
}
