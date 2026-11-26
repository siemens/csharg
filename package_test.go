// (c) Siemens AG 2026
//
// SPDX-License-Identifier: MIT

package csharg

import (
	"io"
	"log/slog"
	"testing"

	"github.com/siemens/csharg/v2/api"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestCsharg(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Csharg package suite")
}

// discoverTargets explicitly discovers the capture targets of st.
func discoverTargets(st SharkTank) error {
	_, err := st.Targets()
	return err
}

// captureDiscovered captures from a target lacking network interfaces, so that
// st needs to discover the capture targets first.
func captureDiscovered(st SharkTank) error {
	_, err := st.Capture(io.Discard, &api.Target{Name: "foo"}, nil)
	return err
}

// captureHandshake captures from a target with network interfaces, so that st
// immediately attempts the capture WebSocket handshake; as rejected handshakes
// get logged, this logs to the GinkgoWriter.
func captureHandshake(st SharkTank) error {
	DeferCleanup(slog.SetDefault, slog.Default())
	slog.SetDefault(slog.New(slog.NewTextHandler(GinkgoWriter, nil)))
	_, err := st.Capture(io.Discard, &api.Target{Name: "foo", NetworkInterfaces: []string{"eth0"}}, nil)
	return err
}
