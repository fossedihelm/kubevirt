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

package rest

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"

	"github.com/emicklei/go-restful/v3"

	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"

	virtv1 "kubevirt.io/api/core/v1"
	"kubevirt.io/client-go/log"

	"kubevirt.io/kubevirt/pkg/pointer"
	virtoperatorutils "kubevirt.io/kubevirt/pkg/virt-operator/util"
)

func (app *SubresourceAPIApp) UpdateMachineTypeHandler(request *restful.Request, response *restful.Response) {
	namespace := request.PathParameter("namespace")
	bodyStruct := &virtv1.UpdateMachineTypeRequest{}
	if request.Request.Body != nil {
		err := yaml.NewYAMLOrJSONDecoder(request.Request.Body, 1024).Decode(&bodyStruct)
		switch err {
		case io.EOF, nil:
			break
		default:
			writeError(errors.NewBadRequest(fmt.Sprintf(unmarshalRequestErrFmt, err)), response)
			return
		}
	}
	var kvConfig virtoperatorutils.KubeVirtDeploymentConfig
	kv := app.clusterConfig.GetConfigFromKubeVirtCR()
	if kv == nil {
		writeError(errors.NewInternalError(fmt.Errorf("failed getting KubeVirt config")), response)
		return
	}
	err := json.Unmarshal([]byte(kv.Status.ObservedDeploymentConfig), &kvConfig)
	if err != nil {
		writeError(errors.NewInternalError(fmt.Errorf("%v", err)), response)
		return
	}

	job := generateMachineTypeUpdaterJob(namespace, kvConfig.GetMachineTypeUpdaterImage(), *bodyStruct)
	job, err = app.virtCli.BatchV1().Jobs(namespace).Create(context.Background(), job, metav1.CreateOptions{})
	if err != nil {
		writeError(errors.NewInternalError(fmt.Errorf("error creating machine-type-updater job: %v", err)), response)
		return
	}

	updateMachineTypeInfo := &virtv1.UpdateMachineTypeInfo{
		JobName:      job.Name,
		JobNamespace: job.Namespace,
	}
	err = response.WriteEntity(updateMachineTypeInfo)
	if err != nil {
		log.Log.Reason(err).Error("Failed to write http response.")
	}
}

func generateMachineTypeUpdaterJob(namespace, image string, request virtv1.UpdateMachineTypeRequest) *batchv1.Job {
	const (
		machineTypeJobCmd      = "machine-type-updater"
		machineTypeGlobEnvName = "MACHINE_TYPE_GLOB"
		namespaceEnvName       = "NAMESPACE"
		restartRequiredEnvName = "RESTART_REQUIRED"
		labelSelectorEnvName   = "LABEL_SELECTOR"
	)
	return &batchv1.Job{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "batch/v1",
			Kind:       "Job",
		},

		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "machine-type-updater-",
		},

		Spec: batchv1.JobSpec{
			Template: v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name:  machineTypeJobCmd,
							Image: image,
							Env: []v1.EnvVar{
								{
									Name:  machineTypeGlobEnvName,
									Value: request.MachineTypeGlob,
								},
								{
									Name:  namespaceEnvName,
									Value: namespace,
								},
								{
									Name:  restartRequiredEnvName,
									Value: strconv.FormatBool(request.RestartRequired),
								},
								{
									Name:  labelSelectorEnvName,
									Value: request.LabelSelector,
								},
							},
							SecurityContext: &v1.SecurityContext{
								AllowPrivilegeEscalation: pointer.P(false),
								Capabilities: &v1.Capabilities{
									Drop: []v1.Capability{"ALL"},
								},
								SeccompProfile: &v1.SeccompProfile{
									Type: v1.SeccompProfileTypeRuntimeDefault,
								},
							},
						},
					},
					SecurityContext: &v1.PodSecurityContext{
						RunAsNonRoot: pointer.P(true),
					},
					RestartPolicy: v1.RestartPolicyOnFailure,
				},
			},
		},
	}
}
