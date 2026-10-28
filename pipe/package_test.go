// (c) Siemens AG 2023
//
// SPDX-License-Identifier: MIT

package pipe

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestPipe(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Csharg pipe package suite")
}
