package models

import (
	"billing-job/config"
	"billing-job/log"
	"time"
)

type Project struct {
	ID             uint
	Name           string
	Namespace      string
	OrganizationID uint
}

func GetProjectByNamespace(namespace string) (*Project, error) {
	var project Project
	err := DB.First(&project, "namespace = ?", namespace).Error
	return &project, err

}

func ListProject() ([]Project, error) {
	var projects []Project
	twoHoursAgo := time.Now().Add(-2 * time.Hour)
	err := DB.Where("deleted_at IS NULL OR deleted_at >= ?", twoHoursAgo).Find(&projects).Error
	return projects, err
}

func SetNamespace() (map[string]struct{}, error) {
	project, err := ListProject()
	if err != nil {
		log.SugarLogger.Errorf("Get project list error: %v", err)
		return nil, err
	}
	namespaces := make(map[string]struct{}, len(project))
	for _, p := range project {
		namespaces[p.Namespace] = struct{}{}
	}
	config.SetNamespaces(namespaces)
	return namespaces, nil
}
