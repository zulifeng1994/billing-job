package models

import (
	"billing-job/config"
	"billing-job/log"
	"encoding/json"
)

type Config struct {
	Type   string `json:"type"`
	Config string `json:"config"`
}

type BillingConfig struct {
	Enable             bool   `json:"enable"`
	StopPod            bool   `json:"stopPod"`
	CheckAllNamespaces bool   `json:"checkAllNamespaces"`
	SysNamespaces      string `json:"sysNamespaces"`
	Price              Price  `json:"price"`
}

type Price struct {
	CPU     int64           `json:"cpu,omitempty"`
	Memory  int64           `json:"memory,omitempty"`
	GPU     []*GPU          `json:"gpu,omitempty"`
	Storage []*StoragePrice `json:"storage,omitempty"`
}

type GPU struct {
	Type  string `json:"type,omitempty"`
	Price int64  `json:"price,omitempty"`
}

type StoragePrice struct {
	StorageID uint  `json:"storageID"`
	Price     int64 `json:"price"`
}

func (c *Config) GetPriceConfig() (*Config, error) {
	var conf Config
	if err := DB.First(&conf).Where("type = ?", "billing").Error; err != nil {
		log.SugarLogger.Infof("Error fetching cluster info: %v", err)
		return nil, err
	}
	return &conf, nil
}

func (c *Config) SetPriceConfig() {
	conf, err := c.GetPriceConfig()
	if err != nil {
		log.SugarLogger.Errorf("Error fetching cluster info: %v", err)
		return
	}
	var bc BillingConfig
	err = json.Unmarshal([]byte(conf.Config), &bc)
	if err != nil {
		log.SugarLogger.Errorf("Error fetching cluster info: %v", err)
		return
	}
	gpu := make(map[string]int64)
	for _, v := range bc.Price.GPU {
		gpu[v.Type] = v.Price
	}
	storage := make(map[uint]int64)
	for _, v := range bc.Price.Storage {
		storage[v.StorageID] = v.Price
	}
	p := config.Price{
		CPU:     bc.Price.CPU,
		Memory:  bc.Price.Memory,
		GPU:     gpu,
		Storage: storage,
	}
	config.SetPrice(p)

	cc := config.Config{
		Enable: bc.Enable,
	}
	config.SetConfig(cc)
}
