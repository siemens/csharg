// (c) Siemens AG 2026
//
// SPDX-License-Identifier: MIT

// Sets up the test suite for unit testing our ClusterShark external capture
// plugin.

package cli

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestCmdCli(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Csharg cmd/cli package suite")
}
