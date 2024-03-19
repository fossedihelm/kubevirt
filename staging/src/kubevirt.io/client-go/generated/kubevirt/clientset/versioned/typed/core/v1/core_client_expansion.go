package v1

import (
	"k8s.io/client-go/rest"
	v1 "kubevirt.io/api/core/v1"
	"kubevirt.io/client-go/generated/kubevirt/clientset/versioned/scheme"
)

// NewKubevirtClientForConfig creates a new KubevirtV1Client for the given config without overriding NegotiatedSerializer.
func NewKubevirtClientForConfig(c *rest.Config) (*KubevirtV1Client, error) {
	config := *c
	gv := v1.SchemeGroupVersion
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
	return &KubevirtV1Client{client}, nil
}
