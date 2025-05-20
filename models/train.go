package models

import (
	"billing-job/config"
	"github.com/pkg/errors"
	"time"
)

type TrainingJob struct {
	ID      uint64 `json:"id"`
	Name    string `json:"name"`
	UserID  string `json:"user_id"`
	JobName string `json:"job_name"`
}

func listTrainingJobs() ([]TrainingJob, error) {
	var jobs []TrainingJob
	twoHoursAgo := time.Now().Add(-2 * time.Hour)
	if err := DB.Where("deleted_at IS NULL OR deleted_at >= ?", twoHoursAgo).Find(&jobs).Error; err != nil {
		return nil, errors.Wrap(err, "failed to list training jobs")
	}
	return jobs, nil
}

func SetTrainJobs() error {
	jobs, err := listTrainingJobs()
	if err != nil {
		return errors.Wrap(err, "failed to list training jobs")
	}
	mp := make(map[string]string, len(jobs))
	for _, job := range jobs {
		mp[job.JobName] = job.UserID
	}
	config.SetTrainJobs(mp)
	return nil
}
