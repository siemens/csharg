// (c) Siemens AG 2026
//
// SPDX-License-Identifier: MIT

package e2e

import (
	"context"
	"fmt"
	"time"

	"github.com/onsi/gomega/gexec"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("listing capture targets", func() {

	DescribeTable("lists a running container for different service URL forms",
		func(ctx context.Context, name, hostFormat string) {
			newTarget(ctx, name, pingScript)
			cli := runCshargOn(fmt.Sprintf(hostFormat, serviceAddr), "list", "containers", "--no-headers")
			Eventually(cli).WithTimeout(30 * time.Second).Should(gexec.Exit(0))
			Expect(cli.Out.Contents()).To(MatchRegexp(`(?m)^%s\s+docker\s+%s\s*$`, name, nodeName))
		},
		Entry("host:port", "csharg-e2e-list-plain", "%s"),
		Entry("http://host:port", "csharg-e2e-list-scheme", "http://%s"),
		Entry("http://host:port/", "csharg-e2e-list-slash", "http://%s/"),
	)

	It("lists only process-less network stacks when asked for networks", func(ctx context.Context) {
		const name = "csharg-e2e-list-networks"
		newTarget(ctx, name, pingScript)
		cli := runCsharg("list", "networks", "--no-headers")
		Eventually(cli).WithTimeout(30 * time.Second).Should(gexec.Exit(0))
		Expect(cli.Out.Contents()).NotTo(ContainSubstring(name))
		Expect(cli.Out.Contents()).NotTo(MatchRegexp(`(?m)\sdocker\s`))
	})

	It("reports an unreachable capture service", func() {
		cli := runCshargOn(nodeName+":1", "list")
		Eventually(cli).WithTimeout(30 * time.Second).Should(gexec.Exit(1))
		Expect(cli.Err.Contents()).To(ContainSubstring("querying targets from GhostWire-on-Packetflix service failed"))
	})

	It("reports a wrong service path", func() {
		cli := runCshargOn(serviceAddr+"/wrong", "list")
		Eventually(cli).WithTimeout(30 * time.Second).Should(gexec.Exit(1))
		Expect(cli.Err.Contents()).To(ContainSubstring("cannot decode targets from GhostWire-on-Packetflix service"))
	})

})
