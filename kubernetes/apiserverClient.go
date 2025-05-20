package kubernetes

import (
	"fmt"

	client "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

// CreateApiserverClient creates a new Kubernetes Apiserver client using the provided kubeconfig string.
// If the kubeconfig string is empty, it assumes the client is running inside a Kubernetes cluster and attempts to discover the Apiserver.
func CreateApiserverClient(clusterName, kubeconfig string) (*client.Clientset, error) {
	var config *clientcmdapi.Config

	// If kubeconfig is provided, load it
	if kubeconfig != "" {
		clientConfig, err := clientcmd.NewClientConfigFromBytes([]byte(kubeconfig))
		if err != nil {
			return nil, fmt.Errorf("error loading kubeconfig string: %v", err)
		}
		cfg, err := clientConfig.ClientConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to get client config from kubeconfig: %v", err)
		}
		clientset, err := client.NewForConfig(cfg)
		if err != nil {
			return nil, fmt.Errorf("failed to create Kubernetes clientset: %v", err)
		}
		return clientset, nil
	}
	// Default logic if kubeconfig is not provided, use clusterName
	config = clientcmdapi.NewConfig()
	config.CurrentContext = clusterName

	clientBuilder := clientcmd.NewNonInteractiveClientConfig(*config, clusterName, &clientcmd.ConfigOverrides{}, nil)

	cfg, err := clientBuilder.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get client config: %v", err)
	}

	clientset, err := client.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes clientset: %v", err)
	}
	return clientset, nil
}
