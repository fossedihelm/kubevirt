package expose

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	k8sv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/tools/clientcmd"

	v1 "kubevirt.io/api/core/v1"
	"kubevirt.io/client-go/kubecli"

	"kubevirt.io/kubevirt/pkg/virtctl/templates"
)

const (
	COMMAND_EXPOSE = "expose"
)

type expose struct {
	serviceName       string
	clusterIP         string
	externalIP        string
	loadBalancerIP    string
	port              int32
	strProtocol       string
	strTargetPort     string
	strServiceType    string
	portName          string
	strIPFamily       string
	strIPFamilyPolicy string

	clientConfig clientcmd.ClientConfig

	targetPort     intstr.IntOrString
	protocol       k8sv1.Protocol
	serviceType    k8sv1.ServiceType
	ipFamilies     []k8sv1.IPFamily
	ipFamilyPolicy k8sv1.IPFamilyPolicy

	namespace string
	client    kubecli.KubevirtClient
}

func NewCommand(clientConfig clientcmd.ClientConfig) *cobra.Command {
	e := expose{clientConfig: clientConfig}
	cmd := &cobra.Command{
		Use:   "expose (TYPE NAME)",
		Short: "Expose a virtual machine instance, virtual machine, or virtual machine instance replica set as a new service.",
		Long: `Looks up a virtual machine instance, virtual machine or virtual machine instance replica set by name and use its selector as the selector for a new service on the specified port.
A virtual machine instance replica set will be exposed as a service only if its selector is convertible to a selector that service supports, i.e. when the selector contains only the matchLabels component.
Note that if no port is specified via --port and the exposed resource has multiple ports, all will be re-used by the new service.
Also if no labels are specified, the new service will re-use the labels from the resource it exposes.

Possible types are (case insensitive, both single and plurant forms):

virtualmachineinstance (vmi), virtualmachine (vm), virtualmachineinstancereplicaset (vmirs)`,
		Example: usage(),
		Args:    cobra.ExactArgs(2),
		RunE:    e.run,
	}

	cmd.Flags().StringVar(&e.serviceName, "name", "", "Name of the service created for the exposure of the VM.")
	if err := cmd.MarkFlagRequired("name"); err != nil {
		panic(err)
	}
	cmd.Flags().StringVar(&e.clusterIP, "cluster-ip", "", "ClusterIP to be assigned to the service. Leave empty to auto-allocate, or set to 'None' to create a headless service.")
	cmd.Flags().StringVar(&e.externalIP, "external-ip", "", "Additional external IP address (not managed by the cluster) to accept for the service. If this IP is routed to a node, the service can be accessed by this IP in addition to its generated service IP. Optional.")
	cmd.Flags().Int32Var(&e.port, "port", 0, "The port that the service should serve on.")
	cmd.Flags().StringVar(&e.strProtocol, "protocol", "TCP", "The network protocol for the service to be created.")
	cmd.Flags().StringVar(&e.strTargetPort, "target-port", "", "Name or number for the port on the VM that the service should direct traffic to. Optional.")
	cmd.Flags().StringVar(&e.strServiceType, "type", "ClusterIP", "Type for this service: ClusterIP, NodePort, or LoadBalancer.")
	cmd.Flags().StringVar(&e.portName, "port-name", "", "Name of the port. Optional.")
	cmd.Flags().StringVar(&e.strIPFamily, "ip-family", "", "IP family over which the service will be exposed. Valid values are 'IPv4', 'IPv6', 'IPv4,IPv6' or 'IPv6,IPv4'")
	cmd.Flags().StringVar(&e.strIPFamilyPolicy, "ip-family-policy", "", "IP family policy defines whether the service can use IPv4, IPv6, or both. Valid values are 'SingleStack', 'PreferDualStack' or 'RequireDualStack'")

	cmd.SetUsageTemplate(templates.UsageTemplate())

	return cmd
}

func usage() string {
	return `  # Expose SSH to a virtual machine instance called 'myvm' on each node via a NodePort service:
  {{ProgramName}} expose vmi myvm --port=22 --name=myvm-ssh --type=NodePort

  # Expose all defined pod-network ports of a virtual machine instance replicaset on a service:
  {{ProgramName}} expose vmirs myvmirs --name=vmirs-service

  # Expose port 8080 as port 80 from a virtual machine instance replicaset on a service:
  {{ProgramName}} expose vmirs myvmirs --port=80 --target-port=8080 --name=vmirs-service`
}

