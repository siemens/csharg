// (c) Siemens AG 2026
//
// SPDX-License-Identifier: MIT

package client

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestCmdCliClient(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Csharg cmd/cli/client package suite")
}
