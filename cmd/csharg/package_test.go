// (c) Siemens AG 2026
//
// SPDX-License-Identifier: MIT

package main

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestCmdCsharg(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Csharg cmd/csharg package suite")
}
