// (c) Siemens AG 2023
//
// SPDX-License-Identifier: MIT

// This statically typed data model matches the the JSON schema used for the
// "/mobyshark" service URL path of GhostWire services. This data model
// describes the discovered containers, pods, processes with their own IP
// stacks, and process-less IP stacks. Additionally, the data model supports
// DinD setups, as well as some other slightly arcane container system
// configuration.

package api

// GwTargetList describes the capture targets/containers discovered by a
// GhostWire “mobyshark” discovery service endpoint. We use a list of
// references, so we don't need to copy things around all the time ... in order
// to not get bitten by copies.
type GwTargetList struct {
	Targets Targets `json:"containers"`
}
