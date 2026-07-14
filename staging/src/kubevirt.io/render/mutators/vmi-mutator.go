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

package mutators

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "kubevirt.io/api/core/v1"
	"kubevirt.io/client-go/log"
	"kubevirt.io/render/defaults"

	virtconfig "kubevirt.io/kubevirt/pkg/virt-config"
)

// ApplyNewVMIMutations applies all VMI mutations to a VMI object.
func ApplyNewVMIMutations(newVMI *v1.VirtualMachineInstance, clusterConfig *virtconfig.ClusterConfig) error {
	// Set VirtualMachineInstance defaults
	log.Log.Object(newVMI).V(4).Info("Apply defaults")
	if err := defaults.SetDefaultVirtualMachineInstance(clusterConfig, newVMI); err != nil {
		return err
	}

	if defaults.SupportsPCIeHotplug(&newVMI.Spec) {
		SetDefaultPciTopologyVersion(&newVMI.ObjectMeta)
	}

	if newVMI.Spec.Domain.CPU.IsolateEmulatorThread {
		_, emulatorThreadCompleteToEvenParityAnnotationExists := clusterConfig.GetConfigFromKubeVirtCR().Annotations[v1.EmulatorThreadCompleteToEvenParity]
		if emulatorThreadCompleteToEvenParityAnnotationExists && clusterConfig.AlignCPUsEnabled() {
			log.Log.V(4).Infof("Copy %s annotation from Kubevirt CR", v1.EmulatorThreadCompleteToEvenParity)
			if newVMI.Annotations == nil {
				newVMI.Annotations = map[string]string{}
			}
			newVMI.Annotations[v1.EmulatorThreadCompleteToEvenParity] = ""
		}
	}

	if IsTDXVMI(newVMI) {
		qgsSocketPath := clusterConfig.GetQGSSocketPath()
		if qgsSocketPath != "" {
			if newVMI.Annotations == nil {
				newVMI.Annotations = map[string]string{}
			}
			newVMI.Annotations[v1.QGSSocketPathAnnotation] = qgsSocketPath
		}
	}

	if !clusterConfig.RootEnabled() {
		markAsNonroot(newVMI)
	}

	return nil
}

func markAsNonroot(vmi *v1.VirtualMachineInstance) {
	vmi.Status.RuntimeUser = 107
}

// SetDefaultPciTopologyVersion sets the PCI topology version annotation to v3
// if not already present. Used by both VMI and VM mutating webhooks on CREATE.
func SetDefaultPciTopologyVersion(meta *metav1.ObjectMeta) {
	if _, exists := meta.Annotations[v1.PciTopologyVersionAnnotation]; exists {
		return
	}
	if meta.Annotations == nil {
		meta.Annotations = map[string]string{}
	}
	meta.Annotations[v1.PciTopologyVersionAnnotation] = v1.PciTopologyVersionV3
}

// Check if a VMI spec requests Intel TDX
func IsTDXVMI(vmi *v1.VirtualMachineInstance) bool {
	return vmi.Spec.Domain.LaunchSecurity != nil && vmi.Spec.Domain.LaunchSecurity.TDX != nil
}
