package v1alpha2

import (
	"k8s.io/client-go/rest"
	"kubevirt.io/api/instancetype/v1alpha2"
	"kubevirt.io/client-go/generated/kubevirt/clientset/versioned/scheme"
)

// NewInstanceTypeClientForConfig creates a new InstancetypeV2alpha1Client for the given config without overriding NegotiatedSerializer.
func NewInstanceTypeClientForConfig(c *rest.Config) (*InstancetypeV1alpha2Client, error) {
	config := *c
	gv := v1alpha2.SchemeGroupVersion
	config.GroupVersion = &gv
	config.APIPath = "/apis"
	if config.NegotiatedSerializer == nil {
		config.NegotiatedSerializer = scheme.Codecs.WithoutConversion()
	}

	if config.UserAgent == "" {
		config.UserAgent = rest.DefaultKubernetesUserAgent()
	}
	client, err := rest.RESTClientFor(&config)
	if err != nil {
		return nil, err
	}
	return &InstancetypeV1alpha2Client{client}, nil
}
