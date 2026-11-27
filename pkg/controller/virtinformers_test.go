/*
 * This file is part of the KubeVirt project
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 * Copyright The KubeVirt Authors.
 *
 */

package controller

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"

	v1 "kubevirt.io/api/core/v1"
)

var _ = Describe("VMIsOnNode", func() {
	const (
		host      = "thisnode"
		otherHost = "othernode"
	)

	newVMI := func(name, node string) *v1.VirtualMachineInstance {
		return &v1.VirtualMachineInstance{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: metav1.NamespaceDefault},
			Status:     v1.VirtualMachineInstanceStatus{NodeName: node},
		}
	}

	var indexer cache.Indexer

	BeforeEach(func() {
		indexer = cache.NewIndexer(cache.MetaNamespaceKeyFunc, GetVMIInformerIndexers())
	})

	It("should return the VMIs running on the node", func() {
		Expect(indexer.Add(newVMI("local-1", host))).To(Succeed())
		Expect(indexer.Add(newVMI("local-2", host))).To(Succeed())
		Expect(indexer.Add(newVMI("remote", otherHost))).To(Succeed())

		vmis, err := VMIsOnNode(indexer, host)
		Expect(err).NotTo(HaveOccurred())
		Expect(vmis).To(ConsistOf(newVMI("local-1", host), newVMI("local-2", host)))
	})

	It("should return no VMIs for a node that runs none", func() {
		Expect(indexer.Add(newVMI("remote", otherHost))).To(Succeed())

		vmis, err := VMIsOnNode(indexer, host)
		Expect(err).NotTo(HaveOccurred())
		Expect(vmis).To(BeEmpty())
	})

	// A migration hands a VMI over by writing Status.NodeName and the node
	// label in the same update; the node index follows the status, which is
	// what makes it stand in for the node-scoped informers' label selector.
	// The moved VMI is a fresh object: entries in an informer cache are shared
	// pointers and must not be mutated in place.
	It("should follow the VMI when its node changes", func() {
		Expect(indexer.Add(newVMI("migrated", host))).To(Succeed())
		moved := newVMI("migrated", otherHost)
		Expect(indexer.Update(moved)).To(Succeed())

		vmis, err := VMIsOnNode(indexer, host)
		Expect(err).NotTo(HaveOccurred())
		Expect(vmis).To(BeEmpty())

		vmis, err = VMIsOnNode(indexer, otherHost)
		Expect(err).NotTo(HaveOccurred())
		Expect(vmis).To(ConsistOf(moved))
	})
})
