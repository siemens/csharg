// (c) Siemens AG 2026
//
// SPDX-License-Identifier: MIT

package mutual

import (
	"slices"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/thediveo/clippy/cliplugin"
	"github.com/thediveo/go-plugger/v3"
)

func init() {
	plugger.Group[cliplugin.SetupCLI]().Register(setupCLI,
		plugger.WithPlacement(">"), // run last
		plugger.WithPlugin("mutually-exclusive"))
}

// mutualFlagGroupAnnotationKey is a flag annotation for grouping mutually
// exclusive flags: due to the open-ended plugin architecture of csharg we
// cannot directly use cobra's MarkFlagsMutuallyExclusive in plugins, but
// instead such plugins need to annotate their flags and we then gather the
// groups with their flag members in order to issue MarkFlagsMutuallyExclusive
// as necessary.
const mutualFlagGroupAnnotationKey = "mutually-exclusive-group"

// ExclusiveInGroup marks the specified flag in the passed FlagSet as
// mutually exclusive to other marked flags in the same specified group.
func ExclusiveInGroup(fs *pflag.FlagSet, flagname string, group string) {
	fs.SetAnnotation(flagname, mutualFlagGroupAnnotationKey, []string{group})
}

func setupCLI(cmd *cobra.Command) {
	collectExclusives(cmd)
}

// exclusiveFlagsByGroup maps an “exclusive” group (name) to its mutually
// exclusive flags (names).
type exclusiveFlagsByGroup map[string][]string

// collectExclusives starts with the specified command and collects mutually
// exclusive flags as identified by their annotations. It then configures them
// into their groups. This process then recursively repeats with each child
// command.
func collectExclusives(cmd *cobra.Command) {
	exclusives := exclusiveFlagsByGroup{}
	cmd.MarkFlagsMutuallyExclusive() // hack: trigger merging if not already happened
	cmd.Flags().VisitAll(func(flag *pflag.Flag) {
		group := flag.Annotations[mutualFlagGroupAnnotationKey]
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
		collectExclusives(subcmd)
	}
}
