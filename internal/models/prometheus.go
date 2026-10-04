package models

import "time"

type PromQueryResponse struct {
	Data struct {
		Result []struct {
			Metric map[string]string `json:"metric"`
			Value  []string          `json:"value"`
		} `json:"result"`
	} `json:"data"`
}

type PrometheusTargetGroup struct {
	TenantId string `json:"tenantId" gorm:"index"`
	ID       int64  `json:"id" gorm:"autoIncrement"`
	Name     string `json:"name" gorm:"column:name"`
}

func (PrometheusTargetGroup) TableName() string {
	return "w8t_prometheus_target_group"
}

type PrometheusTarget struct {
	TenantId     string                       `json:"tenantId"`
	GroupId      int64                        `json:"groupId"`
	ID           int64                        `json:"id" gorm:"autoIncrement"`
	Targets      []string                     `json:"targets" gorm:"targets;serializer:json"`
	Labels       map[string]string            `json:"labels" gorm:"labels;serializer:json"`
	TargetLabels map[string]map[string]string `json:"targetLabels" gorm:"targetLabels;serializer:json"`
}

func (PrometheusTarget) TableName() string {
	return "w8t_prometheus_target"
}

// PrometheusTargetVersion 记录单条 target 的变更历史
type PrometheusTargetVersion struct {
	TenantId     string                       `json:"tenantId" gorm:"index"`
	TargetId     int64                        `json:"targetId" gorm:"index"`
	ID           int64                        `json:"id" gorm:"autoIncrement"`
	Version      int                          `json:"version"`
	Targets      []string                     `json:"targets" gorm:"targets;serializer:json"`
	Labels       map[string]string            `json:"labels" gorm:"labels;serializer:json"`
	TargetLabels map[string]map[string]string `json:"targetLabels" gorm:"targetLabels;serializer:json"`
	CreatedAt    time.Time                    `json:"createdAt" gorm:"autoCreateTime"`
}

func (PrometheusTargetVersion) TableName() string {
	return "w8t_prometheus_target_version"
}