func (e *expose) run(cmd *cobra.Command, args []string) error {
	// first argument is type of VM: VMI, VM or VMIRS
	vmType := strings.ToLower(args[0])
	// second argument must be name of the VM
	vmName := args[1]

	if err := e.parseFlags(); err != nil {
		return err
	}

	var err error
	if e.namespace, _, err = e.clientConfig.Namespace(); err != nil {
		return err
	}
	if e.client, err = kubecli.GetKubevirtClientFromClientConfig(e.clientConfig); err != nil {
		return fmt.Errorf("cannot obtain KubeVirt client: %v", err)
	}

	serviceSelector, ports, err := e.getServiceSelectorAndPorts(vmType, vmName)
	if err != nil {
		return err
	}

	if err := e.createService(serviceSelector, ports); err != nil {
		return err
	}

	cmd.Printf("Service %s successfully created for %s %s\n", e.serviceName, vmType, vmName)
	return nil
}

func (e *expose) parseFlags() error {
	e.targetPort = intstr.Parse(e.strTargetPort)

	var err error
	if e.protocol, err = convertProtocol(e.strProtocol); err != nil {
		return err
	}
	if e.serviceType, err = convertServiceType(e.strServiceType); err != nil {
		return err
	}
	if e.ipFamilies, err = convertIPFamily(e.strIPFamily); err != nil {
		return err
	}
	if e.ipFamilyPolicy, err = convertIPFamilyPolicy(e.strIPFamilyPolicy, e.ipFamilies); err != nil {
		return err
	}

	return nil
}

func (e *expose) getServiceSelectorAndPorts(vmType, vmName string) (serviceSelector map[string]string, ports []k8sv1.ServicePort, err error) {
	switch vmType {
	case "vmi", "vmis", "virtualmachineinstance", "virtualmachineinstances":
		vmi, err := e.client.VirtualMachineInstance(e.namespace).Get(context.Background(), vmName, metav1.GetOptions{})
		if err != nil {
			return nil, nil, fmt.Errorf("error fetching VirtualMachineInstance: %v", err)
		}
		ports = podNetworkPorts(&vmi.Spec)
		serviceSelector = vmi.ObjectMeta.Labels
		delete(serviceSelector, v1.NodeNameLabel)
		delete(serviceSelector, v1.VirtualMachinePoolRevisionName)
		delete(serviceSelector, v1.MigrationTargetNodeNameLabel)
	case "vm", "vms", "virtualmachine", "virtualmachines":
		vm, err := e.client.VirtualMachine(e.namespace).Get(context.Background(), vmName, metav1.GetOptions{})
		if err != nil {
			return nil, nil, fmt.Errorf("error fetching VirtualMachine: %v", err)
		}
		if vm.Spec.Template != nil {
			ports = podNetworkPorts(&vm.Spec.Template.Spec)
		}
		serviceSelector = vm.Spec.Template.ObjectMeta.Labels
		delete(serviceSelector, v1.VirtualMachinePoolRevisionName)
	case "vmirs", "vmirss", "virtualmachineinstancereplicaset", "virtualmachineinstancereplicasets":
		vmirs, err := e.client.ReplicaSet(e.namespace).Get(context.Background(), vmName, metav1.GetOptions{})
		if err != nil {
			return nil, nil, fmt.Errorf("error fetching VirtualMachineInstanceReplicaSet: %v", err)
		}
		if vmirs.Spec.Template != nil {
			ports = podNetworkPorts(&vmirs.Spec.Template.Spec)
		}
		if vmirs.Spec.Selector != nil {
			if len(vmirs.Spec.Selector.MatchExpressions) > 0 {
				return nil, nil, fmt.Errorf("cannot expose VirtualMachineInstanceReplicaSet with match expressions")
			}
			serviceSelector = vmirs.Spec.Selector.MatchLabels
		}
	default:
		return nil, nil, fmt.Errorf("unsupported resource type: %s", vmType)
	}

	if e.port != 0 {
		ports = []k8sv1.ServicePort{{Name: e.portName, Protocol: e.protocol, Port: e.port, TargetPort: e.targetPort}}
	}

	if len(serviceSelector) == 0 {
		return nil, nil, fmt.Errorf("cannot expose %s without any label: %s", vmType, vmName)
	}
	if len(ports) == 0 {
		return nil, nil, fmt.Errorf("couldn't find port via --port flag or introspection")
	}

	return serviceSelector, ports, nil
}

