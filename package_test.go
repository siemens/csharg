// (c) Siemens AG 2026
//
// SPDX-License-Identifier: MIT

package csharg

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestCsharg(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Csharg package suite")
}
