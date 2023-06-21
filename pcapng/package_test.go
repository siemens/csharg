// (c) Siemens AG 2023
//
// SPDX-License-Identifier: MIT

// Sets up the test suite for unit testing our ClusterShark external capture
// plugin.

package pcapng

import (
	"testing"

	log "github.com/sirupsen/logrus"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestPcapng(t *testing.T) {
	log.SetLevel(log.DebugLevel)

	RegisterFailHandler(Fail)
	RunSpecs(t, "Csharg pcapng package suite")
}