func (e *expose) createService(serviceSelector map[string]string, ports []k8sv1.ServicePort) error {
	service := &k8sv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      e.serviceName,
			Namespace: e.namespace,
		},
		Spec: k8sv1.ServiceSpec{
			Ports:      ports,
			Selector:   serviceSelector,
			ClusterIP:  e.clusterIP,
			Type:       e.serviceType,
			IPFamilies: e.ipFamilies,
		},
	}
	if len(e.externalIP) > 0 {
		service.Spec.ExternalIPs = []string{e.externalIP}
	}
	if e.ipFamilyPolicy != "" {
		service.Spec.IPFamilyPolicy = &e.ipFamilyPolicy
	}
	if _, err := e.client.CoreV1().Services(e.namespace).Create(context.Background(), service, metav1.CreateOptions{}); err != nil {
		return fmt.Errorf("service creation failed: %v", err)
	}

	return nil
}

func convertProtocol(strProtocol string) (k8sv1.Protocol, error) {
	switch strings.ToLower(strProtocol) {
	case strings.ToLower(string(k8sv1.ProtocolTCP)):
		return k8sv1.ProtocolTCP, nil
	case strings.ToLower(string(k8sv1.ProtocolUDP)):
		return k8sv1.ProtocolUDP, nil
	default:
		return "", fmt.Errorf("unknown protocol: %s", strProtocol)
	}
}

func convertServiceType(strServiceType string) (k8sv1.ServiceType, error) {
	switch strings.ToLower(strServiceType) {
	case strings.ToLower(string(k8sv1.ServiceTypeClusterIP)):
		return k8sv1.ServiceTypeClusterIP, nil
	case strings.ToLower(string(k8sv1.ServiceTypeNodePort)):
		return k8sv1.ServiceTypeNodePort, nil
	case strings.ToLower(string(k8sv1.ServiceTypeLoadBalancer)):
		return k8sv1.ServiceTypeLoadBalancer, nil
	case strings.ToLower(string(k8sv1.ServiceTypeExternalName)):
		return "", fmt.Errorf("type: %s not supported", strServiceType)
	default:
		return "", fmt.Errorf("unknown service type: %s", strServiceType)
	}
}

func convertIPFamily(strIPFamily string) ([]k8sv1.IPFamily, error) {
	switch strings.ToLower(strIPFamily) {
	case "":
		return []k8sv1.IPFamily{}, nil
	case strings.ToLower(string(k8sv1.IPv4Protocol)):
		return []k8sv1.IPFamily{k8sv1.IPv4Protocol}, nil
	case strings.ToLower(string(k8sv1.IPv6Protocol)):
		return []k8sv1.IPFamily{k8sv1.IPv6Protocol}, nil
	case strings.ToLower(string(k8sv1.IPv4Protocol) + "," + string(k8sv1.IPv6Protocol)):
		return []k8sv1.IPFamily{k8sv1.IPv4Protocol, k8sv1.IPv6Protocol}, nil
	case strings.ToLower(string(k8sv1.IPv6Protocol) + "," + string(k8sv1.IPv4Protocol)):
		return []k8sv1.IPFamily{k8sv1.IPv6Protocol, k8sv1.IPv4Protocol}, nil
	default:
		return nil, fmt.Errorf("unknown IPFamily/s: %s", strIPFamily)
	}
}

func convertIPFamilyPolicy(strIPFamilyPolicy string, ipFamilies []k8sv1.IPFamily) (k8sv1.IPFamilyPolicy, error) {
	switch strings.ToLower(strIPFamilyPolicy) {
	case "":
		if len(ipFamilies) > 1 {
			return k8sv1.IPFamilyPolicyPreferDualStack, nil
		}
		return "", nil
	case strings.ToLower(string(k8sv1.IPFamilyPolicySingleStack)):
		return k8sv1.IPFamilyPolicySingleStack, nil
	case strings.ToLower(string(k8sv1.IPFamilyPolicyPreferDualStack)):
		return k8sv1.IPFamilyPolicyPreferDualStack, nil
	case strings.ToLower(string(k8sv1.IPFamilyPolicyRequireDualStack)):
		return k8sv1.IPFamilyPolicyRequireDualStack, nil
	default:
		return "", fmt.Errorf("unknown IPFamilyPolicy/s: %s", strIPFamilyPolicy)
	}
}

func podNetworkPorts(vmiSpec *v1.VirtualMachineInstanceSpec) []k8sv1.ServicePort {
	podNetworkName := ""
	for _, network := range vmiSpec.Networks {
		if network.Pod != nil {
			podNetworkName = network.Name
			break
		}
	}
	if podNetworkName != "" {
		for _, device := range vmiSpec.Domain.Devices.Interfaces {
			if device.Name == podNetworkName {
				ports := []k8sv1.ServicePort{}
				for i, port := range device.Ports {
					ports = append(ports, k8sv1.ServicePort{Name: fmt.Sprintf("port-%d", i+1), Protocol: k8sv1.Protocol(port.Protocol), Port: port.Port})
				}
				return ports
			}
		}
	}
	return nil
}
