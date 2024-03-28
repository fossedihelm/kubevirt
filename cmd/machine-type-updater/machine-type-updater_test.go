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
 * Copyright The Kubevirt Authors
 *
 */

package main

import (
	"context"
	"fmt"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"

	virtv1 "kubevirt.io/api/core/v1"
	"kubevirt.io/client-go/kubecli"

	"kubevirt.io/kubevirt/pkg/util"

	"kubevirt.io/kubevirt/pkg/libvmi"
)

const (
	machineTypeGlob        = "*glob8.*"
	machineTypeNeedsUpdate = "smth-glob8.10.0"
	machineTypeNoUpdate    = "smth-glob9.10.0"
)

var _ = Describe("MachineTypeUpdater", func() {
	var (
		ctrl               *gomock.Controller
		virtClient         *kubecli.MockKubevirtClient
		vmInterface        *kubecli.MockVirtualMachineInterface
		kubeClient         *fake.Clientset
		machineTypeUpdater *MachineTypeUpdater
	)

	shouldExpectPatchMachineType := func(vm *virtv1.VirtualMachine) {
		patchData := fmt.Sprintf(`[{"op":"test","path":"/spec/template/spec/domain/machine/type","value":"%s"},{"op":"remove","path":"/spec/template/spec/domain/machine","value":null}]`, vm.Spec.Template.Spec.Domain.Machine.Type)
		vmInterface.EXPECT().Patch(context.Background(), vm.Name, types.JSONPatchType, []byte(patchData), &v1.PatchOptions{}).Times(1)
	}

	expectVMListToNamespace := func(vm virtv1.VirtualMachine, namespace string) {
		virtClient.EXPECT().VirtualMachine(namespace).Return(vmInterface).AnyTimes()
		vmInterface.EXPECT().List(context.Background(), gomock.Any()).Times(1).Return(&virtv1.VirtualMachineList{Items: []virtv1.VirtualMachine{vm}}, nil)
	}

	expectVMListWithLabelSelector := func(vm virtv1.VirtualMachine, labelSelector string) {
		virtClient.EXPECT().VirtualMachine(vm.Namespace).Return(vmInterface).AnyTimes()
		vmInterface.EXPECT().List(context.Background(), &v1.ListOptions{LabelSelector: labelSelector}).Times(1).Return(&virtv1.VirtualMachineList{Items: []virtv1.VirtualMachine{vm}}, nil)
	}

	expectVMList := func(vm virtv1.VirtualMachine) {
		expectVMListToNamespace(vm, vm.Namespace)
	}

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		virtClient = kubecli.NewMockKubevirtClient(ctrl)
		vmInterface = kubecli.NewMockVirtualMachineInterface(ctrl)
		kubeClient = fake.NewSimpleClientset()
		virtClient.EXPECT().CoreV1().Return(kubeClient.CoreV1()).AnyTimes()
		EnvVarManager = &util.EnvVarManagerMock{}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	When("there is no MACHINE_TYPE environment variable set", func() {
		It("should return an error", func() {
			_, err := NewMachineTypeUpdater(virtClient)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(fmt.Errorf("no machine type was specified")))
		})
	})

	When("MACHINE_TYPE environment variable is set", func() {
		const badGlob = "[--"

		It("should return an error in case of syntax error in pattern", func() {
			EnvVarManager.Setenv(machineTypeEnvName, badGlob)
			EnvVarManager.Setenv(namespaceEnvName, v1.NamespaceDefault)
			_, err := NewMachineTypeUpdater(virtClient)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(fmt.Errorf("syntax error in pattern of %s environment variable, value \"%s\"", machineTypeEnvName, badGlob)))
		})

		When("glob is correct", func() {
			BeforeEach(func() {
				var err error
				EnvVarManager.Setenv(machineTypeEnvName, machineTypeGlob)
				EnvVarManager.Setenv(namespaceEnvName, v1.NamespaceDefault)
				machineTypeUpdater, err = NewMachineTypeUpdater(virtClient)
				Expect(err).ToNot(HaveOccurred())
				Expect(machineTypeUpdater.machineTypeGlob).To(BeEquivalentTo(machineTypeGlob))
			})

			DescribeTable("", func(machineType string, expectUpdate bool) {
				vmi := libvmi.New(
					libvmi.WithNamespace(v1.NamespaceDefault),
					withMachineType(machineType),
				)
				vm := libvmi.NewVirtualMachine(
					vmi,
				)
				expectVMList(*vm)
				if expectUpdate {
					shouldExpectPatchMachineType(vm)
				}
				machineTypeUpdater.Run()
			},
				Entry("should remove machineType if the vm machine type match", machineTypeNeedsUpdate, true),
				Entry("should not update machineType if the vm machine type does not match", machineTypeNoUpdate, false),
			)
		})
	})

	When("there is no NAMESPACE environment variable set", func() {
		BeforeEach(func() {
			EnvVarManager.Setenv(machineTypeEnvName, machineTypeGlob)
		})

		It("should return an error", func() {
			_, err := NewMachineTypeUpdater(virtClient)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(fmt.Errorf("no namespace was specified")))
		})

	})

	When("NAMESPACE environment variable is set", func() {
		const badNamespaceName = "bad namespace pattern"

		It("should return an error in case of syntax error", func() {
			EnvVarManager.Setenv(machineTypeEnvName, machineTypeGlob)
			EnvVarManager.Setenv(namespaceEnvName, badNamespaceName)
			_, err := NewMachineTypeUpdater(virtClient)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("syntax error in %s environment variable, value \"%s\"", namespaceEnvName, badNamespaceName)))
		})

		When("it is correct", func() {
			const namespaceName = "filter-namespace"

			BeforeEach(func() {
				var err error
				EnvVarManager.Setenv(machineTypeEnvName, machineTypeGlob)
				EnvVarManager.Setenv(namespaceEnvName, namespaceName)
				machineTypeUpdater, err = NewMachineTypeUpdater(virtClient)
				Expect(err).ToNot(HaveOccurred())
				Expect(machineTypeUpdater.namespace).To(BeEquivalentTo(namespaceName))
			})

			It("should only return vm in that namespace", func() {
				vmi := libvmi.New(
					libvmi.WithNamespace(namespaceName),
					withMachineType(machineTypeNoUpdate),
				)
				vm := libvmi.NewVirtualMachine(
					vmi,
				)
				expectVMListToNamespace(*vm, namespaceName)
				machineTypeUpdater.Run()
			})
		})
	})

	When("optional environment variables are not set", func() {
		BeforeEach(func() {
			EnvVarManager.Setenv(machineTypeEnvName, machineTypeGlob)
			EnvVarManager.Setenv(namespaceEnvName, v1.NamespaceDefault)
		})

		It("should use the default values", func() {
			updater, err := NewMachineTypeUpdater(virtClient)
			Expect(err).ToNot(HaveOccurred())
			Expect(updater.labelSelector).To(BeEquivalentTo(labels.Everything()))
			Expect(updater.restartRequired).To(BeFalse())
		})
	})

	When("RESTART_REQUIRED environment variable is set", func() {
		const badBoolean = "not_a_boolean"

		BeforeEach(func() {
			EnvVarManager.Setenv(machineTypeEnvName, machineTypeGlob)
			EnvVarManager.Setenv(namespaceEnvName, v1.NamespaceDefault)
		})

		It("should return an error in case of not boolean value", func() {
			EnvVarManager.Setenv(restartRequiredEnvName, badBoolean)
			_, err := NewMachineTypeUpdater(virtClient)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("error parsing %s environment variable, value \"%s\"", restartRequiredEnvName, badBoolean)))
		})

		When("it is true", func() {
			BeforeEach(func() {
				var err error
				EnvVarManager.Setenv(machineTypeEnvName, machineTypeGlob)
				EnvVarManager.Setenv(namespaceEnvName, v1.NamespaceDefault)
				EnvVarManager.Setenv(restartRequiredEnvName, "true")
				machineTypeUpdater, err = NewMachineTypeUpdater(virtClient)
				Expect(err).ToNot(HaveOccurred())
				Expect(machineTypeUpdater.restartRequired).To(BeTrue())
			})

			DescribeTable("", func(running bool) {
				vmi := libvmi.New(
					libvmi.WithNamespace(v1.NamespaceDefault),
					withMachineType(machineTypeNeedsUpdate),
				)
				var opts []libvmi.VMOption
				if running {
					opts = []libvmi.VMOption{
						libvmi.WithRunning(),
						withPrintableStatus(virtv1.VirtualMachineStatusRunning),
					}
				}
				vm := libvmi.NewVirtualMachine(
					vmi,
					opts...,
				)
				expectVMList(*vm)
				shouldExpectPatchMachineType(vm)
				if running {
					vmInterface.EXPECT().Restart(context.Background(), vm.Name, &virtv1.RestartOptions{}).Times(1)
				}
				machineTypeUpdater.Run()
			},
				Entry("should restart running vm after the patch", true),
				Entry("should not restart non-running vm after the patch", false),
			)
		})
	})

	When("LABEL_SELECTOR environment variable is set", func() {
		const badLabelSelector = "non_a_valid for create error"

		BeforeEach(func() {
			EnvVarManager.Setenv(machineTypeEnvName, machineTypeGlob)
			EnvVarManager.Setenv(namespaceEnvName, v1.NamespaceDefault)
		})

		It("should return an error in case of parsing error", func() {
			EnvVarManager.Setenv(labelSelectorEnvName, badLabelSelector)
			_, err := NewMachineTypeUpdater(virtClient)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("error parsing %s environment variable, value \"%s\"", labelSelectorEnvName, badLabelSelector)))
		})

		When("it is correct", func() {
			const labelSelector = "valid_label in (value1,value2)"

			BeforeEach(func() {
				var err error
				EnvVarManager.Setenv(machineTypeEnvName, machineTypeGlob)
				EnvVarManager.Setenv(namespaceEnvName, v1.NamespaceDefault)
				EnvVarManager.Setenv(labelSelectorEnvName, labelSelector)
				machineTypeUpdater, err = NewMachineTypeUpdater(virtClient)
				Expect(err).ToNot(HaveOccurred())
				Expect(machineTypeUpdater.labelSelector.String()).To(BeEquivalentTo(labelSelector))
			})

			It("should only return vm that matches label selector", func() {
				vmi := libvmi.New(
					libvmi.WithNamespace(v1.NamespaceDefault),
					withMachineType(machineTypeNoUpdate),
				)
				vm := libvmi.NewVirtualMachine(
					vmi,
					withLabel("valid_label", "value1"),
				)
				expectVMListWithLabelSelector(*vm, labelSelector)
				machineTypeUpdater.Run()
			})
		})
	})
})

func withMachineType(machineType string) libvmi.Option {
	return func(vmi *virtv1.VirtualMachineInstance) {
		if vmi.Spec.Domain.Machine == nil {
			vmi.Spec.Domain.Machine = &virtv1.Machine{}
		}

		vmi.Spec.Domain.Machine.Type = machineType
	}
}

func withPrintableStatus(status virtv1.VirtualMachinePrintableStatus) libvmi.VMOption {
	return func(vm *virtv1.VirtualMachine) {
		vm.Status.PrintableStatus = status
	}
}

func withLabel(key, value string) libvmi.VMOption {
	return func(vm *virtv1.VirtualMachine) {
		if vm.Labels == nil {
			vm.Labels = map[string]string{}
		}
		vm.Labels[key] = value
	}
}
