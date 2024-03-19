package v1beta1

import (
	"k8s.io/client-go/rest"
	"kubevirt.io/api/instancetype/v1beta1"
	"kubevirt.io/client-go/generated/kubevirt/clientset/versioned/scheme"
)

// NewInstanceTypeClientForConfig creates a new InstancetypeV1beta1Client for the given config without overriding NegotiatedSerializer.
func NewInstanceTypeClientForConfig(c *rest.Config) (*InstancetypeV1beta1Client, error) {
	config := *c
	var gv = v1beta1.SchemeGroupVersion
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
	return &InstancetypeV1beta1Client{client}, nil
}
