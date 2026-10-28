// (c) Siemens AG 2026
//
// SPDX-License-Identifier: MIT

package silent

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestCmdCliSilent(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Csharg cmd/cli/silent package suite")
}
