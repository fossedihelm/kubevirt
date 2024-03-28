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

package machinetype

import (
	"context"
	"fmt"
	"path"

	"github.com/spf13/cobra"

	"k8s.io/client-go/tools/clientcmd"

	virtv1 "kubevirt.io/api/core/v1"
	"kubevirt.io/client-go/kubecli"

	"kubevirt.io/kubevirt/pkg/virtctl/templates"
)

const (
	machineTypeCmd    = "machine-types"
	restartNowFlag    = "restart-now"
	labelSelectorFlag = "label-selector"
)

type Command struct {
	clientConfig    clientcmd.ClientConfig
	namespace       string
	restartNow      bool
	labelSelector   string
	machineTypeGlob string
	image           string
}

// NewCommand generates a new "update machine-types" command
func NewCommand(clientConfig clientcmd.ClientConfig) *cobra.Command {
	c := Command{clientConfig: clientConfig}
	cmd := &cobra.Command{
		Use:   machineTypeCmd,
		Short: "Perform a mass machine type transition on any VMs in the given namespace with a machine type matching the specified glob.",
		Long: `Create and deploy a Job that iterates through VMs, updating the machine type of any VMs that match the machine type glob specified by argument to the latest machine type.
If --restart-now is set to true, the running VMs will be automatically restarted.
//If no namespace is specified via --target-namespace, the mass machine type transition will be applied across all namespaces.
The --label-selector flag can be used to further limit which VMs the machine type update will be applied to.
Note that should the Job fail, it will be restarted. Additionally, once the Job is terminated, it will not be automatically deleted.
The Job can be monitored and then deleted manually after it has been terminated using 'kubectl' commands.`,
		Example: usage(),
		Args:    templates.ExactArgs(machineTypeCmd, 1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return c.run(args)
		},
	}

	// flags for the "update machine-types" command
	//TODO remove leftover
	// cmd.Flags().StringVar(&c.targetNamespace, targetNamespaceFlag, "", "Namespace in which the mass machine type transition will be applied. Defaults to all namespaces.")
	cmd.Flags().BoolVar(&c.restartNow, restartNowFlag, false, "When true, immediately restarts all VMs that have their machine types updated. Otherwise, updated VMs must be restarted manually for the machine type change to take effect. Defaults to false.")
	cmd.Flags().StringVar(&c.labelSelector, labelSelectorFlag, "", "Selector (label query) on which to filter VMs to be updated.")
	cmd.SetUsageTemplate(templates.UsageTemplate())

	return cmd
}

func usage() string {
	//return `  # Update the machine types of all VMs with the designated machine type across all namespaces without automatically restarting running VMs:
	//{{ProgramName}} update machine-types *q35-2.*
	//# Update the machine types of all VMs with the designated machine type in the namespace 'default':
	//{{ProgramName}} update machine-types *q35-2.* --target-namespace=default
	//# Update the machine types of all VMs with the designated machine type and automatically restart them if they are running:
	//{{ProgramName}} update machine-types *q35-2.* --restart-now=true
	//
	//# Update the machine types of all VMs with the designated machine type and with the label 'kubevirt.io/memory=large':
	//{{ProgramName}} update machine-types *q35-2.* --label-selector=kubevirt.io/memory=large`
	return `  # Update the machine types of all VMs  in the namespace 'default' with the designated machine type without automatically restarting running VMs:
  {{ProgramName}} --namespace default update machine-types *q35-2.*
  # Update the machine types of all VMs in the namespace 'default' with the designated machine type and automatically restart them if they are running:
  {{ProgramName}} --namespace default update machine-types *q35-2.* --restart-now=true

  # Update the machine types of all VMs in the namespace 'default' with the designated machine type and with the label 'kubevirt.io/memory=large':
  {{ProgramName}} --namespace default update machine-types *q35-2.* --label-selector=kubevirt.io/memory=large`
}

func (o *Command) run(args []string) error {
	o.machineTypeGlob = args[0]
	// Execute a match with empty string to check if the pattern is correct
	_, err := path.Match(o.machineTypeGlob, "")
	if err != nil {
		return fmt.Errorf("syntax error in pattern value %s: %v", o.machineTypeGlob, err)
	}
	namespace, _, err := o.clientConfig.Namespace()
	if err != nil {
		return err
	}
	o.namespace = namespace
	virtClient, err := kubecli.GetKubevirtClientFromClientConfig(o.clientConfig)
	if err != nil {
		return fmt.Errorf("cannot obtain KubeVirt client: %v", err)
	}

	job, err := virtClient.VirtualMachine(namespace).UpdateMachineType(context.Background(), &virtv1.UpdateMachineTypeRequest{MachineTypeGlob: o.machineTypeGlob, RestartRequired: o.restartNow, LabelSelector: o.labelSelector})
	if err != nil {
		return fmt.Errorf("error creating machine-type-updater job: %v", err)
	}
	fmt.Printf(`
	Successfully created job %s.
	This job can be monitored using 'kubectl get job %s -n %s' and 'kubectl describe job %s -n %s'.
	Once terminated, this job can be deleted by using 'kubectl delete job %s -n %s'.
	`, job.JobName, job.JobName, job.JobNamespace, job.JobName, job.JobNamespace, job.JobName, job.JobNamespace)
	return nil
}
