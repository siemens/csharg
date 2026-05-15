// (c) Siemens AG 2026
//
// SPDX-License-Identifier: MIT

package mutual

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestMutual(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Csharg cmd/cli/mutual package suite")
}
