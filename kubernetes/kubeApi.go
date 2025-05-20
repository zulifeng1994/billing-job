package kubernetes

import (
	"billing-job/log"
	"context"
	"fmt"

	"billing-job/config"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kclient "k8s.io/client-go/kubernetes"
)

type Kubernetes struct {
	C *kclient.Clientset
}

const (
	resourceVersion = "0"
)

func NewClient(clusterID string) (*Kubernetes, error) {
	if c, ok := config.GetClusters()[clusterID]; ok {
		return newKubeClient(clusterID, c.Config)
	}
	return nil, fmt.Errorf("找不到该集群，名字：%s", clusterID)
}

func newKubeClient(name, kubeconfig string) (*Kubernetes, error) {
	method := "newKubeClient"
	apiClient, err := CreateApiserverClient(name, kubeconfig)
	if err != nil {
		log.SugarLogger.Error(method, "get kubernetes client failed, error:", err)
		return nil, err
	}
	return &Kubernetes{apiClient}, nil
}

func (k8s *Kubernetes) GetPodListAll() (*v1.PodList, error) {
	return k8s.C.CoreV1().Pods("").List(context.Background(), metav1.ListOptions{ResourceVersion: resourceVersion})
}
