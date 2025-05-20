package controller

import (
	"billing-job/config"
	"billing-job/kubernetes"
	"billing-job/log"
	"context"
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"strconv"
	"time"
)

type PodInfo struct {
	TotalMemory int                      `json:"total_mem"`
	TotalCpu    int                      `json:"total_cpu"`
	TotalGpu    int                      `json:"gpu_num"`
	GpuType     string                   `json:"gpu_type"`
	Containers  map[string]containerInfo `json:"containers"`
}

type containerInfo struct {
	Memory int `json:"memory"`
	CPU    int `json:"cpu"`
	GPU    int `json:"gpu"`
}

const (
	STATUSRUN  = "run"
	STATUSSTOP = "stop"
)

func BillingMain() {

	clusters := config.GetClusters()
	for clusterID := range clusters {
		apiClient, err := kubernetes.NewClient(clusterID)
		if err != nil {
			log.SugarLogger.Errorf("fail to create kubernetes client: %v", err)
		}
		err = apiClient.C.RESTClient().Get().Do(context.Background()).Error()
		if err != nil {
			log.SugarLogger.Errorf("fail to create kubernetes client: %v", err)
			continue
		}
		go billingRoutine()
		pw := PodWatcher{ClusterID: clusterID}
		go syncRoutine(config.GetNextSyncTime(), clusterID)
		watchForPods(apiClient.C, pw)
	}
}

func GetPodInfo(p *corev1.Pod) *PodInfo {
	info := &PodInfo{}
	containers := make(map[string]containerInfo)
	for _, c := range p.Spec.Containers {
		containerMemory := int(c.Resources.Limits.Memory().Value() / (1024 * 1024))
		containerCpu := int(c.Resources.Limits.Cpu().MilliValue())

		containerGpu := 0
		if gpuQuantity, ok := c.Resources.Limits["nvidia.com/gpu"]; ok {
			containerGpu = int(gpuQuantity.Value())
			info.TotalGpu += containerGpu
		}

		containers[c.Name] = containerInfo{
			Memory: containerMemory,
			CPU:    containerCpu,
			GPU:    containerGpu,
		}
		info.TotalMemory += containerMemory
		info.TotalCpu += containerCpu
	}
	if gpuType, ok := p.Spec.NodeSelector["nvidia.com/gpu.product"]; ok {
		info.GpuType = gpuType
	}
	info.Containers = containers
	return info
}

func GetPodInfoFromStr(str string) (*PodInfo, error) {
	info := &PodInfo{}
	err := json.Unmarshal([]byte(str), info)
	if err != nil {
		return info, errors.Wrap(err, "failed to unmarshal pod info")
	}
	return info, err
}

func (i *PodInfo) String() string {
	bytes, err := json.Marshal(i)
	if err != nil {
		fmt.Println(err)
		return "{}"
	}
	return string(bytes)
}

func getDayPrefix(day time.Time) string {
	return day.Format("2006_01_02_") + getPhasePrefix()
}

func getPhasePrefix() string {
	if config.Billing.BillingInterval >= config.MinutesPerDay {
		// if BillingInterval large than one day, then not use phase
		return ""
	}
	return "phase_" + strconv.Itoa(config.GetPhase()) + "_"
}

func getTimePrefix() string {
	now := time.Now()
	return now.Format("2006_01_02_") + getPhasePrefix()
}
