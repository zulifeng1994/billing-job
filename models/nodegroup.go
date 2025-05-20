package models

import (
	"billing-job/config"
	"github.com/pkg/errors"
	"time"
)

type NodeGroup struct {
	ID        uint64 `gorm:"primary_key;auto_increment"`
	Name      string
	ClusterID string
	Taint     string
	Type      string
}

const (
	Shared    = "shared"
	exclusive = "exclusive"
)

func ListNodeGroups() ([]*NodeGroup, error) {
	var nodeGroups []*NodeGroup
	twoHoursAgo := time.Now().Add(-2 * time.Hour)
	if err := DB.Where("deleted_at IS NULL OR deleted_at >= ?", twoHoursAgo).Find(&nodeGroups).Error; err != nil {
		return nil, errors.Wrap(err, "failed to list node groups")
	}
	return nodeGroups, nil
}

func SetNodeGroups() error {
	nodeGroups, err := ListNodeGroups()
	if err != nil {
		return errors.Wrap(err, "failed to list node groups")
	}
	ngs := make(map[string]string, len(nodeGroups))
	for _, ng := range nodeGroups {
		ngs[ng.Taint] = ng.Type
	}
	config.SetNodeGroups(ngs)
	return nil
}
