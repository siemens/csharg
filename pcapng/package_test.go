// (c) Siemens AG 2023
//
// SPDX-License-Identifier: MIT

// Sets up the test suite for unit testing our ClusterShark external capture
// plugin.

package pcapng

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestPcapng(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Csharg pcapng package suite")
}
