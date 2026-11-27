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

package rest

import (
	"fmt"
	"net/http"

	"github.com/emicklei/go-restful/v3"

	"k8s.io/client-go/tools/cache"

	v1 "kubevirt.io/api/core/v1"
)

// getVMI resolves the VMI a request names. The store is cluster-wide, so the
// lookup is also what keeps a handler to its own node: a VMI that runs
// elsewhere is not this node's to serve, exactly as it was absent from the
// node-scoped store this replaces.
func getVMI(request *restful.Request, vmiStore cache.Store, nodeName string) (*v1.VirtualMachineInstance, int, error) {
	key := fmt.Sprintf("%s/%s", request.PathParameter("namespace"), request.PathParameter("name"))
	vmiObj, vmiExists, err := vmiStore.GetByKey(key)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}
	if !vmiExists {
		return nil, http.StatusNotFound, fmt.Errorf("VMI %s does not exist", key)
	}
	vmi := vmiObj.(*v1.VirtualMachineInstance)
	if vmi.Status.NodeName != nodeName {
		return nil, http.StatusNotFound, fmt.Errorf("VMI %s does not exist on node %s", key, nodeName)
	}
	return vmi, 0, nil
}
