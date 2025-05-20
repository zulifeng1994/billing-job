package models

import (
	"github.com/pkg/errors"
	"gorm.io/gorm"
)

type UserStorage struct {
	gorm.Model
	UserId              string
	Name                string
	Status              string
	StorageClassID      uint
	Capacity            int64
	ExpandCapacity      int64
	OrganizationGuid    string
	ConsumptionType     string
	StorageChangeRecord []*StorageChangeRecord
}

type StorageChangeRecord struct {
	gorm.Model
	UserStorageID uint
	OldVolume     int64
	NewVolume     int64
	IsPass        int
}

func ListStorage() ([]UserStorage, error) {
	var storage []UserStorage
	if err := DB.Preload("StorageChangeRecord", func(db *gorm.DB) *gorm.DB {
		return db.Order("created_at DESC")
	}).
		Where("consumption_type = ?", "ByTime").
		Find(&storage).Error; err != nil {
		return nil, errors.Wrap(err, "failed to list storage")
	}
	return storage, nil
}
