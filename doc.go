/*
Package csharg captures network traffic inside container hosts that are either
stand-alone or part of a Kubernetes cluster. These are live captures: the
captured network packets are immediately streamed to capture clients. There is
no need to record first, and then download; instead, you can analyse network
traffic live as it happens. So, Streaming Killed the Download Star – with
apologies to Trevor Horn.

Live network captures can be taken not only from standalone containers, but also
from Kubernetes pods, the node (host) network stacks, and even process-less
standalone virtual network stacks.

Please note that due to the “sandbox” design of Kubernetes pods, it is not
possible to capture from a single container inside a pod: all containers in the
same pod share the same network stack.

Optionally, network captures can be confined to only a subset of network
interfaces of a pod, container, et cetera. Furthermore, clients can specify
packet filters to apply at the sources, before sending the capture stream.
Please see struct CaptureOptions for details.

Normally, packet capture streaming will go on until you stop it. See the
examples for how to automatically stop a packet capture stream after a given
amount of time.
*/
package csharg
