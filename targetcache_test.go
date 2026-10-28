// (c) Siemens AG 2026
//
// SPDX-License-Identifier: MIT

package csharg

import (
	"github.com/siemens/csharg/v2/api"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func onNode(tc *TargetCache, nodename, prefix, name string) *api.Target {
	GinkgoHelper()
	t, ok := tc.OnNode(nodename, prefix, name)
	Expect(ok).To(BeTrue())
	return t
}

var _ = Describe("target cache", func() {

	var (
		pod  *api.Target
		cntr *api.Target
		dupe *api.Target
		tc   *TargetCache
	)

	BeforeEach(func() {
		pod = &api.Target{Name: "default/foo", Type: "pod", NodeName: "node1"}
		cntr = &api.Target{Name: "bar", Type: "docker", NodeName: "node1"}
		dupe = &api.Target{Name: "bar", Type: "docker", NodeName: "node2"}
		tc = &TargetCache{}
	})

	It("starts empty and can be cleared", func() {
		Expect(tc.IsEmpty()).To(BeTrue())
		tc.Set(api.Targets{pod})
		Expect(tc.IsEmpty()).To(BeFalse())
		Expect(tc.Targets()).To(HaveExactElements(pod))
		tc.Clear()
		Expect(tc.IsEmpty()).To(BeTrue())
		Expect(tc.Targets()).To(BeEmpty())
	})

	It("doesn't look up targets after clearing", func() {
		tc.Set(api.Targets{pod, cntr})
		tc.Clear()
		_, ok := tc.Pod("", pod.Name)
		Expect(ok).To(BeFalse())
		_, ok = tc.OnNode("node1", "", cntr.Name)
		Expect(ok).To(BeFalse())
	})

	It("indexes targets without nil entries", func() {
		tc.Set(api.Targets{pod, cntr, dupe})
		Expect(tc.index[targetkey{name: pod.Name}]).To(HaveExactElements(pod))
		Expect(tc.index[targetkey{name: cntr.Name}]).To(HaveExactElements(cntr, dupe))
		Expect(tc.index[targetkey{name: cntr.Name, nodename: "node1"}]).To(HaveExactElements(cntr))
		Expect(tc.index[targetkey{name: dupe.Name, nodename: "node2"}]).To(HaveExactElements(dupe))
	})

	It("looks up pods", func() {
		tc.Set(api.Targets{pod, cntr})
		t, ok := tc.Pod("", pod.Name)
		Expect(ok).To(BeTrue())
		Expect(t).To(Equal(pod))
		_, ok = tc.Pod("", "default/nada")
		Expect(ok).To(BeFalse())
	})

	It("looks up pods with prefixes", func() {
		prefixed := &api.Target{Name: pod.Name, Type: "pod", Prefix: "kind", NodeName: "node1"}
		tc.Set(api.Targets{prefixed})
		t, ok := tc.Pod("kind", pod.Name)
		Expect(ok).To(BeTrue())
		Expect(t).To(Equal(prefixed))
		_, ok = tc.Pod("", pod.Name)
		Expect(ok).To(BeFalse())
	})

	It("doesn't look up non-pods as pods", func() {
		tc.Set(api.Targets{cntr})
		_, ok := tc.Pod("", cntr.Name)
		Expect(ok).To(BeFalse())
	})

	It("looks up targets on nodes", func() {
		tc.Set(api.Targets{pod, cntr, dupe})
		Expect(onNode(tc, "node1", "", cntr.Name)).To(Equal(cntr))
		Expect(onNode(tc, "node2", "", dupe.Name)).To(Equal(dupe))
		Expect(onNode(tc, "node1", "", pod.Name)).To(Equal(pod))
		_, ok := tc.OnNode("node3", "", cntr.Name)
		Expect(ok).To(BeFalse())
	})

	It("distinguishes prefixes", func() {
		prefixed := &api.Target{Name: "bar", Type: "docker", Prefix: "dind", NodeName: "node1"}
		tc.Set(api.Targets{cntr, prefixed})
		Expect(onNode(tc, "node1", "dind", "bar")).To(Equal(prefixed))
		Expect(onNode(tc, "node1", "", "bar")).To(Equal(cntr))
	})

	It("isolates the cached targets from callers", func() {
		pod.NetworkInterfaces = []string{"eth0"}
		pod.Cluster = &api.Cluster{UID: "42"}
		ts := api.Targets{pod}
		tc.Set(ts)
		pristine := *pod
		pristine.NetworkInterfaces = []string{"eth0"}
		pristine.Cluster = &api.Cluster{UID: "42"}

		ts[0] = cntr
		pod.Name = "default/fooled"
		pod.NetworkInterfaces[0] = "fooled"
		pod.Cluster.UID = "fooled"
		Expect(tc.Targets()).To(HaveExactElements(&pristine))

		cached := tc.Targets()
		cached[0].NetworkInterfaces[0] = "fooled"
		cached[0].Cluster.UID = "fooled"
		cached[0] = cntr
		t, ok := tc.Pod("", pristine.Name)
		Expect(ok).To(BeTrue())
		Expect(t).To(Equal(&pristine))

		t.Name = "default/fooled"
		Expect(onNode(tc, "node1", "", pristine.Name)).To(Equal(&pristine))
	})

})
