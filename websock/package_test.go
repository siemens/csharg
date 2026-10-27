// (c) Siemens AG 2026
//
// SPDX-License-Identifier: MIT

package websock

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestWebsock(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Csharg websock package suite")
}
