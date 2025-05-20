package models

import (
	"billing-job/config"
	"github.com/pkg/errors"
	"time"
)

type InferenceJob struct {
	ID      uint64 `json:"id"`
	Name    string `json:"name"`
	UserID  string `json:"user_id"`
	JobName string `json:"job_name"`
}

func listInferences() ([]InferenceJob, error) {
	var inferenceJobs []InferenceJob
	twoHoursAgo := time.Now().Add(-2 * time.Hour)
	if err := DB.Where("deleted_at IS NULL OR deleted_at >= ?", twoHoursAgo).Find(&inferenceJobs).Error; err != nil {
		return nil, errors.Wrap(err, "failed to list InferenceJobs")
	}
	return inferenceJobs, nil
}

func SetInference() error {
	isvcs, err := listInferences()
	if err != nil {
		return errors.Wrap(err, "failed to list inference service")
	}
	mp := make(map[string]string, len(isvcs))
	for _, isvc := range isvcs {
		mp[isvc.JobName] = isvc.UserID
	}
	config.SetInferences(mp)
	return nil
}
