package models

import (
	"billing-job/config"
	"github.com/pkg/errors"
	"time"
)

type Notebook struct {
	ID      uint64 `json:"id"`
	Name    string `json:"name"`
	UserID  string `json:"user_id"`
	JobName string `json:"job_name"`
}

func listNotebook() ([]Notebook, error) {
	var notebooks []Notebook
	twoHoursAgo := time.Now().Add(-2 * time.Hour)
	if err := DB.Where("deleted_at IS NULL OR deleted_at >= ?", twoHoursAgo).Find(&notebooks).Error; err != nil {
		return notebooks, err
	}
	return notebooks, nil
}

func SetNotebooks() error {
	notebooks, err := listNotebook()
	if err != nil {
		return errors.Wrap(err, "failed to list notebooks")
	}
	mp := make(map[string]string, len(notebooks))
	for _, notebook := range notebooks {
		mp[notebook.JobName] = notebook.UserID
	}
	config.SetNotebooks(mp)
	return nil
}
