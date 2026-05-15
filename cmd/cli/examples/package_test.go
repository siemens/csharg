// (c) Siemens AG 2026
//
// SPDX-License-Identifier: MIT

package examples

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestCmdExamples(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Csharg cmd/cli/examples package suite")
}
