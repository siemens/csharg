// (c) Siemens AG 2026
//
// SPDX-License-Identifier: MIT

package commands

import (
	"bytes"
	"errors"
	"strings"

	"github.com/spf13/cobra"
	"github.com/thediveo/go-plugger/v3"

	"github.com/siemens/csharg/v2"
	"github.com/siemens/csharg/v2/api"
	"github.com/siemens/csharg/v2/cmd/cli/client"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// list runs "csharg list" with the specified args against the fake capture
// client st, returning stdout.
func list(st *fakeShark, args ...string) (string, error) {
	plugger.Group[client.New]().Register(func(*cobra.Command) (csharg.SharkTank, error) {
		return st, nil
	}, plugger.WithPlugin("fake"))
	root := &cobra.Command{Use: "csharg", SilenceUsage: true}
	listSetupCLI(root)
	out := &bytes.Buffer{}
	root.SetArgs(append([]string{"list"}, args...))
	root.SetOut(out)
	root.SetErr(GinkgoWriter)
	err := root.Execute()
	return out.String(), err
}

// rows returns the output lines with their column padding collapsed.
func rows(out string) []string {
	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	for i, line := range lines {
		lines[i] = strings.Join(strings.Fields(line), " ")
	}
	return lines
}

var _ = Describe("list command", func() {

	// deliberately unsorted.
	targets := api.Targets{
		{Name: "moby", Type: "docker", NodeName: "node-1", CaptureService: "pf-1"},
		{Name: "netw", Type: "bindmount", NodeName: "node-2", CaptureService: "pf-2"},
		{Name: "default/pod1", Type: "pod", NodeName: "node-1", CaptureService: "pf-1"},
		{Name: "init (1)", Type: "proc", NodeName: "node-2", CaptureService: "pf-2"},
	}

	BeforeEach(func() {
		grp := plugger.Group[client.New]()
		DeferCleanup(grp.Restore, grp.Backup())
		grp.Clear()
	})

	It("registers the list command with its alias", func() {
		root := &cobra.Command{Use: "csharg"}
		listSetupCLI(root)
		cmd, _, err := root.Find([]string{"ps"})
		Expect(err).NotTo(HaveOccurred())
		Expect(cmd.Name()).To(Equal("list"))
	})

	DescribeTable("lists targets",
		func(args []string, expected []string) {
			out, err := list(&fakeShark{targets: targets}, args...)
			Expect(err).NotTo(HaveOccurred())
			Expect(rows(out)).To(Equal(expected))
		},
		Entry("of all types", nil, []string{
			"TARGET TYPE NODE",
			"default/pod1 pod node-1",
			"init (1) proc node-2",
			"moby docker node-1",
			"netw bindmount node-2",
		}),
		Entry("of all types, wide", []string{"-o", "wide"}, []string{
			"TARGET TYPE NODE SERVICE",
			"default/pod1 pod node-1 pf-1",
			"init (1) proc node-2 pf-2",
			"moby docker node-1 pf-1",
			"netw bindmount node-2 pf-2",
		}),
		Entry("without headers", []string{"--no-headers"}, []string{
			"default/pod1 pod node-1",
			"init (1) proc node-2",
			"moby docker node-1",
			"netw bindmount node-2",
		}),
		Entry("of pods only", []string{"pods"}, []string{
			"POD",
			"default/pod1",
		}),
		Entry("of pods only, wide", []string{"pod", "-o", "wide"}, []string{
			"POD NODE",
			"default/pod1 node-1",
		}),
		Entry("of containers only", []string{"containers"}, []string{
			"TARGET TYPE NODE",
			"moby docker node-1",
		}),
		Entry("of networks only", []string{"network"}, []string{
			"TARGET TYPE NODE",
			"init (1) proc node-2",
			"netw bindmount node-2",
		}),
		Entry("of pods and containers", []string{"pods", "container"}, []string{
			"TARGET TYPE NODE",
			"default/pod1 pod node-1",
			"moby docker node-1",
		}),
		Entry("by name", []string{"-o", "name"}, []string{
			"default/pod1",
			"init (1)",
			"moby",
			"netw",
		}),
		Entry("unsorted", []string{"-o", "name", "--sort-by", ""}, []string{
			"moby",
			"netw",
			"default/pod1",
			"init (1)",
		}),
		Entry("using custom columns", []string{"-o", "custom-columns=N:{.Name},S:{.CaptureService}", "networks"}, []string{
			"N S",
			"init (1) pf-2",
			"netw pf-2",
		}),
	)

	It("lists targets as JSON", func() {
		out, err := list(&fakeShark{targets: targets}, "-o", "json", "containers")
		Expect(err).NotTo(HaveOccurred())
		Expect(out).To(MatchRegexp(`"name":\s*"moby"`))
		Expect(out).NotTo(MatchRegexp(`"name":\s*"netw"`))
	})

	It("lists targets as YAML", func() {
		out, err := list(&fakeShark{targets: targets}, "-o", "yaml", "containers")
		Expect(err).NotTo(HaveOccurred())
		Expect(out).To(MatchRegexp(`name:\s*moby`))
		Expect(out).NotTo(MatchRegexp(`name:\s*netw`))
	})

	It("lists targets using JSONPath", func() {
		Expect(list(&fakeShark{targets: targets}, "-o", "jsonpath={[*].name}", "networks")).
			To(Equal("netw init (1)"))
	})

	DescribeTable("rejects",
		func(st *fakeShark, args []string, expectedErr string) {
			Expect(list(st, args...)).Error().To(MatchError(ContainSubstring(expectedErr)))
		},
		Entry("an invalid target type", &fakeShark{}, []string{"foo"}, `invalid argument "foo"`),
		Entry("an invalid output format", &fakeShark{}, []string{"-o", "foo"}, "foo"),
		Entry("an invalid --sort-by", &fakeShark{}, []string{"--sort-by", "{.["}, "invalid --sort-by"),
		Entry("when targets can't be discovered", &fakeShark{targetsErr: errors.New("no targets")},
			nil, "no targets"),
	)

	It("reports a failing capture client", func() {
		grp := plugger.Group[client.New]()
		grp.Register(func(*cobra.Command) (csharg.SharkTank, error) {
			return nil, errors.New("no shark")
		}, plugger.WithPlugin("failing"))
		root := &cobra.Command{Use: "csharg", SilenceUsage: true}
		listSetupCLI(root)
		root.SetArgs([]string{"list"})
		root.SetOut(GinkgoWriter)
		root.SetErr(GinkgoWriter)
		Expect(root.Execute()).To(MatchError("no shark"))
	})

})
