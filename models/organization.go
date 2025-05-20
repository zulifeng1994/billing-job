package models

import (
	"billing-job/config"
	"github.com/pkg/errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Organization struct {
	ID      uint `gorm:"primary_key"`
	GUID    string
	Name    string
	Balance int64
}

func UpdateBalance(id uint, guid string, cost int64) error {
	tx := DB.Begin()
	if tx.Error != nil {
		return tx.Error
	}

	var org Organization
	if id > 0 {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", id).
			First(&org).Error; err != nil {
			tx.Rollback()
			return err
		}
	} else if guid != "" {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("guid = ?", guid).
			First(&org).Error; err != nil {
			tx.Rollback()
			return err
		}
	}

	if err := tx.Model(&org).Update("balance", gorm.Expr("balance - ?", cost)).Error; err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit().Error
}

func ListAllOrg() ([]Organization, error) {
	var orgs []Organization
	if err := DB.Find(&orgs).Error; err != nil {
		return nil, err
	}
	return orgs, nil
}

func SetOrgIds() error {
	orgs, err := ListAllOrg()
	if err != nil {
		return errors.Wrap(err, "Error listing organizations")
	}
	mp := make(map[uint]string)
	for _, org := range orgs {
		mp[org.ID] = org.GUID
	}
	config.SetOrgIDs(mp)
	return nil
}
