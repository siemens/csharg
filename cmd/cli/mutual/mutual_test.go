// (c) Siemens AG 2026
//
// SPDX-License-Identifier: MIT

package mutual

import (
	"github.com/spf13/cobra"
	"github.com/thediveo/clippy/cliplugin"
	"github.com/thediveo/go-plugger/v3"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func newCmd(name string) *cobra.Command {
	cmd := &cobra.Command{
		Use:  name,
		Args: cobra.NoArgs,
		Run:  func(*cobra.Command, []string) {},
	}
	cmd.SetOut(GinkgoWriter)
	cmd.SetErr(GinkgoWriter)
	return cmd
}

// execute builds a fresh command tree, as pflag never resets the “changed”
// state of flags, then sets up the mutually exclusive groups and finally runs
// the command tree with the specified args.
func execute(build func() *cobra.Command, args ...string) error {
	cmd := build()
	setupCLI(cmd)
	cmd.SetArgs(args)
	return cmd.Execute()
}

var _ = Describe("mutually exclusive flag groups", func() {

	It("registers as a CLI setup plugin", func() {
		Expect(plugger.Group[cliplugin.SetupCLI]().Plugins()).To(
			ContainElement("mutually-exclusive"))
	})

	It("annotates flags with their exclusive group", func() {
		cmd := newCmd("foo")
		cmd.Flags().Bool("a", false, "")
		ExclusiveInGroup(cmd.Flags(), "a", "grp")
		Expect(cmd.Flags().Lookup("a").Annotations).To(
			HaveKeyWithValue(mutualFlagGroupAnnotationKey, []string{"grp"}))
	})

	It("ignores annotating unknown flags", func() {
		cmd := newCmd("foo")
		Expect(func() { ExclusiveInGroup(cmd.Flags(), "nope", "grp") }).NotTo(Panic())
	})

	It("leaves flags without groups alone", func() {
		build := func() *cobra.Command {
			cmd := newCmd("foo")
			cmd.Flags().Bool("a", false, "")
			cmd.Flags().Bool("b", false, "")
			return cmd
		}
		Expect(execute(build, "--a", "--b")).To(Succeed())
	})

	It("rejects mutually exclusive flags from the same group", func() {
		build := func() *cobra.Command {
			cmd := newCmd("foo")
			fs := cmd.Flags()
			fs.Bool("a", false, "")
			fs.Bool("b", false, "")
			fs.Bool("c", false, "")
			ExclusiveInGroup(fs, "a", "grp")
			ExclusiveInGroup(fs, "b", "grp")
			return cmd
		}
		Expect(execute(build, "--a")).To(Succeed())
		Expect(execute(build, "--b")).To(Succeed())
		Expect(execute(build, "--a", "--c")).To(Succeed())
		Expect(execute(build, "--a", "--b")).To(
			MatchError(ContainSubstring("none of the others can be")))
	})

	It("allows flags from different groups", func() {
		build := func() *cobra.Command {
			cmd := newCmd("foo")
			fs := cmd.Flags()
			for _, name := range []string{"a1", "a2", "b1", "b2"} {
				fs.Bool(name, false, "")
			}
			ExclusiveInGroup(fs, "a1", "A")
			ExclusiveInGroup(fs, "a2", "A")
			ExclusiveInGroup(fs, "b1", "B")
			ExclusiveInGroup(fs, "b2", "B")
			return cmd
		}
		Expect(execute(build, "--a1", "--b2")).To(Succeed())
		Expect(execute(build, "--a1", "--a2")).To(
			MatchError(ContainSubstring("none of the others can be")))
		Expect(execute(build, "--b1", "--b2")).To(
			MatchError(ContainSubstring("none of the others can be")))
	})

	It("handles child commands and inherited persistent flags", func() {
		build := func() *cobra.Command {
			root := newCmd("root")
			pfs := root.PersistentFlags()
			pfs.Bool("a", false, "")
			pfs.Bool("b", false, "")
			ExclusiveInGroup(pfs, "a", "grp")
			ExclusiveInGroup(pfs, "b", "grp")

			child := newCmd("child")
			cfs := child.Flags()
			cfs.Bool("x", false, "")
			cfs.Bool("y", false, "")
			ExclusiveInGroup(cfs, "x", "kid")
			ExclusiveInGroup(cfs, "y", "kid")
			root.AddCommand(child)
			return root
		}
		Expect(execute(build, "child", "--a", "--x")).To(Succeed())
		Expect(execute(build, "child", "--a", "--b")).To(
			MatchError(ContainSubstring("none of the others can be")))
		Expect(execute(build, "child", "--x", "--y")).To(
			MatchError(ContainSubstring("none of the others can be")))
		Expect(execute(build, "--a", "--b")).To(
			MatchError(ContainSubstring("none of the others can be")))
	})

})
