package v1alpha1

import (
	"k8s.io/client-go/rest"
	"kubevirt.io/api/clone/v1alpha1"
	"kubevirt.io/client-go/generated/kubevirt/clientset/versioned/scheme"
)

// NewCloneClientForConfig creates a new CloneV1alpha1Client for the given config without overriding NegotiatedSerializer.
func NewCloneClientForConfig(c *rest.Config) (*CloneV1alpha1Client, error) {
	config := *c
	gv := v1alpha1.SchemeGroupVersion
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
	return &CloneV1alpha1Client{client}, nil
}
