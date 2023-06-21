// (c) Siemens AG 2023
//
// SPDX-License-Identifier: MIT

// Provides caching capture target descriptions and looking them up again.

package csharg

import (
	"sync"

	"github.com/siemens/csharg/api"
)

// TargetCache caches and indexes a set of capture targets. It can safely be
// accessed simultaneously by multiple go routines.
type TargetCache struct {
	// The list of capture target descriptions
	ts api.Targets
	// Map of (prefix, name) and (prefix, name, node) to the corresponding
	// capture target(s). In case of (prefix, name) there might be multiple
	// targets on different nodes (not for pods, but for standalone containers,
	// process-less IP stacks, et cetera).
	index map[targetkey]api.Targets
	m     sync.Mutex
}

// targetkey represents keys to the target index: prefix and name of a target.
// The node name is optional in that we will build the index to contain the same
// capture target both without the hosting node name, and with the node name in
// the keys.
type targetkey struct {
	prefix   string
	name     string
	nodename string
}

// IsEmpty returns true if the cache is empty, otherwise false.
func (tc *TargetCache) IsEmpty() bool {
	tc.m.Lock()
	defer tc.m.Unlock()
	return len(tc.ts) == 0
}

// Targets returns the list of capture target descriptions.
func (tc *TargetCache) Targets() api.Targets {
	tc.m.Lock()
	defer tc.m.Unlock()
	return tc.ts
}

// Pod returns the pod capture target with the specified prefix and name. For
// other types of targets the lookup will fail and return (nil, false). Use
// OnNode() instead when looking up capture targets that may occur multiple
// times inside a cluster on different cluster nodes.
func (tc *TargetCache) Pod(name string) (*api.Target, bool) {
	tc.m.Lock()
	defer tc.m.Unlock()
	if ts, ok := tc.index[targetkey{name: name}]; ok {
		// Only return a match if there is exactly one pod capture target;
		// otherwise, there is no match.
		if len(ts) == 1 && ts[0].Type == "pod" {
			return ts[0], true
		}
	}
	return nil, false
}

// OnNode returns the capture target with the given prefix+name and located on
// the specified cluster node. Use OnNode() when capturing from per-node
// targets, such as a kubelet, et cetera. For capturing from pods, use Pod()
// instead, as it doesn't need the specific nodename to be told.
func (tc *TargetCache) OnNode(nodename, prefix, name string) (*api.Target, bool) {
	tc.m.Lock()
	defer tc.m.Unlock()
	if ts, ok := tc.index[targetkey{nodename: nodename, prefix: prefix, name: name}]; ok {
		// Only return a match if there is exactly one capture target;
		// otherwise, there is no match.
		if len(ts) == 1 {
			return ts[0], true
		}
	}
	return nil, false
}

// Set the target descriptions to be cached.
func (tc *TargetCache) Set(ts api.Targets) {
	tc.m.Lock()
	defer tc.m.Unlock()
	tc.ts = ts
	// Also build an index of capture targets...
	tc.index = make(map[targetkey]api.Targets)
	for _, t := range ts {
		// Index the capture target just by its prefix+name.
		k := targetkey{
			prefix: t.Prefix,
			name:   t.Name,
		}
		// Pod targets can only appear once in a cluster, but other capture
		// targets might well appear multiple times with the same prefix+name,
		// on different nodes. So we allocate some more capacity for non-pod
		// targets.
		cap := 1
		if t.Type != "pod" {
			cap = 10
		}
		if ttt, ok := tc.index[k]; ok {
			tc.index[k] = append(ttt, t)
		} else {
			tc.index[k] = make(api.Targets, cap)
			tc.index[k][0] = t
		}
		// And now index the capture target by its nodename+prefix+name. This
		// combination can appear only once.
		k.nodename = t.NodeName
		tc.index[k] = api.Targets{t}
	}
}

// Clear the cached capture target descriptions.
func (tc *TargetCache) Clear() {
	// We're lazy and just empty the list of targets.
	tc.m.Lock()
	defer tc.m.Unlock()
	tc.ts = api.Targets{}
}
