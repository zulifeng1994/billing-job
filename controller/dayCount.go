package controller

import (
	"math"
	"strconv"
	"time"

	"billing-job/config"
	"billing-job/log"
	"billing-job/models"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

const (
	// new empty record
	costStatusInitial = "0"

	// cost was saved in redis, wait to be wirte to db
	costStatusBeforeBilling = "1"

	// cost was write to db, this record should be considered read-only.
	costStatusAllDone = "2"
)

func billingRoutine() {
	for {
		now := time.Now()
		dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		next := config.GetNextBillingTime()

		log.SugarLogger.Infof("billing interval is %d", config.Billing.BillingInterval)
		log.SugarLogger.Infof("netx start date is %s", dayStart.Format(time.DateOnly))
		log.SugarLogger.Infof("netx end date is %s", next.Format(time.DateTime))

		duration := time.NewTimer(next.Sub(now))
		<-duration.C

		log.Logger.Info("******************** BEGIN BILLING ACTION ********************")
		podBilling(dayStart, next)
		log.Logger.Info("********************  END BILLING ACTION  ********************")
	}
}

func podBilling(dayStart, lastEnd time.Time) {

	syncCache()
	cluster := config.GetClusters()
	orgIDs := config.GetOrgIDs()
	nodeGroups := config.GetNodeGroups()
	namespaces := config.GetNamespaces()

	prefix := getDayPrefix(dayStart)
	res, err := models.GetKeyNames(prefix + "*")
	if err != nil {
		log.Logger.Error("Error getting keynames", zap.Error(err))
	}
	keys, ok := res.([]string)
	if !ok {
		log.Logger.Error("Error getting keynames", zap.Any("keys", res))
	}

	log.SugarLogger.Infof("keys size is %d", len(keys))

	now := time.Now()
	for _, v := range keys {
		key := v
		log.SugarLogger.Infof("key is %s", key)
		record, err := models.RedisGetByKey(key)
		if err != nil {
			log.SugarLogger.Errorf("Error getting by key: %s, err: %v ", key, zap.Error(err))
			continue
		}

		if _, ok := namespaces[record["Namespace"]]; !ok {
			log.SugarLogger.Infof("namespace %s is system namespace, skip", record["Namespace"])
			continue
		}

		if userID := getUserID(record); userID == "" {
			log.SugarLogger.Infof("userID is null,  skip")
			continue
		}

		if record["CostStatus"] == costStatusAllDone {
			err = models.RedisDel(key)
			if err != nil {
				log.SugarLogger.Errorf("Error deleting by key: %s, err: %v ", key, zap.Error(err))
			}
		}

		podInfo, err := GetPodInfoFromStr(record["PodInfo"])
		if err != nil {
			log.SugarLogger.Errorf("Error getting pod info from str: %s, err: %v ", record["PodInfo"], err)
		}

		podName := record["Name"]

		var cost int64
		var price int64
		if record["Status"] == STATUSRUN {
			log.SugarLogger.Errorf("Pod %s is running, costStatus is %s", podName, record["CostStatus"])
			lastTime, err := time.ParseInLocation(time.DateTime, record["LastActionTime"], now.Location())
			if err != nil {
				log.SugarLogger.Errorf("Error parsing time from %s, err: %v ", record[""], err)
			}
			duration := int(lastEnd.Sub(lastTime).Minutes() + 1)
			log.SugarLogger.Infof("start time is %s, end time is %s and duration is %d",
				lastTime, lastEnd.Format(time.DateTime), duration)

			if duration < 0 {
				duration = 0
			}

			oldDuration, err := strconv.Atoi(record["TotalRunTime"])
			if err != nil {
				log.SugarLogger.Errorf("Error parsing time from %s, err: %v ", record[""], err)
			}

			oldcost, err := strconv.ParseInt(record["Cost"], 10, 64)
			if err != nil {
				log.SugarLogger.Errorf("Error parsing cost from %s, err: %v ", record[""], err)
				oldcost = 0
			}
			price, cost = calculatePodCost(podName, duration+oldDuration, podInfo)

			record["TotalRunTime"] = strconv.Itoa(duration + oldDuration)
			record["LastActionTime"] = lastEnd.Format(time.DateTime)
			record["Cost"] = strconv.FormatInt(cost+oldcost, 10)
			record["CostStatus"] = costStatusBeforeBilling
			ok, err = models.RedisSet(key, record)
			if err != nil {
				log.SugarLogger.Errorf("Error setting cost status to %s, err: %v ", key, err)
			}
			if !ok {
				log.SugarLogger.Warnf("fiail setting cost status to %s ", key)
			}
		} else {
			log.SugarLogger.Infof("pod %s is not running, current status is %s", podName, record["Status"])
			totalTime, err := strconv.Atoi(record["TotalRunTime"])
			if err != nil {
				log.SugarLogger.Errorf("Error parsing time from %s, err: %v ", record[""], err)
			}
			price, cost = calculatePodCost(podName, totalTime, podInfo)
		}
		if cost == 0 {
			continue
		}

		project, err := models.GetProjectByNamespace(record["Namespace"])
		if err != nil {
			log.SugarLogger.Errorf("Error getting project by namespace: %s, err: %v", record["Namespace"], err)
		}

		deducted := 2
		if config.GetConfig().Enable && nodeGroups[record["Taint"]] == models.Shared {
			if err = models.UpdateBalance(project.OrganizationID, "", cost); err != nil {
				log.SugarLogger.Errorf("Error updating balance of project: %s, err: %v", record["Namespace"], err)
			} else {
				deducted = 1
			}
		}

		record["CostStatus"] = costStatusAllDone
		ok, err := models.RedisSet(key, record)
		if err != nil {
			log.SugarLogger.Errorf("Error setting cost status to %s, err: %v ", key, err)
		}
		if !ok {
			log.SugarLogger.Warnf("fiail setting cost status to %s ", key)
		}

		runtime, err := strconv.Atoi(record["TotalRunTime"])
		if err != nil {
			log.SugarLogger.Errorf("Error parsing time from %s, err: %v ", record[""], err)
		}

		if runtime < 1 {
			runtime = 1
		}

		startTime, err := time.ParseInLocation(time.DateTime, record["StartTime"], now.Location())
		if err != nil {
			log.SugarLogger.Errorf("Error parsing time from %s, err: %v ", record[""], err)
		}

		cons := &models.Consumptions{
			OrderID:      uuid.NewString(),
			OrgGUID:      orgIDs[project.OrganizationID],
			UserID:       getUserID(record),
			PodName:      podName,
			PodInfo:      podInfo.String(),
			Instance:     record["App"],
			InstanceType: record["Type"],
			Namespace:    record["Namespace"],
			Type:         1,
			ClusterID:    cluster[record["Cluster"]].ID,
			Price:        price,
			Amount:       cost,
			StartTime:    startTime,
			TotalRuntime: int64(runtime),
			Deducted:     deducted,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}
		err = models.InsertCons(cons)
		if err != nil {
			log.SugarLogger.Errorf("Error inserting consumptions: %s, err: %v", record[""], err)
		}
	}
}

func calculatePodCost(name string, runtime int, podInfo *PodInfo) (int64, int64) {
	cpu := podInfo.TotalCpu
	mem := podInfo.TotalMemory
	gpu := podInfo.TotalGpu
	if runtime < 1 {
		log.SugarLogger.Infof("pod %s runtime is less than 1 , skip this pod ", name)
		return 0, 0
	}

	price := config.GetPrice()

	cpuFloat64 := float64(price.CPU) * float64(cpu) / 1000 * (float64(runtime) / float64(60))
	memFloat64 := float64(price.Memory) * float64(mem) / float64(1024) * (float64(runtime) / float64(60))
	p := int64(float64(price.CPU)*float64(cpu)/1000 + float64(price.Memory)*float64(mem)/float64(1024))
	cost := int64(math.Ceil(cpuFloat64 + memFloat64))

	if gpu > 0 || podInfo.GpuType != "" {
		gpuPrice := price.GPU[podInfo.GpuType]
		gpuFloat64 := float64(gpuPrice) * float64(gpu) * (float64(runtime) / float64(60))
		cost += int64(gpuFloat64)
		p += gpuPrice * int64(gpu)
	}

	if cost < 100 {
		// cost less than 1 fen ,set 100
		return p, 100
	}
	return p, cost
}

func syncCache() {
	conf := models.Config{}
	conf.SetPriceConfig()

	if _, err := models.SetNamespace(); err != nil {
		log.SugarLogger.Errorf("failed to set namespace: %v", err)
	}

	if err := models.SetOrgIds(); err != nil {
		log.SugarLogger.Errorf("failed to set orgids: %v", err)
	}

	if err := models.SetNodeGroups(); err != nil {
		log.SugarLogger.Errorf("failed to set nodegroups: %v", err)
	}

	if err := models.SetNotebooks(); err != nil {
		log.SugarLogger.Errorf("failed to set notebooks: %v", err)
	}

	if err := models.SetTrainJobs(); err != nil {
		log.SugarLogger.Errorf("failed to set trainjobs: %v", err)
	}

	if err := models.SetInference(); err != nil {
		log.SugarLogger.Errorf("failed to set inference: %v", err)
	}
}

func getUserID(record map[string]string) string {
	switch record["Type"] {
	case "Train":
		return config.GetTrainJobs()[record["App"]]
	case "Inference":
		return config.GetInferences()[record["App"]]
	case "Notebook":
		return config.GetNotebooks()[record["App"]]
	default:
		return ""
	}
}
