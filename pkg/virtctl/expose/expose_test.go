package expose_test

import (
	"context"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gstruct"
	"github.com/onsi/gomega/types"

	k8sv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/fake"
	v1 "kubevirt.io/api/core/v1"
	kubevirtfake "kubevirt.io/client-go/generated/kubevirt/clientset/versioned/fake"
	"kubevirt.io/client-go/kubecli"

	"kubevirt.io/kubevirt/pkg/libvmi"
	"kubevirt.io/kubevirt/pkg/pointer"
	"kubevirt.io/kubevirt/pkg/virtctl/expose"
	"kubevirt.io/kubevirt/tests/clientcmd"
)

var _ = Describe("Expose", func() {
	var (
		kubeClient *fake.Clientset
		virtClient *kubevirtfake.Clientset
	)

	BeforeEach(func() {
		kubeClient = fake.NewSimpleClientset()
		virtClient = kubevirtfake.NewSimpleClientset()

		ctrl := gomock.NewController(GinkgoT())
		kubecli.GetKubevirtClientFromClientConfig = kubecli.GetMockKubevirtClientFromClientConfig
		kubecli.MockKubevirtClientInstance = kubecli.NewMockKubevirtClient(ctrl)

		kubecli.MockKubevirtClientInstance.EXPECT().VirtualMachineInstance(metav1.NamespaceDefault).
			Return(virtClient.KubevirtV1().VirtualMachineInstances(metav1.NamespaceDefault)).AnyTimes()
		kubecli.MockKubevirtClientInstance.EXPECT().VirtualMachine(metav1.NamespaceDefault).
			Return(virtClient.KubevirtV1().VirtualMachines(metav1.NamespaceDefault)).AnyTimes()
		kubecli.MockKubevirtClientInstance.EXPECT().ReplicaSet(metav1.NamespaceDefault).
			Return(virtClient.KubevirtV1().VirtualMachineInstanceReplicaSets(metav1.NamespaceDefault)).AnyTimes()
		kubecli.MockKubevirtClientInstance.EXPECT().CoreV1().Return(kubeClient.CoreV1()).AnyTimes()
	})

	Context("should fail", func() {
		DescribeTable("with invalid argument count", func(args ...string) {
			err := runCommand(args...)
			Expect(err).To(MatchError(ContainSubstring("accepts 2 arg(s), received")))
		},
			Entry("no arguments"),
			Entry("single argument", "vmi"),
			Entry("three arguments", "vmi", "test", "invalid"),
		)

		It("with invalid resource type", func() {
			err := runCommand("kaboom", "my-vm", "--name", "my-service")
			Expect(err).To(MatchError("unsupported resource type: kaboom"))
		})

		It("with unknown flag", func() {
			err := runCommand("vmi", "my-vm", "--name", "my-service", "--kaboom")
			Expect(err).To(MatchError("unknown flag: --kaboom"))
		})

		It("missing --name flag", func() {
			err := runCommand("vmi", "my-vm")
			Expect(err).To(MatchError("required flag(s) \"name\" not set"))
		})

		DescribeTable("invalid flag value", func(arg, errMsg string) {
			err := runCommand("vmi", "my-vm", "--name", "my-service", arg)
			Expect(err).To(MatchError(errMsg))
		},
			Entry("invalid protocol", "--protocol=madeup", "unknown protocol: madeup"),
			Entry("invalid service type", "--type=madeup", "unknown service type: madeup"),
			Entry("service type externalname", "--type=externalname", "type: externalname not supported"),
			Entry("invalid ip family", "--ip-family=madeup", "unknown IPFamily/s: madeup"),
			Entry("invalid ip family policy", "--ip-family-policy=madeup", "unknown IPFamilyPolicy/s: madeup"),
		)

		It("when client has an error", func() {
			kubecli.GetKubevirtClientFromClientConfig = kubecli.GetInvalidKubevirtClientFromClientConfig
			err := runCommand("vmi", "my-vm", "--name", "my-service")
			Expect(err).To(MatchError(ContainSubstring("cannot obtain KubeVirt client")))
		})

		DescribeTable("with missing resource", func(resource, errMsg string) {
			err := runCommand(resource, "unknown", "--name", "my-service")
			Expect(err).To(MatchError(ContainSubstring(errMsg)))
		},
			Entry("vmi", "vmi", "virtualmachineinstances.kubevirt.io \"unknown\" not found"),
			Entry("vm", "vm", "virtualmachines.kubevirt.io \"unknown\" not found"),
			Entry("vmirs", "vmirs", "virtualmachineinstancereplicasets.kubevirt.io \"unknown\" not found"),
		)

		It("with missing port and missing pod network ports", func() {
			vmi := libvmi.New(libvmi.WithLabel("key", "value"))
			vmi, err := virtClient.KubevirtV1().VirtualMachineInstances(metav1.NamespaceDefault).Create(context.Background(), vmi, metav1.CreateOptions{})
			Expect(err).ToNot(HaveOccurred())
			err = runCommand("vmi", vmi.Name, "--name", "my-service")
			Expect(err).To(MatchError("couldn't find port via --port flag or introspection"))
		})

		Context("when labels are missing", func() {
			It("with VirtualMachineInstance", func() {
				vmi := libvmi.New()
				vmi, err := virtClient.KubevirtV1().VirtualMachineInstances(metav1.NamespaceDefault).Create(context.Background(), vmi, metav1.CreateOptions{})
				Expect(err).ToNot(HaveOccurred())
				err = runCommand("vmi", vmi.Name, "--name", "my-service")
				Expect(err).To(MatchError(ContainSubstring("cannot expose vmi without any label")))
			})

			It("with VirtualMachineInstance and unwanted labels", func() {
				vmi := libvmi.New(
					libvmi.WithLabel(v1.NodeNameLabel, "value"),
					libvmi.WithLabel(v1.VirtualMachinePoolRevisionName, "value"),
					libvmi.WithLabel(v1.MigrationTargetNodeNameLabel, "value"),
				)
				vmi, err := virtClient.KubevirtV1().VirtualMachineInstances(metav1.NamespaceDefault).Create(context.Background(), vmi, metav1.CreateOptions{})
				Expect(err).ToNot(HaveOccurred())
				err = runCommand("vmi", vmi.Name, "--name", "my-service")
				Expect(err).To(MatchError(ContainSubstring("cannot expose vmi without any label")))
			})

			It("with VirtualMachine", func() {
				vm := libvmi.NewVirtualMachine(libvmi.New())
				vm, err := virtClient.KubevirtV1().VirtualMachines(metav1.NamespaceDefault).Create(context.Background(), vm, metav1.CreateOptions{})
				Expect(err).ToNot(HaveOccurred())
				err = runCommand("vm", vm.Name, "--name", "my-service")
				Expect(err).To(MatchError(ContainSubstring("cannot expose vm without any label")))
			})

			It("with VirtualMachine and unwanted labels", func() {
				vm := libvmi.NewVirtualMachine(libvmi.New(
					libvmi.WithLabel(v1.VirtualMachinePoolRevisionName, "value"),
				))
				vm, err := virtClient.KubevirtV1().VirtualMachines(metav1.NamespaceDefault).Create(context.Background(), vm, metav1.CreateOptions{})
				Expect(err).ToNot(HaveOccurred())
				err = runCommand("vm", vm.Name, "--name", "my-service")
				Expect(err).To(MatchError(ContainSubstring("cannot expose vm without any label")))
			})

			It("with VirtualMachineInstanceReplicaSet", func() {
				vmirs := kubecli.NewMinimalVirtualMachineInstanceReplicaSet("vmirs")
				vmirs, err := virtClient.KubevirtV1().VirtualMachineInstanceReplicaSets(metav1.NamespaceDefault).Create(context.Background(), vmirs, metav1.CreateOptions{})
				Expect(err).ToNot(HaveOccurred())
				err = runCommand("vmirs", vmirs.Name, "--name", "my-service")
				Expect(err).To(MatchError(ContainSubstring("cannot expose vmirs without any label")))
			})
		})

		It("when VirtualMachineInstanceReplicaSet has MatchExpressions", func() {
			vmirs := kubecli.NewMinimalVirtualMachineInstanceReplicaSet("vmirs")
			vmirs.Spec.Selector = &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{Key: "test"},
				},
			}
			vmirs, err := virtClient.KubevirtV1().VirtualMachineInstanceReplicaSets(metav1.NamespaceDefault).Create(context.Background(), vmirs, metav1.CreateOptions{})
			Expect(err).ToNot(HaveOccurred())
			err = runCommand("vmirs", vmirs.Name, "--name", "my-service")
			Expect(err).To(MatchError(ContainSubstring("cannot expose VirtualMachineInstanceReplicaSet with match expressions")))
		})
	})

	Context("should succeed", func() {
		const (
			labelKey   = "my-key"
			labelValue = "my-value"

			serviceName    = "my-service"
			servicePort    = int32(9999)
			servicePortStr = "9999"
		)

		var (
			vmi       *v1.VirtualMachineInstance
			vm        *v1.VirtualMachine
			vmirs     *v1.VirtualMachineInstanceReplicaSet
			resources map[string]string
		)

		expectService := func(serviceName string, matcher types.GomegaMatcher) {
			createdService, err := kubeClient.CoreV1().Services(metav1.NamespaceDefault).Get(context.Background(), serviceName, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			Expect(createdService.Name).To(Equal(serviceName))
			Expect(createdService.Spec.Selector).To(HaveLen(1))
			Expect(createdService.Spec.Selector).To(HaveKeyWithValue(labelKey, labelValue))
			Expect(createdService.Spec).To(matcher)
		}

		BeforeEach(func() {
			vmi = libvmi.New(libvmi.WithLabel(labelKey, labelValue))
			vmi, err := virtClient.KubevirtV1().VirtualMachineInstances(metav1.NamespaceDefault).Create(context.Background(), vmi, metav1.CreateOptions{})
			Expect(err).ToNot(HaveOccurred())

			vm, err = virtClient.KubevirtV1().VirtualMachines(metav1.NamespaceDefault).Create(context.Background(), libvmi.NewVirtualMachine(vmi), metav1.CreateOptions{})
			Expect(err).ToNot(HaveOccurred())

			vmirs = kubecli.NewMinimalVirtualMachineInstanceReplicaSet("vmirs")
			vmirs.Spec.Template = &v1.VirtualMachineInstanceTemplateSpec{}
			vmirs.Spec.Selector = &metav1.LabelSelector{
				MatchLabels: map[string]string{
					labelKey: labelValue,
				},
			}
			vmirs, err = virtClient.KubevirtV1().VirtualMachineInstanceReplicaSets(metav1.NamespaceDefault).Create(context.Background(), vmirs, metav1.CreateOptions{})
			Expect(err).ToNot(HaveOccurred())

			resources = map[string]string{
				"vmi":   vmi.Name,
				"vm":    vm.Name,
				"vmirs": vmirs.Name,
			}
		})

		It("creating a service with default settings", func() {
			for resourceType, resourceName := range resources {
				svn := serviceName + resourceType
				err := runCommand(resourceType, resourceName, "--name", svn, "--port", servicePortStr)
				Expect(err).ToNot(HaveOccurred())
				expectService(svn, gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
					"Ports": ConsistOf(gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
						"Port":     Equal(servicePort),
						"Protocol": Equal(k8sv1.ProtocolTCP),
					})),
					"ClusterIP":      BeEmpty(),
					"Type":           Equal(k8sv1.ServiceTypeClusterIP),
					"IPFamilies":     BeEmpty(),
					"ExternalIPs":    BeEmpty(),
					"IPFamilyPolicy": BeNil(),
				}))
			}
		})

		Context("with missing port but existing pod network ports", func() {
			BeforeEach(func() {
				addPodNetworkWithPorts := func(spec *v1.VirtualMachineInstanceSpec) {
					ports := []v1.Port{{Name: "a", Protocol: "TCP", Port: 80}, {Name: "b", Protocol: "UDP", Port: 81}}
					spec.Networks = append(spec.Networks, v1.Network{Name: "pod", NetworkSource: v1.NetworkSource{Pod: &v1.PodNetwork{}}})
					spec.Domain.Devices.Interfaces = append(spec.Domain.Devices.Interfaces, v1.Interface{Name: "pod", Ports: ports})
				}

				var err error
				addPodNetworkWithPorts(&vmi.Spec)
				vmi, err = virtClient.KubevirtV1().VirtualMachineInstances(metav1.NamespaceDefault).Update(context.Background(), vmi, metav1.UpdateOptions{})
				Expect(err).ToNot(HaveOccurred())
				addPodNetworkWithPorts(&vm.Spec.Template.Spec)
				vm, err = virtClient.KubevirtV1().VirtualMachines(metav1.NamespaceDefault).Update(context.Background(), vm, metav1.UpdateOptions{})
				Expect(err).ToNot(HaveOccurred())
				addPodNetworkWithPorts(&vmirs.Spec.Template.Spec)
				vmirs, err = virtClient.KubevirtV1().VirtualMachineInstanceReplicaSets(metav1.NamespaceDefault).Update(context.Background(), vmirs, metav1.UpdateOptions{})
				Expect(err).ToNot(HaveOccurred())
			})

			It("to create a service", func() {
				for resourceType, resourceName := range resources {
					svn := serviceName + resourceType
					err := runCommand(resourceType, resourceName, "--name", svn)
					Expect(err).ToNot(HaveOccurred())
					expectService(svn, gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
						"Ports": ConsistOf(k8sv1.ServicePort{Name: "port-1", Protocol: "TCP", Port: 80}, k8sv1.ServicePort{Name: "port-2", Protocol: "UDP", Port: 81}),
					}))
				}
			})
		})

		It("creating a service with cluster-ip", func() {
			const clusterIP = "1.2.3.4"
			for resourceType, resourceName := range resources {
				svn := serviceName + resourceType
				err := runCommand(resourceType, resourceName, "--name", svn, "--port", servicePortStr, "--cluster-ip", clusterIP)
				Expect(err).ToNot(HaveOccurred())

				expectService(svn, gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
					"ClusterIP": Equal(clusterIP),
				}))
			}
		})

		It("creating a service with external-ip", func() {
			const externalIP = "1.2.3.4"
			for resourceType, resourceName := range resources {
				svn := serviceName + resourceType
				err := runCommand(resourceType, resourceName, "--name", svn, "--port", servicePortStr, "--external-ip", externalIP)
				Expect(err).ToNot(HaveOccurred())

				expectService(svn, gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
					"ExternalIPs": ConsistOf(externalIP),
				}))
			}
		})

		DescribeTable("creating a service", func(protocol k8sv1.Protocol) {
			for resourceType, resourceName := range resources {
				svn := serviceName + resourceType
				err := runCommand(resourceType, resourceName, "--name", svn, "--port", servicePortStr, "--protocol", string(protocol))
				Expect(err).ToNot(HaveOccurred())

				expectService(svn, gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
					"Ports": ConsistOf(gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
						"Protocol": Equal(protocol),
					})),
				}))
			}
		},
			Entry("with protocol TCP", k8sv1.ProtocolTCP),
			Entry("with protocol UDP", k8sv1.ProtocolUDP),
		)

		DescribeTable("creating a service", func(targetPort string, expected intstr.IntOrString) {
			for resourceType, resourceName := range resources {
				svn := serviceName + resourceType
				err := runCommand(resourceType, resourceName, "--name", svn, "--port", servicePortStr, "--target-port", targetPort)
				Expect(err).ToNot(HaveOccurred())

				expectService(svn, gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
					"Ports": ConsistOf(gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
						"TargetPort": Equal(expected),
					})),
				}))
			}
		},
			Entry("with target-port", "8000", intstr.IntOrString{Type: intstr.Int, IntVal: 8000}),
			Entry("with string target-port", "http", intstr.IntOrString{Type: intstr.String, StrVal: "http"}),
		)

		DescribeTable("creating a service", func(serviceType k8sv1.ServiceType) {
			for resourceType, resourceName := range resources {
				svn := serviceName + resourceType
				err := runCommand(resourceType, resourceName, "--name", svn, "--port", servicePortStr, "--type", string(serviceType))
				Expect(err).ToNot(HaveOccurred())

				expectService(svn, gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
					"Ports": ConsistOf(gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
						"Port": Equal(servicePort),
					})),
				}))
			}
		},
			Entry("with type ClusterIP", k8sv1.ServiceTypeClusterIP),
			Entry("with type NodePort", k8sv1.ServiceTypeNodePort),
			Entry("with type LoadBalancer", k8sv1.ServiceTypeLoadBalancer),
		)

		It("creating a service with named port", func() {
			const portName = "test-port"
			for resourceType, resourceName := range resources {
				svn := serviceName + resourceType
				err := runCommand(resourceType, resourceName, "--name", svn, "--port", servicePortStr, "--port-name", portName)
				Expect(err).ToNot(HaveOccurred())

				expectService(svn, gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
					"Ports": ConsistOf(gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
						"Name": Equal(portName),
					})),
				}))
			}
		})

		DescribeTable("creating a service selecting a suitable default IPFamilyPolicy", func(ipFamily string, ipFamilyPolicy *k8sv1.IPFamilyPolicy, expected ...k8sv1.IPFamily) {
			for resourceType, resourceName := range resources {
				svn := serviceName + resourceType
				err := runCommand(resourceType, resourceName, "--name", svn, "--port", servicePortStr, "--ip-family", ipFamily)
				Expect(err).ToNot(HaveOccurred())

				if ipFamilyPolicy != nil {
					expectService(svn, gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
						"IPFamilies":     Equal(expected),
						"IPFamilyPolicy": gstruct.PointTo(Equal(*ipFamilyPolicy)),
					}))
				} else {
					expectService(svn, gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
						"IPFamilies":     Equal(expected),
						"IPFamilyPolicy": BeNil(),
					}))
				}
			}
		},
			Entry("with IPFamily IPv4", "ipv4", nil, k8sv1.IPv4Protocol),
			Entry("with IPFamily IPv6", "ipv6", nil, k8sv1.IPv6Protocol),
			Entry("with IPFamilies IPv4,IPv6", "ipv4,ipv6", pointer.P(k8sv1.IPFamilyPolicyPreferDualStack), k8sv1.IPv4Protocol, k8sv1.IPv6Protocol),
			Entry("with IPFamilies IPv6,IPv4", "ipv6,ipv4", pointer.P(k8sv1.IPFamilyPolicyPreferDualStack), k8sv1.IPv6Protocol, k8sv1.IPv4Protocol),
		)

		DescribeTable("creating a service", func(ipFamilyPolicy k8sv1.IPFamilyPolicy) {
			for resourceType, resourceName := range resources {
				svn := serviceName + resourceType
				err := runCommand(resourceType, resourceName, "--name", svn, "--port", servicePortStr, "--ip-family-policy", string(ipFamilyPolicy))
				Expect(err).ToNot(HaveOccurred())

				expectService(svn, gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
					"IPFamilyPolicy": gstruct.PointTo(Equal(ipFamilyPolicy)),
				}))
			}
		},
			Entry("with IPFamilyPolicy SingleStack", k8sv1.IPFamilyPolicySingleStack),
			Entry("with IPFamilyPolicy PreferDualStack", k8sv1.IPFamilyPolicyPreferDualStack),
			Entry("with IPFamilyPolicy RequireDualStack", k8sv1.IPFamilyPolicyRequireDualStack),
		)
	})
})

func runCommand(args ...string) error {
	return clientcmd.NewRepeatableVirtctlCommand(append([]string{expose.COMMAND_EXPOSE}, args...)...)()
}
