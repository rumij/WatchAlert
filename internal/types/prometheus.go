package types

import "watchAlert/internal/models"

// 服务组
type RequestPrometheusCreateTargetGroup struct {
	TenantId string `json:"tenantId"`
	ID       int64  `json:"id"`
	Name     string `json:"name"`
}

type RequestPrometheusUpdateTargetGroup struct {
	TenantId string `json:"tenantId"`
	ID       int64  `json:"id"`
	Name     string `json:"name"`
}

type RequestPrometheusDeleteTargetGroup struct {
	TenantId string `json:"tenantId"`
	ID       int64  `json:"id"`
}

type RequestPrometheusGetTargetGroup struct {
	TenantId string `json:"tenantId" form:"tenantId"`
	ID       int64  `json:"id" form:"id"`
}

type RequestPrometheusListTargetGroup struct {
	TenantId string `json:"tenantId" form:"tenantId"`
	ID       int64  `json:"id" form:"id"`
	Query    string `json:"query" form:"query"`
	models.Page
}

type ResponsePrometheusTargetGroupList struct {
	List []models.PrometheusTargetGroup `json:"list"`
	models.Page
}

// 服务
type RequestPrometheusCreateTarget struct {
	TenantId     string                       `json:"tenantId"`
	GroupId      int64                        `json:"groupId"`
	ID           int64                        `json:"id"`
	Targets      []string                     `json:"targets" gorm:"targets;serializer:json"`
	Labels       map[string]string            `json:"labels" gorm:"labels;serializer:json"`
	TargetLabels map[string]map[string]string `json:"targetLabels" gorm:"targetLabels;serializer:json"`
}

type RequestPrometheusUpdateTarget struct {
	TenantId     string                       `json:"tenantId"`
	GroupId      int64                        `json:"groupId"`
	ID           int64                        `json:"id"`
	Targets      []string                     `json:"targets" gorm:"targets;serializer:json"`
	Labels       map[string]string            `json:"labels" gorm:"labels;serializer:json"`
	TargetLabels map[string]map[string]string `json:"targetLabels" gorm:"targetLabels;serializer:json"`
}

type RequestPrometheusDeleteTarget struct {
	TenantId string `json:"tenantId"`
	GroupId  int64  `json:"groupId"`
	ID       int64  `json:"id"`
}

type RequestPrometheusGetTarget struct {
	TenantId string `json:"tenantId" form:"tenantId"`
	GroupId  int64  `json:"groupId" form:"groupId"`
	ID       int64  `json:"id" form:"id"`
}

type RequestPrometheusListTarget struct {
	TenantId string `json:"tenantId" form:"tenantId"`
	GroupId  int64  `json:"groupId" form:"groupId"`
	ID       int64  `json:"id" form:"id"`
	Query    string `json:"query" form:"query"`
	models.Page
}

type ResponsePrometheusTargetList struct {
	List []models.PrometheusTarget `json:"list"`
	models.Page
}

// 版本历史
type RequestPrometheusListTargetVersion struct {
	TenantId string `json:"tenantId" form:"tenantId"`
	TargetId int64  `json:"targetId" form:"targetId"`
	models.Page
}

type RequestPrometheusGetTargetVersion struct {
	TenantId string `json:"tenantId" form:"tenantId"`
	ID       int64  `json:"id" form:"id"`
}

type RequestPrometheusRollbackTargetVersion struct {
	TenantId string `json:"tenantId"`
	ID       int64  `json:"id"`
}

type ResponsePrometheusTargetVersionList struct {
	List []models.PrometheusTargetVersion `json:"list"`
	models.Page
}

// -----------------------------------------------------------------------
// Service Discovery（供 Prometheus HTTP SD 调用）
// -----------------------------------------------------------------------

// RequestPrometheusSDTargets query 参数
// includeGroup/excludeGroup   : 服务组名称（逗号分隔多个）
// includeTarget/excludeTarget : target 地址（逗号分隔多个）
// includeLabel/excludeLabel   : label，格式 key:value 或 key（逗号分隔多个）
type RequestPrometheusSDTargets struct {
	TenantId      string `form:"tenantId" binding:"required"`
	IncludeGroup  string `form:"includeGroup"`
	IncludeTarget string `form:"includeTarget"`
	IncludeLabel  string `form:"includeLabel"`
	ExcludeGroup  string `form:"excludeGroup"`
	ExcludeTarget string `form:"excludeTarget"`
	ExcludeLabel  string `form:"excludeLabel"`
}

// PrometheusSDItem 符合 Prometheus HTTP SD 规范的单条输出
type PrometheusSDItem struct {
	Targets []string          `json:"targets"`
	Labels  map[string]string `json:"labels"`
}

