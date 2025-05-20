package controller

import (
	"fmt"
	"strconv"
	"time"

	"billing-job/log"
	"billing-job/models"

	corev1 "k8s.io/api/core/v1"
	kSelector "k8s.io/apimachinery/pkg/fields"
	kclient "k8s.io/client-go/kubernetes"
	kcache "k8s.io/client-go/tools/cache"
	kframework "k8s.io/client-go/tools/cache"
)

const (
	// Resync period for the kube controller loop.
	resyncPeriod = 30 * time.Minute
)

var NeverStop <-chan struct{} = make(chan struct{})

type PodWatcher struct {
	Pod       *corev1.Pod
	ClusterID string
}

func watchForPods(kubeClient *kclient.Clientset, podfunc PodWatcher) {
	_, podController := kframework.NewInformer(
		createPodsLW(kubeClient),
		&corev1.Pod{},
		resyncPeriod,
		kframework.ResourceEventHandlerFuncs{
			AddFunc:    podfunc.HandlePodAdd,
			DeleteFunc: podfunc.HandlePodDelete,
			UpdateFunc: podfunc.HandlePodUpdate,
		},
	)
	go podController.Run(NeverStop)
}

func createPodsLW(kubeClient *kclient.Clientset) *kcache.ListWatch {
	return kcache.NewListWatchFromClient(kubeClient.CoreV1().RESTClient(), "pods", corev1.NamespaceAll, kSelector.Everything())
}

func (pw *PodWatcher) HandlePodAdd(obj interface{}) {

	pod := obj.(*corev1.Pod)
	if pod.Status.Phase != corev1.PodRunning {
		return
	}
	// tensorboard skip
	if pod.Spec.Containers[0].Name == "tensorboard" {
		return
	}
	pw.Pod = pod
	pw.AddNewRecord()
}

func (pw *PodWatcher) HandlePodUpdate(oldObj, newObj interface{}) {
	oldPod := oldObj.(*corev1.Pod)
	newPod := newObj.(*corev1.Pod)

	log.SugarLogger.Infof("Pod Updated: %s/%s, Old Phase: %s, New Phase: %s",
		newPod.Namespace, newPod.Name, oldPod.Status.Phase, newPod.Status.Phase)

	pw.Pod = newPod
	if oldPod.Status.Phase != "Running" && newPod.Status.Phase == "Running" {
		pw.AddNewRecord()
		return
	}

	if oldPod.Status.Phase == "Running" && newPod.Status.Phase != "Running" {
		key := pw.getKey()
		res, err := models.RedisGetByKey(key)
		if err != nil {
			log.SugarLogger.Error("failed to get by key %v", err)
			return
		}
		if res["Status"] != STATUSRUN {
			log.SugarLogger.Error("The status is not right. have got %s", res["Status"])
		} else {
			now := time.Now()
			last, err := time.ParseInLocation(time.DateTime, res["LastActionTime"], now.Location())
			if err != nil {
				log.SugarLogger.Error("failed to parse LastActionTime %v", err)
			}
			duration := now.Sub(last).Minutes()
			res["LastActionTime"] = now.Format(time.DateTime)
			res["TotalRunTime"] = strconv.FormatInt(int64(duration), 10)
			res["Status"] = STATUSSTOP
		}
		podInfo, err := GetPodInfoFromStr(res["PodInfo"])
		if err != nil {
			log.SugarLogger.Error("failed to parse PodInfo %v", err)
		}
		runTime, err := strconv.Atoi(res["TotalRunTime"])
		if err != nil {
			log.SugarLogger.Error("failed to parse TotalRunTime %v", err)
		}
		_, cost := calculatePodCost(res["Name"], runTime, podInfo)
		res["Cost"] = strconv.FormatInt(cost, 10)
		res["CostStatus"] = costStatusBeforeBilling

		ok, err := models.RedisSet(key, res)
		if err != nil {
			log.SugarLogger.Errorf("error to set key %v", err)
		}
		if !ok {
			log.SugarLogger.Error("failed to set key")
		}
	}

}

func (pw *PodWatcher) HandlePodDelete(obj interface{}) {

	pod := obj.(*corev1.Pod)
	pw.Pod = pod
	key := pw.getKey()
	res, err := models.RedisGetByKey(key)
	if err != nil {
		log.SugarLogger.Error("failed to get by key %v", err)
		return
	}
	if len(res) == 0 {
		models.RedisDel(key)
		return
	}
	if res["Status"] != STATUSRUN {
		log.SugarLogger.Error("The status is not right. have got %s", res["Status"])
	} else {
		now := time.Now()
		last, err := time.ParseInLocation(time.DateTime, res["LastActionTime"], now.Location())
		if err != nil {
			log.SugarLogger.Error("failed to parse LastActionTime %v", err)
		}

		duration := now.Sub(last).Minutes()
		res["LastActionTime"] = now.Format(time.DateTime)
		res["TotalRunTime"] = strconv.FormatInt(int64(duration), 10)
		res["Status"] = STATUSSTOP
	}
	podInfo, err := GetPodInfoFromStr(res["PodInfo"])
	if err != nil {
		log.SugarLogger.Error("failed to parse PodInfo %v", err)
	}
	totalTime, err := strconv.Atoi(res["TotalRunTime"])
	if err != nil {
		log.SugarLogger.Error("failed to parse TotalRunTime %v", err)
	}

	_, cost := calculatePodCost(res["Name"], totalTime, podInfo)
	res["Cost"] = strconv.FormatInt(cost, 10)
	res["CostStatus"] = costStatusBeforeBilling
	ok, err := models.RedisSet(key, res)
	if err != nil {
		log.SugarLogger.Error("Error to set %v", err)
	}
	if !ok {
		log.SugarLogger.Error("failed to set %v", err)
	}
}

func (pw *PodWatcher) AddNewRecord() {
	key := pw.getKey()

	record := NewRedisRecord(pw.Pod, pw.ClusterID)
	log.SugarLogger.Infof("add key: %s", key)
	if err := models.SaveToRedis(key, record); err != nil {
		log.SugarLogger.Error("fail to save redis record, err: %v", err)
	}
}

func (pw *PodWatcher) getKey() string {
	return fmt.Sprintf("%s_%s_%s_%s", getTimePrefix(), pw.ClusterID, pw.Pod.ObjectMeta.Namespace, pw.Pod.ObjectMeta.Name)
}

func NewRedisRecord(p *corev1.Pod, cluster string) map[string]string {
	record := map[string]string{
		"Name":           p.Name,
		"Namespace":      p.Namespace,
		"Cluster":        cluster,
		"Type":           "Pod",
		"StartTime":      time.Now().Format(time.DateTime),
		"LastActionTime": time.Now().Format(time.DateTime),
		"TotalRunTime":   "0",
		"Cost":           "0",
		"Status":         STATUSRUN,
		"CostStatus":     costStatusInitial,
		"PodInfo":        GetPodInfo(p).String(),
		"Labels":         p.ObjectMeta.Labels["name"],
		"Taint":          p.Spec.NodeSelector["system/node-group"],
	}
	if name, ok := p.Labels["notebook-name"]; ok {
		record["App"] = name
		record["Type"] = "Notebook"
	} else if tp, ok := p.Labels["lexun.ai/type"]; ok && tp == "inference" {
		record["Type"] = "Inference"
		record["App"] = p.Labels["lexun.ai/inference"]
	} else if name, ok = p.Labels["training.kubeflow.org/job-name"]; ok {
		record["Type"] = "Train"
		record["App"] = name
	}
	return record
}
