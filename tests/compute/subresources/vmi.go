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
 * Copyright The KubeVirt Authors
 *
 */

package compute

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "kubevirt.io/api/core/v1"
	"kubevirt.io/client-go/kubecli"

	"kubevirt.io/kubevirt/pkg/libvmi"
	"kubevirt.io/kubevirt/tests/compute"
	"kubevirt.io/kubevirt/tests/framework/kubevirt"
	"kubevirt.io/kubevirt/tests/framework/matcher"
	"kubevirt.io/kubevirt/tests/libnet"
	"kubevirt.io/kubevirt/tests/libvmifact"
	"kubevirt.io/kubevirt/tests/libwait"
	"kubevirt.io/kubevirt/tests/testsuite"
)

var _ = compute.SIGDescribe("VirtualMachineInstance subresource", func() {
	var virtClient kubecli.KubevirtClient

	BeforeEach(func() {
		virtClient = kubevirt.Client()
	})

	Context("Freeze Unfreeze should fail", func() {
		var vm *v1.VirtualMachine

		BeforeEach(func() {
			var err error
			vmi := libvmifact.NewCirros()
			vm = libvmi.NewVirtualMachine(vmi, libvmi.WithRunning())
			vm, err = virtClient.VirtualMachine(testsuite.GetTestNamespace(vmi)).Create(context.Background(), vm, metav1.CreateOptions{})
			Expect(err).ToNot(HaveOccurred())
			Eventually(func() bool {
				vmi, err = virtClient.VirtualMachineInstance(vm.Namespace).Get(context.Background(), vm.Name, metav1.GetOptions{})
				if errors.IsNotFound(err) {
					return false
				}
				Expect(err).ToNot(HaveOccurred())
				return vmi.Status.Phase == v1.Running
			}, 180*time.Second, time.Second).Should(BeTrue())
			libwait.WaitForSuccessfulVMIStart(vmi,
				libwait.WithTimeout(180),
			)
		})

		It("[test_id:7476]Freeze without guest agent", func() {
			expectedErr := "Internal error occurred"
			err := virtClient.VirtualMachineInstance(testsuite.GetTestNamespace(vm)).Freeze(context.Background(), vm.Name, 0)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(expectedErr))
		})

		It("[test_id:7477]Unfreeze without guest agent", func() {
			expectedErr := "Internal error occurred"
			err := virtClient.VirtualMachineInstance(testsuite.GetTestNamespace(vm)).Unfreeze(context.Background(), vm.Name)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(expectedErr))
		})
	})

	Context("Freeze Unfreeze commands", func() {
		var vm *v1.VirtualMachine

		BeforeEach(func() {
			var err error
			vmi := libvmifact.NewFedora(libnet.WithMasqueradeNetworking())
			vmi.Namespace = testsuite.GetTestNamespace(vmi)
			vm = libvmi.NewVirtualMachine(vmi, libvmi.WithRunning())
			vm, err = virtClient.VirtualMachine(testsuite.GetTestNamespace(vmi)).Create(context.Background(), vm, metav1.CreateOptions{})
			Expect(err).ToNot(HaveOccurred())
			Eventually(func() bool {
				vmi, err = virtClient.VirtualMachineInstance(vm.Namespace).Get(context.Background(), vm.Name, metav1.GetOptions{})
				if errors.IsNotFound(err) {
					return false
				}
				Expect(err).ToNot(HaveOccurred())
				return vmi.Status.Phase == v1.Running
			}, 180*time.Second, time.Second).Should(BeTrue())
			libwait.WaitForSuccessfulVMIStart(vmi,
				libwait.WithTimeout(300),
			)
			Eventually(matcher.ThisVMI(vmi), 12*time.Minute, 2*time.Second).Should(matcher.HaveConditionTrue(v1.VirtualMachineInstanceAgentConnected))
		})

		waitVMIFSFreezeStatus := func(expectedStatus string) {
			Eventually(func() bool {
				updatedVMI, err := virtClient.VirtualMachineInstance(vm.Namespace).Get(context.Background(), vm.Name, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())
				return updatedVMI.Status.FSFreezeStatus == expectedStatus
			}, 30*time.Second, 2*time.Second).Should(BeTrue())
		}

		It("[test_id:7479]Freeze Unfreeze should succeed", func() {
			By("Freezing VMI")
			err := virtClient.VirtualMachineInstance(testsuite.GetTestNamespace(vm)).Freeze(context.Background(), vm.Name, 0)
			Expect(err).ToNot(HaveOccurred())

			waitVMIFSFreezeStatus("frozen")

			By("Unfreezing VMI")
			err = virtClient.VirtualMachineInstance(testsuite.GetTestNamespace(vm)).Unfreeze(context.Background(), vm.Name)
			Expect(err).ToNot(HaveOccurred())

			waitVMIFSFreezeStatus("")
		})

		It("[test_id:7480]Multi Freeze Unfreeze calls should succeed", func() {
			for i := 0; i < 5; i++ {
				By("Freezing VMI")
				err := virtClient.VirtualMachineInstance(testsuite.GetTestNamespace(vm)).Freeze(context.Background(), vm.Name, 0)
				Expect(err).ToNot(HaveOccurred())

				waitVMIFSFreezeStatus("frozen")
			}

			By("Unfreezing VMI")
			for i := 0; i < 5; i++ {
				err := virtClient.VirtualMachineInstance(testsuite.GetTestNamespace(vm)).Unfreeze(context.Background(), vm.Name)
				Expect(err).ToNot(HaveOccurred())

				waitVMIFSFreezeStatus("")
			}
		})

		It("Freeze without Unfreeze should trigger unfreeze after timeout", func() {
			By("Freezing VMI")
			unfreezeTimeout := 10 * time.Second
			err := virtClient.VirtualMachineInstance(testsuite.GetTestNamespace(vm)).Freeze(context.Background(), vm.Name, unfreezeTimeout)
			Expect(err).ToNot(HaveOccurred())

			waitVMIFSFreezeStatus("frozen")

			By("Wait Unfreeze VMI to be triggered")
			waitVMIFSFreezeStatus("")
		})
	})
})
