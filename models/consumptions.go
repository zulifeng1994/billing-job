package models

import (
	"time"
)

type Consumptions struct {
	ID           uint `gorm:"primary_key"`
	OrderID      string
	OrgGUID      string
	UserID       string
	PodName      string
	PodInfo      string
	Instance     string
	InstanceType string
	Namespace    string
	Type         int
	ClusterID    uint
	Price        int64
	Amount       int64
	StartTime    time.Time
	TotalRuntime int64
	Deducted     int //0 为未扣费 1 为已扣费
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func InsertCons(consumptions *Consumptions) error {
	return DB.Create(consumptions).Error
}
