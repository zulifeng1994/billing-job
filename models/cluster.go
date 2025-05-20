package models

import (
	"billing-job/config"
	"billing-job/log"
)

type Cluster struct {
	ID     uint   `json:"id"`
	GUID   string `json:"guid"`
	Name   string `json:"name"`
	Config string `json:"config"`
}

func (c *Cluster) GetClusters() ([]Cluster, error) {
	var clusters []Cluster
	if err := DB.Find(&clusters).Error; err != nil {
		log.SugarLogger.Infof("Error fetching cluster info: %v", err)
		return nil, err
	}
	return clusters, nil
}

func (c *Cluster) SetClusterConfig() {
	clusters, err := c.GetClusters()
	if err != nil {
		log.SugarLogger.Errorf("Error fetching cluster info: %v", err)
		return
	}
	clusterConfigs := make(map[string]*config.Cluster)
	for _, cluster := range clusters {
		c2 := &config.Cluster{
			ID:     cluster.ID,
			Name:   cluster.Name,
			Config: cluster.Config,
		}
		clusterConfigs[cluster.GUID] = c2
	}
	config.SetClusterInfo(clusterConfigs)
}
