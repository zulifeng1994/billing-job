package controller

import (
	"fmt"
	"github.com/google/uuid"
	"strconv"
	"time"

	"billing-job/config"
	"billing-job/kubernetes"
	"billing-job/log"
	"billing-job/models"

	"go.uber.org/zap"
)

func syncRoutine(initNextSyncTime time.Time, clusterID string) {
	var nextSyncTime = initNextSyncTime
	var getNextSyncTime = func() time.Time {
		currentNextSyncTime := nextSyncTime
		nextSyncTime = nextSyncTime.Add(config.GetSyncInterval())
		return currentNextSyncTime
	}
	for {
		oldPhase := config.GetPhase()
		now := time.Now()
		next := getNextSyncTime()
		log.SugarLogger.Info("come into Every hour refeash sync. next time is", next,
			"cluster id", clusterID, "old phase is", config.GetPhase(), "~~~~~~~~~~~~~~~")
		tduration := time.NewTimer(next.Sub(now))
		<-tduration.C
		// make sure HourSync() be called after phase was increased by GetNextBillingTime()
		for oldPhase == config.GetPhase() {
			log.SugarLogger.Info("Waiting for next phase to start", oldPhase, config.GetPhase())
			time.Sleep(time.Millisecond * 1000)
		}
		log.SugarLogger.Info("New phase is", config.GetPhase())
		go HourSync(clusterID)
		go StorageCount()

	}
}

func HourSync(clusterID string) {
	now := time.Now()
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	client, err := kubernetes.NewClient(clusterID)
	if err != nil {
		log.SugarLogger.Errorf("fail to create kubernetes client: %v", err)
	}
	pods, err := client.GetPodListAll()
	if err != nil {
		log.SugarLogger.Errorf("fail to get pod list: %v", err)
	}

	prefix := getDayPrefix(dayStart)
	res, err := models.GetKeyNames(prefix + "*")
	if err != nil {
		log.Logger.Error("Error getting keynames", zap.Error(err))
	}
	keys, ok := res.([]string)
	if !ok {
		log.Logger.Error("Error getting keynames", zap.Any("keys", res))
	}

	namespaces := config.GetNamespaces()
	podDbSync := make(map[string]map[string]string, 0)

	for _, key := range keys {
		maps, err := models.RedisGetByKey(key)
		if err != nil {
			return
		}

		have := false
		lenapi := len(pods.Items)
		k := 0
		for k < lenapi {
			if _, ok := namespaces[pods.Items[k].Namespace]; !ok || pods.Items[k].Status.Phase != "Running" {
				k++
				continue
			}

			if maps["Name"] == pods.Items[k].ObjectMeta.Name {
				have = true
				pods.Items = append(pods.Items[:k], pods.Items[k+1:]...)
				now := time.Now()
				dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
				dayEnd := dayStart.Add(time.Hour * 24)

				dbLastAction, err := time.ParseInLocation(time.DateTime, maps["last_action_time"], now.Location())
				if err != nil {
					log.SugarLogger.Errorf("fail to parse last action time: %v", err)
				}

				if maps["Status"] == STATUSSTOP {
					duration := now.Sub(dbLastAction).Minutes()
					oldtotal, err := strconv.Atoi(maps["TotalRunTime"])
					if err != nil {
						log.Logger.Error("Error converting oldtotal to int", zap.Error(err))
					}

					maps["Status"] = STATUSRUN
					maps["TotalRunTime"] = strconv.FormatInt(int64(duration)+int64(oldtotal), 10)
					maps["LastActionTime"] = now.Format(time.DateTime)

					ok, err := models.RedisSet(key, maps)
					if err != nil {
						log.SugarLogger.Errorf("fail to set key %s: %v", key, err)
					}
					if !ok {
						log.SugarLogger.Errorf("fail to set key %s", key)
					}
				}

				if dbLastAction.Before(dayStart) {

					maps["LastActionTime"] = dayStart.Format(time.DateTime)
					ok, err = models.RedisSet(key, maps)
					if err != nil {
						log.SugarLogger.Errorf("fail to set key %s: %v", key, err)
					}
					if !ok {
						log.SugarLogger.Errorf("fail to set key %s", key)
					}
					break
				}
				if dbLastAction.After(dayEnd) {
					maps["LastActionTime"] = dayStart.Format(time.DateTime)
					ok, err = models.RedisSet(key, maps)
					if err != nil {
						log.SugarLogger.Errorf("fail to set key %s: %v", key, err)
					}
					if !ok {
						log.SugarLogger.Errorf("fail to set key %s", key)
					}
					break
				}
				lenapi--
				break
			} else {
				k++
			}
		}
		if !have {
			if maps["Status"] == STATUSRUN {
				podDbSync[key] = maps
			}

		}
	}

	for key := range podDbSync {
		mp := make(map[string]string, 0)
		mp["Status"] = STATUSSTOP
		ok, err = models.RedisSet(key, mp)
		if err != nil {
			log.SugarLogger.Errorf("fail to set key %s: %v", key, err)
		}
		if !ok {
			log.SugarLogger.Errorf("fail to set key %s", key)
		}
	}

	for _, p := range pods.Items {
		if _, ok := namespaces[p.Namespace]; !ok || p.Status.Phase != "Running" {
			continue
		}

		key := fmt.Sprintf("%s_%s_%s_%s", getTimePrefix(), clusterID, p.ObjectMeta.Namespace, p.ObjectMeta.Name)
		record := NewRedisRecord(&p, clusterID)
		err = models.SaveToRedis(key, record)
		if err != nil {
			log.SugarLogger.Errorf("fail to set key %s: %v", key, err)
		}
	}
}

func StorageCount() {
	storages, err := models.ListStorage()
	if err != nil {
		log.SugarLogger.Errorf("fail to list storage: %v", err)
	}
	priceConfig := config.GetPrice()
	for _, storage := range storages {
		var capacity int64
		if len(storage.StorageChangeRecord) == 0 {
			continue
		}
		record := storage.StorageChangeRecord[0]
		if len(storage.StorageChangeRecord) == 1 {
			if record.IsPass == 2 || time.Since(record.CreatedAt) < time.Hour {
				continue
			}
			capacity = record.NewVolume
		} else {
			if record.IsPass == 2 || time.Since(record.CreatedAt) < time.Hour {
				capacity = record.OldVolume
			} else {
				capacity = record.NewVolume
			}
		}

		price, ok := priceConfig.Storage[storage.StorageClassID]
		if !ok {
			log.SugarLogger.Errorf("fail to get storage class price%s", storage.StorageClassID)
			continue
		}
		cost := capacity * price
		deducted := 2
		if config.GetConfig().Enable {
			if err = models.UpdateBalance(0, storage.OrganizationGuid, cost); err != nil {
				log.SugarLogger.Errorf("fail to update balance: %v", err)
			} else {
				deducted = 1
			}
		}

		cons := &models.Consumptions{
			OrderID:      uuid.NewString(),
			OrgGUID:      storage.OrganizationGuid,
			UserID:       storage.UserId,
			InstanceType: "Storage",
			Type:         1,
			Price:        cost,
			Amount:       cost,
			StartTime:    time.Now(),
			TotalRuntime: 60,
			Deducted:     deducted,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}
		if err = models.InsertCons(cons); err != nil {
			log.SugarLogger.Errorf("fail to insert cons: %v", err)
		}
	}

}
