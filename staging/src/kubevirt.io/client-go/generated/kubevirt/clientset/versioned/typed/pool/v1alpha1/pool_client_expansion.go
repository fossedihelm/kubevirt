package v1alpha1

import (
	"k8s.io/client-go/rest"
	"kubevirt.io/api/pool/v1alpha1"
	"kubevirt.io/client-go/generated/kubevirt/clientset/versioned/scheme"
)

// NewPoolClientForConfig creates a new PoolV1alpha1Client for the given config without overriding NegotiatedSerializer.
func NewPoolClientForConfig(c *rest.Config) (*PoolV1alpha1Client, error) {
	config := *c
	var gv = v1alpha1.SchemeGroupVersion
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
	return &PoolV1alpha1Client{client}, nil
}
