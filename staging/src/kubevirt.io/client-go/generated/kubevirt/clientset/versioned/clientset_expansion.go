package versioned

import (
	"fmt"

	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/flowcontrol"
	clonev1alpha1 "kubevirt.io/client-go/generated/kubevirt/clientset/versioned/typed/clone/v1alpha1"
	kubevirtv1 "kubevirt.io/client-go/generated/kubevirt/clientset/versioned/typed/core/v1"
	exportv1alpha1 "kubevirt.io/client-go/generated/kubevirt/clientset/versioned/typed/export/v1alpha1"
	instancetypev1alpha1 "kubevirt.io/client-go/generated/kubevirt/clientset/versioned/typed/instancetype/v1alpha1"
	instancetypev1alpha2 "kubevirt.io/client-go/generated/kubevirt/clientset/versioned/typed/instancetype/v1alpha2"
	instancetypev1beta1 "kubevirt.io/client-go/generated/kubevirt/clientset/versioned/typed/instancetype/v1beta1"
	migrationsv1alpha1 "kubevirt.io/client-go/generated/kubevirt/clientset/versioned/typed/migrations/v1alpha1"
	poolv1alpha1 "kubevirt.io/client-go/generated/kubevirt/clientset/versioned/typed/pool/v1alpha1"
	snapshotv1alpha1 "kubevirt.io/client-go/generated/kubevirt/clientset/versioned/typed/snapshot/v1alpha1"
)

func NewKubeVirtCliensetForConfig(c *rest.Config) (*Clientset, error) {
	configShallowCopy := *c
	if configShallowCopy.RateLimiter == nil && configShallowCopy.QPS > 0 {
		if configShallowCopy.Burst <= 0 {
			return nil, fmt.Errorf("burst is required to be greater than 0 when RateLimiter is not set and QPS is set to greater than 0")
		}
		configShallowCopy.RateLimiter = flowcontrol.NewTokenBucketRateLimiter(configShallowCopy.QPS, configShallowCopy.Burst)
	}
	var cs Clientset
	var err error
	cs.cloneV1alpha1, err = clonev1alpha1.NewCloneClientForConfig(&configShallowCopy)
	if err != nil {
		return nil, err
	}
	cs.kubevirtV1, err = kubevirtv1.NewKubevirtClientForConfig(&configShallowCopy)
	if err != nil {
		return nil, err
	}
	cs.exportV1alpha1, err = exportv1alpha1.NewExportClientForConfig(&configShallowCopy)
	if err != nil {
		return nil, err
	}
	cs.instancetypeV1alpha1, err = instancetypev1alpha1.NewInstanceTypeClientForConfig(&configShallowCopy)
	if err != nil {
		return nil, err
	}
	cs.instancetypeV1alpha2, err = instancetypev1alpha2.NewInstanceTypeClientForConfig(&configShallowCopy)
	if err != nil {
		return nil, err
	}
	cs.instancetypeV1beta1, err = instancetypev1beta1.NewInstanceTypeClientForConfig(&configShallowCopy)
	if err != nil {
		return nil, err
	}
	cs.migrationsV1alpha1, err = migrationsv1alpha1.NewMigrationsClientForConfig(&configShallowCopy)
	if err != nil {
		return nil, err
	}
	cs.poolV1alpha1, err = poolv1alpha1.NewPoolClientForConfig(&configShallowCopy)
	if err != nil {
		return nil, err
	}
	cs.snapshotV1alpha1, err = snapshotv1alpha1.NewSnapshotClientForConfig(&configShallowCopy)
	if err != nil {
		return nil, err
	}

	cs.DiscoveryClient, err = discovery.NewDiscoveryClientForConfig(&configShallowCopy)
	if err != nil {
		return nil, err
	}
	return &cs, nil
}
