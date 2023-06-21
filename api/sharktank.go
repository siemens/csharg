// (c) Siemens AG 2023
//
// SPDX-License-Identifier: MIT

// This statically typed data model matches the the JSON schema used for the
// "/list/json" service URL path of SharkTank capture services. This data model
// describes the discovered containers, pods, processes with their own IP
// stacks, and process-less IP stacks. Additionally, the data model supports
// KinD setups (as well as DinD), as well as other container system
// configurations.
//
// Additionally, we also use (and thus slightly extend) the data model to pass
// container (meta) data between invokations of our extcap plugin. For instance,
// while the container data model originally comes from the containers on a
// single container host, we add information necessary or beneficial when
// dealing with clusters of container hosts.

package api

// Targets is a list of containers, or capture targets.
type Targets []*Target

// Target describes various (more-or-less) interesting as well as extremely
// boring properties of containers, pods, processes, and process-less virtual IP
// stacks (network namespaces) that are of interest to capture as a service.
type Target struct {
	// The (node-local) name of container, (cluster-wide) podname, node-local
	// process name, or node-local filesystem name of a virtual IP stack
	// (network namespace). When it comes to integrated legacy devices,
	// "node-local" then refers to the service instance name instead of the
	// Kubernetes node. And for simplicity, we just lump up everything under the
	// "container" misnomer.
	Name string `json:"name"`
	// Type of target we're dealing with: containers such as "docker", "lxc",
	// etc., "pod", "proc", et cetera.
	Type string `json:"type"`
	// The node-local unique Linux kernel identifier of the virtual IP
	// stack/network namespace. This is simply the inode number of the network
	// namespace. Please note that inode numbers get recycled quickly, so the
	// same inode number might refer to a different network namespace at a
	// (much) later time.
	NetNS int `json:"netns"`
	// List of network interface names inside a specific network namespace.
	// Includes "lo".
	NetworkInterfaces []string `json:"network-interfaces"`
	// An optional (node-local) prefix to the name to cover situations with
	// Docker-in-Docker or multiple Docker side-by-side setups.
	Prefix string `json:"prefix"`

	// Start time after system boot of the "root" process inside the
	// container/network namespace. The start time together with the PID of this
	// "root" process allows for detecting stale and reused network namespace
	// identifiers. The start time is expressed in Linux kernel clock ticks, see
	// also: http://man7.org/linux/man-pages/man5/proc.5.html
	StartTime int64 `json:"starttime,omitempty"`
	// PID of the "root" process of a "container". Together with the StartTime
	// parameter this allows detecting stale or reused network namespace
	// identifiers.
	Pid int `json:"pid,omitempty"`

	// Name of the container host (which is the node name in Kubernetes
	// parlance). For integrated legacy devices, this will be the device name
	// instead of the Kubernetes node name, because there might be multiple
	// legacy capture device integration services on the same Kubernetes node.
	NodeName string `json:"node-name,omitempty"`
	// Optional cluster identity information.
	Cluster *Cluster `json:"cluster,omitempty"`
	// The particular SharkTank capture service name in a cluster. This allows
	// us to later correctly address the capture service responsible for this
	// container (or whatever ... such as integrated legacy devices).
	CaptureService string `json:"capture-service,omitempty"`
	// The (TCP/Websocket) port number of the capture service.
	CapturePort int32 `json:"captureport,omitempty"`
}

// Cluster gives details about the Kubernetes cluster a container belongs to.
type Cluster struct {
	// The name of a client-local context as used by the client to connect to a
	// Kubernetes cluster. Just to repeat: context names are client-local matter
	// and thus are not even guaranteed to be unique across different clients.
	Context string `json:"context"`
	// A (pseudo) unique identifier of a Kubernetes cluster, independent of the
	// non-unique cluster context. For the time being, it is the UID of the
	// "kube-system" namespace, as we can expect it to be not only unique across
	// different Kubernetes installations, but also constant over the lifetime
	// of a single Kubernetes installation.
	UID string `json:"uid,omitempty"`
}

// TargetDiscovery receives the information returned by the SharkTank cluster
// capture service at its "/list/json" REST API endpoint.
type TargetDiscovery struct {
	Targets Targets `json:"targets"`
}
