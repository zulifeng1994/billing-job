package config

import (
	"billing-job/log"
	"os"

	"k8s.io/apimachinery/pkg/util/yaml"

	"strings"
	"time"
)

const (
	// MinutesPerDay 1440
	MinutesPerDay = 24 * 60
)

type BillingConfig struct {
	DeploymentMode     string `json:"deployment_mode"`
	StopPod            bool   `json:"stop_pod"`
	TestEnv            bool   `json:"test_env"`
	ServiceEmail       string `json:"service_email"`
	BillingInterval    int    `json:"billing_interval"`
	BillingTimeStr     string `json:"billing_time"`
	SyncInterval       int    `json:"sync_interval"`
	CheckAllNamespaces bool   `json:"check_all_namespaces"`
	TestNamespace      string `json:"test_namespace"`
	LogLevel           string `json:"log_level"`
	ModeOnceWriteDB    bool   `json:"modeonce_writedb"`
	SystemNamespace    string `json:"system_namespace"`
}

type rootConfig struct {
	Billing BillingConfig `json:"billing_config"`
	Redis   RedisConfig   `json:"redis_config"`
	DB      DBConfig      `json:"db_config"`
	Consul  ConsulConfig  `json:"consul_config"`
}

type DBConfig struct {
	Type     string `json:"type"`
	Host     string `json:"host"`
	Port     string `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

type RedisConfig struct {
	Server   string `json:"server"`
	Password string `json:"password"`
}

type ConsulConfig struct {
	Address string `json:"address"`
	Host    string `json:"host"`
	Token   string `json:"token"`
}

var rawConfig rootConfig
var Redis RedisConfig
var DB DBConfig
var CC ConsulConfig

var (
	Billing           BillingConfig
	PlatformNamespace map[string]struct{}
)

var phase int
var phasesPerDay int
var billingInterval time.Duration
var nextBillingTime time.Time
var syncInterval time.Duration

var checkTime time.Time
var nextSyncTime time.Time

func init() {

	log.Logger.Info("====================Billing start====================")

	configPath := "config.yaml"
	readConfig(configPath)
	Billing = rawConfig.Billing
	Redis = rawConfig.Redis
	DB = rawConfig.DB
	CC = rawConfig.Consul

	checkBillingConfig(&Billing)

	initSyncAndBillingTime()
	initPhasesPerDay()
	initPhase()

}

func readConfig(configPath string) {

	//f, _ := exec.LookPath(os.Args[0])
	//path, _ := filepath.Abs(f)
	//index := strings.LastIndex(path, string(os.PathSeparator))
	//configFile := path[:index] + "/" + *configPath
	file, err := os.Open(configPath)
	if err != nil {
		panic(err)
	}
	stat, err := file.Stat()
	if err != nil {
		log.SugarLogger.Error(err)
		panic(err)
	}
	decoder := yaml.NewYAMLOrJSONDecoder(file, int(stat.Size()))
	err = decoder.Decode(&rawConfig)
	if err != nil {
		log.SugarLogger.Error("Failed to parse config file \"", configPath, "\", program exit.")
	}
	ValidateConfig()
	log.Logger.Info("Deployment mode: " + rawConfig.Billing.DeploymentMode)

	PlatformNamespace = make(map[string]struct{})
	if rawConfig.Billing.SystemNamespace != "" {
		for _, ns := range strings.Split(rawConfig.Billing.SystemNamespace, ",") {
			PlatformNamespace[ns] = struct{}{}
		}
	}
}

func ValidateConfig() {
	//if db setting is null will get values from env
	/*
	   ENV DB_HOST localhost
	   ENV DB_PORT 3306
	   ENV DB_NAME tenxcloud
	   ENV DB_USER tenxcloud
	   ENV DB_PASSWORD tenxcloud
	   ENV REDIS_SERVER localhost:6379
	   ENV REDIS_PASSWORD billing
	     server: "localhost:6379"
	     password: "billing"
	*/

	if os.Getenv("REDIS_HOST") != "" {
		if os.Getenv("REDIS_PORT") != "" {
			rawConfig.Redis.Server = os.Getenv("REDIS_HOST") + ":" + os.Getenv("REDIS_PORT")
		} else {
			rawConfig.Redis.Server = os.Getenv("REDIS_HOST") + ":6379"
		}
	}
	if v := os.Getenv("REDIS_PWD"); v != "" {
		rawConfig.Redis.Password = v
	}
	if rawConfig.DB.Type == "" {
		rawConfig.DB.Type = "mysql"
	}
	if v := os.Getenv("DB_HOST"); v != "" {
		rawConfig.DB.Host = v
	}
	if v := os.Getenv("DB_PORT"); v != "" {
		rawConfig.DB.Port = v
	}
	if v := os.Getenv("DB_NAME"); v != "" {
		rawConfig.DB.Name = v
	}
	if v := os.Getenv("DB_USER"); v != "" {
		rawConfig.DB.User = v
	}
	if v := os.Getenv("DB_PASSWORD"); v != "" {
		rawConfig.DB.Password = v
	}
}

func initPhase() {
	now := time.Now()
	begin := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	duration := now.Sub(begin)
	billingInterval := time.Minute * time.Duration(Billing.BillingInterval)

	phase = int(duration / billingInterval)
	log.SugarLogger.Infof("phase is %d", phase)
}

func initPhasesPerDay() {
	if Billing.BillingInterval >= MinutesPerDay {
		phasesPerDay = 1
	} else {
		phasesPerDay = MinutesPerDay / Billing.BillingInterval
	}
	log.SugarLogger.Infof("phasesPerDay is: %d", phasesPerDay)
}

func initSyncAndBillingTime() {
	// Initialize nextBillingTime and nextSyncTime.
	// nextBillingTime and nextSyncTime will be automatically increased by
	// GetNextBillingTime() and TimeOneHour::getNextSyncDuration()
	now := time.Now()
	baseTime := time.Date(now.Year(), now.Month(), now.Day(),
		now.Hour(), 0, 0, 0, now.Location()) // Calculate each integral hour

	// Delay sync time by 15 seconds make sure phase was increased,
	// force sync function (HourSync) to create new redis key.
	syncDelay := time.Second * 15

	if checkTime.IsZero() {
		// If billing_time not specified,
		// set nextBillingTime and nextSyncTime to current time,
		// so billing function (billingRoutine) will be called now + billing interval,
		// and sync function (syncRoutine) will be called now + sync interval + 15s
		nextBillingTime = baseTime.Add(billingInterval)
		nextSyncTime = baseTime.Add(syncInterval)

	} else {
		nextBillingTime = getNextTimeByInterval(baseTime, checkTime, billingInterval)
		nextSyncTime = getNextTimeByInterval(baseTime, checkTime, syncInterval)
	}

	// Add nextSyncTime by 15 seconds (syncDelay),
	// to make sure sync function be called after phase was
	// increased by GetNextBillingTime()
	nextSyncTime = nextSyncTime.Add(syncDelay)
}

func getNextTimeByInterval(current, target time.Time, interval time.Duration) time.Time {
	// The next time must be ahead of current time by at least 2 minutes.
	// If current time is 15:05 and target time is 15:06,
	// the next time to will at least be 15:07,
	// so it shall be current time + interval.
	minTarget := current.Add(2 * time.Minute)

	// the format of target is like "10:05"
	// make it like "2016-05-09 10:05:00"
	nextTarget := time.Date(minTarget.Year(), minTarget.Month(), minTarget.Day(),
		target.Hour(), target.Minute(), 0, 0, minTarget.Location())
	if nextTarget.Before(minTarget) {
		for nextTarget.Before(minTarget) {
			nextTarget = nextTarget.Add(interval)
		}
	} else if nextTarget.After(minTarget) {
		for nextTarget.After(minTarget) {
			nextTarget = nextTarget.Add(-interval)
		}
		nextTarget = nextTarget.Add(interval)
	}
	return nextTarget
}

func GetPhase() int {
	return phase
}

func GetNextBillingTime() time.Time {

	phase = getNextPhase(phase, phasesPerDay)
	log.SugarLogger.Infof("current phase is %d", phase)

	currentNextBillingTime := nextBillingTime
	nextBillingTime = nextBillingTime.Add(billingInterval)
	return currentNextBillingTime
}

func getNextPhase(phase, total int) (p int) {
	p = phase % total
	p++
	return p
}

func checkBillingConfig(billingConfig *BillingConfig) {
	method := "checkBillingConfig"
	var err error

	// print stop_pod value
	//if billingConfig.StopPod == true {
	//	log.SugarLogger.Warn(method, "\"stop_pod\" is true, user's pod will be stoped!!!")
	//} else {
	//	log.SugarLogger.Info(method, "\"stop_pod\" is not true, user's pod will not be stopped.")
	//}
	//
	// print check_all_namespaces value and test_namespace
	// if check_all_namespaces is true, then test_namespace must be set.
	//if billingConfig.CheckAllNamespaces == true {
	//	log.SugarLogger.Info(method, "\"check_all_namespaces\" is true!!!")
	//} else {
	//	log.SugarLogger.Info(method, "\"check_all_namespaces\" is not true.")
	//	if billingConfig.TestNamespace == "" {
	//		log.SugarLogger.Error(method, "Please set test_namespace when \"check_all_namespaces\" is not true.")
	//		os.Exit(1)
	//	}
	//	log.SugarLogger.Info(method, "\"test_namespace\" is", billingConfig.TestNamespace)
	//}

	if billingConfig.BillingTimeStr == "" {
		// 如果billing time未指定
		// 那么sync interval默认为30分钟，billing interval默认为240分钟
		if billingConfig.SyncInterval <= 0 {
			log.SugarLogger.Info(method, "Set sync interval to 30 minutes.")
			billingConfig.SyncInterval = 30
		}
		log.SugarLogger.Info(method, "Sync interval is", billingConfig.SyncInterval)

		if billingConfig.BillingInterval <= 0 {
			log.SugarLogger.Info(method, "Set billing interval to 240 minutes.")
			billingConfig.BillingInterval = 240
		}
		log.SugarLogger.Info(method, "Billing interval is", billingConfig.BillingInterval, ".")
	} else {
		// 如果billing time有值
		// 那么sync interval默认为60分钟，billing interval默认为1天（1440分钟）
		// 如果用户自行指定了billing interval，那么它必须得是60的n倍
		checkTime, err = time.Parse("15:4", billingConfig.BillingTimeStr)
		if err != nil {
			log.SugarLogger.Error(method, "Failed to parse billing_time", billingConfig.BillingTimeStr, ", format must be \"hour:minute\"!")
			log.SugarLogger.Error(method, "Eg \"0:1\" \"15:30\"")
			os.Exit(1)
		}
		log.SugarLogger.Info(method, "Billing time is", billingConfig.BillingTimeStr)

		if billingConfig.SyncInterval <= 0 {
			log.SugarLogger.Info(method, "Set sync interval to 30 minutes.")
			billingConfig.SyncInterval = 30
		}
		log.SugarLogger.Info(method, "Sync interval is", billingConfig.SyncInterval)

		if billingConfig.BillingInterval <= 0 {
			log.SugarLogger.Info(method, "Set billing interval to 240 minutes.")
			billingConfig.BillingInterval = 240
		}
		log.SugarLogger.Info(method, "Billing interval is", billingConfig.BillingInterval, ".")

		if billingConfig.BillingInterval%60 != 0 {
			log.SugarLogger.Error(method, "Billing interval is not n times of 60.")
			os.Exit(1)
		}
	}

	// 如果用户自己指定了billing interval
	// 如果billing interval大于1天，那么它必须得是1440的n倍，即n天进行一次计费
	// 如果billing interval小于1天，那么1440必须要能把它整除，即一天只内要能进行n次计费
	if (billingConfig.BillingInterval > MinutesPerDay) && (billingConfig.BillingInterval%MinutesPerDay != 0) {
		log.SugarLogger.Error(method, "Billing interval is not n times of 1440")
		os.Exit(1)
	} else if (billingConfig.BillingInterval < MinutesPerDay) && (MinutesPerDay%billingConfig.BillingInterval != 0) {
		log.SugarLogger.Error(method, "1440 is not n times of billing interval")
		os.Exit(1)
	}

	// billing interval must be n times of sync interval
	if billingConfig.BillingInterval%billingConfig.SyncInterval != 0 {
		log.SugarLogger.Error(method, "Billing interval is not n times of sync interval", billingConfig.SyncInterval, ".")
		os.Exit(1)
	}

	billingInterval = time.Minute * time.Duration(Billing.BillingInterval)
	syncInterval = time.Minute * time.Duration(Billing.SyncInterval)

	// default service_email is "cloud-dream@tenxcloud.com"
	const defaultServiceEmail = "cloud-dream@tenxcloud.com"
	if billingConfig.ServiceEmail == "" {
		log.SugarLogger.Info(method, "Set service email to ", defaultServiceEmail)
		billingConfig.ServiceEmail = defaultServiceEmail
	}
	log.SugarLogger.Info(method, "Service email is", billingConfig.ServiceEmail)
}

func GetSyncInterval() time.Duration {
	return syncInterval
}

func GetNextSyncTime() time.Time {
	// currentNextSyncTime := nextSyncTime
	// nextSyncTime = nextSyncTime.Add(syncInterval)
	return nextSyncTime
}
