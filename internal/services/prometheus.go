package services

import (
	"strings"
	"watchAlert/internal/ctx"
	"watchAlert/internal/models"
	"watchAlert/internal/types"
)

type prometheusService struct {
	ctx *ctx.Context
}

type InterPrometheusService interface {
	CreateTargetGroup(req interface{}) (interface{}, interface{})
	UpdateTargetGroup(req interface{}) (interface{}, interface{})
	DeleteTargetGroup(req interface{}) (interface{}, interface{})
	ListTargetGroup(req interface{}) (interface{}, interface{})
	GetTargetGroup(req interface{}) (interface{}, interface{})
	CreateTarget(req interface{}) (interface{}, interface{})
	UpdateTarget(req interface{}) (interface{}, interface{})
	DeleteTarget(req interface{}) (interface{}, interface{})
	ListTarget(req interface{}) (interface{}, interface{})
	GetTarget(req interface{}) (interface{}, interface{})
	ListTargetVersion(req interface{}) (interface{}, interface{})
	GetTargetVersion(req interface{}) (interface{}, interface{})
	RollbackTargetVersion(req interface{}) (interface{}, interface{})
	ListTargetsForSD(req interface{}) (interface{}, interface{})
}

func newInterPrometheusService(ctx *ctx.Context) InterPrometheusService {
	return &prometheusService{
		ctx: ctx,
	}
}

// -----------------------------------------------------------------------
// TargetGroup
// -----------------------------------------------------------------------

func (s prometheusService) CreateTargetGroup(req interface{}) (interface{}, interface{}) {
	r := req.(*types.RequestPrometheusCreateTargetGroup)
	err := s.ctx.DB.PrometheusTargetGroup().Create(&models.PrometheusTargetGroup{
		TenantId: r.TenantId,
		Name:     r.Name,
	})
	if err != nil {
		return nil, err
	}
	return nil, nil
}

func (s prometheusService) UpdateTargetGroup(req interface{}) (interface{}, interface{}) {
	r := req.(*types.RequestPrometheusUpdateTargetGroup)
	err := s.ctx.DB.PrometheusTargetGroup().Update(&models.PrometheusTargetGroup{
		ID:       r.ID,
		TenantId: r.TenantId,
		Name:     r.Name,
	})
	if err != nil {
		return nil, err
	}
	return nil, nil
}

func (s prometheusService) DeleteTargetGroup(req interface{}) (interface{}, interface{}) {
	r := req.(*types.RequestPrometheusDeleteTargetGroup)
	err := s.ctx.DB.PrometheusTargetGroup().Delete(r.TenantId, r.ID)
	if err != nil {
		return nil, err
	}
	return nil, nil
}

func (s prometheusService) ListTargetGroup(req interface{}) (interface{}, interface{}) {
	r := req.(*types.RequestPrometheusListTargetGroup)
	data, count, err := s.ctx.DB.PrometheusTargetGroup().List(r.TenantId, r.Query, r.Page)
	if err != nil {
		return nil, err
	}
	return types.ResponsePrometheusTargetGroupList{
		List: data,
		Page: models.Page{
			Index: r.Page.Index,
			Size:  r.Page.Size,
			Total: count,
		},
	}, nil
}

func (s prometheusService) GetTargetGroup(req interface{}) (interface{}, interface{}) {
	r := req.(*types.RequestPrometheusGetTargetGroup)
	data, err := s.ctx.DB.PrometheusTargetGroup().Get(r.TenantId, r.ID)
	if err != nil {
		return nil, err
	}
	return data, nil
}

// -----------------------------------------------------------------------
// Service
// -----------------------------------------------------------------------

// snapshotTarget 在变更前拍快照，为该条 target 创建新版本
func (s prometheusService) snapshotTarget(tenantId string, groupId, targetId int64) error {
	target, err := s.ctx.DB.PrometheusTarget().Get(tenantId, groupId, targetId)
	if err != nil {
		return err
	}
	return s.ctx.DB.PrometheusTargetVersion().Create(&models.PrometheusTargetVersion{
		TenantId:     tenantId,
		TargetId:     targetId,
		Targets:      target.Targets,
		Labels:       target.Labels,
		TargetLabels: target.TargetLabels,
	})
}

func (s prometheusService) CreateTarget(req interface{}) (interface{}, interface{}) {
	r := req.(*types.RequestPrometheusCreateTarget)
	err := s.ctx.DB.PrometheusTarget().Create(&models.PrometheusTarget{
		TenantId:     r.TenantId,
		GroupId:      r.GroupId,
		Targets:      r.Targets,
		Labels:       r.Labels,
		TargetLabels: r.TargetLabels,
	})
	if err != nil {
		return nil, err
	}
	return nil, nil
}

func (s prometheusService) UpdateTarget(req interface{}) (interface{}, interface{}) {
	r := req.(*types.RequestPrometheusUpdateTarget)
	if err := s.snapshotTarget(r.TenantId, r.GroupId, r.ID); err != nil {
		return nil, err
	}
	err := s.ctx.DB.PrometheusTarget().Update(&models.PrometheusTarget{
		ID:           r.ID,
		TenantId:     r.TenantId,
		GroupId:      r.GroupId,
		Targets:      r.Targets,
		Labels:       r.Labels,
		TargetLabels: r.TargetLabels,
	})
	if err != nil {
		return nil, err
	}
	return nil, nil
}

func (s prometheusService) DeleteTarget(req interface{}) (interface{}, interface{}) {
	r := req.(*types.RequestPrometheusDeleteTarget)
	if err := s.snapshotTarget(r.TenantId, r.GroupId, r.ID); err != nil {
		return nil, err
	}
	err := s.ctx.DB.PrometheusTarget().Delete(r.TenantId, r.GroupId, r.ID)
	if err != nil {
		return nil, err
	}
	return nil, nil
}

func (s prometheusService) ListTarget(req interface{}) (interface{}, interface{}) {
	r := req.(*types.RequestPrometheusListTarget)
	data, count, err := s.ctx.DB.PrometheusTarget().List(r.TenantId, r.GroupId, r.Query, r.Page)
	if err != nil {
		return nil, err
	}
	return types.ResponsePrometheusTargetList{
		List: data,
		Page: models.Page{
			Index: r.Page.Index,
			Size:  r.Page.Size,
			Total: count,
		},
	}, nil
}

func (s prometheusService) GetTarget(req interface{}) (interface{}, interface{}) {
	r := req.(*types.RequestPrometheusGetTarget)
	data, err := s.ctx.DB.PrometheusTarget().Get(r.TenantId, r.GroupId, r.ID)
	if err != nil {
		return nil, err
	}
	return data, nil
}

// -----------------------------------------------------------------------
// TargetVersion
// -----------------------------------------------------------------------

func (s prometheusService) ListTargetVersion(req interface{}) (interface{}, interface{}) {
	r := req.(*types.RequestPrometheusListTargetVersion)
	data, count, err := s.ctx.DB.PrometheusTargetVersion().List(r.TenantId, r.TargetId, r.Page)
	if err != nil {
		return nil, err
	}
	return types.ResponsePrometheusTargetVersionList{
		List: data,
		Page: models.Page{
			Index: r.Page.Index,
			Size:  r.Page.Size,
			Total: count,
		},
	}, nil
}

func (s prometheusService) GetTargetVersion(req interface{}) (interface{}, interface{}) {
	r := req.(*types.RequestPrometheusGetTargetVersion)
	data, err := s.ctx.DB.PrometheusTargetVersion().Get(r.TenantId, r.ID)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (s prometheusService) RollbackTargetVersion(req interface{}) (interface{}, interface{}) {
	r := req.(*types.RequestPrometheusRollbackTargetVersion)

	// 获取目标版本快照
	version, err := s.ctx.DB.PrometheusTargetVersion().Get(r.TenantId, r.ID)
	if err != nil {
		return nil, err
	}

	// 对当前状态拍快照（回滚前的状态也记录）
	current, err := s.ctx.DB.PrometheusTarget().Get(r.TenantId, 0, version.TargetId)
	if err != nil {
		return nil, err
	}
	if err := s.ctx.DB.PrometheusTargetVersion().Create(&models.PrometheusTargetVersion{
		TenantId:     r.TenantId,
		TargetId:     version.TargetId,
		Targets:      current.Targets,
		Labels:       current.Labels,
		TargetLabels: current.TargetLabels,
	}); err != nil {
		return nil, err
	}

	// 将 target 更新为版本快照的状态
	err = s.ctx.DB.PrometheusTarget().Update(&models.PrometheusTarget{
		ID:           current.ID,
		TenantId:     r.TenantId,
		GroupId:      current.GroupId,
		Targets:      version.Targets,
		Labels:       version.Labels,
		TargetLabels: version.TargetLabels,
	})
	if err != nil {
		return nil, err
	}

	return nil, nil
}

// -----------------------------------------------------------------------
// Service Discovery
// -----------------------------------------------------------------------

func (s prometheusService) ListTargetsForSD(req interface{}) (interface{}, interface{}) {
	r := req.(*types.RequestPrometheusSDTargets)

	// 拉取该租户下全部 target 记录
	targets, err := s.ctx.DB.PrometheusTarget().ListAllByTenant(r.TenantId)
	if err != nil {
		return nil, err
	}

	// 拉取组名映射 groupId -> groupName
	groups, err := s.ctx.DB.PrometheusTargetGroup().ListAllByTenant(r.TenantId)
	if err != nil {
		return nil, err
	}
	groupMap := make(map[int64]string, len(groups))
	for _, g := range groups {
		groupMap[g.ID] = g.Name
	}

	// 解析过滤条件
	includeGroup := splitAndTrim(r.IncludeGroup)
	includeTarget := splitAndTrim(r.IncludeTarget)
	includeLabel := splitAndTrim(r.IncludeLabel)
	excludeGroup := splitAndTrim(r.ExcludeGroup)
	excludeTarget := splitAndTrim(r.ExcludeTarget)
	excludeLabel := splitAndTrim(r.ExcludeLabel)

	var result []types.PrometheusSDItem

	for _, t := range targets {
		groupName := groupMap[t.GroupId]

		// 正向检索：各类别内部 OR（任意一项匹配即可），跨类别 AND（所有已指定类别都必须通过）
		if len(includeGroup) > 0 && !anyContains(groupName, includeGroup) {
			continue
		}
		if len(includeTarget) > 0 && !anyTargetAddrMatch(t.Targets, includeTarget) {
			continue
		}
		if len(includeLabel) > 0 && !anyLabelMatch(t.Labels, includeLabel) {
			continue
		}

		// 反向过滤：任意条件命中则排除
		if len(excludeGroup) > 0 && anyContains(groupName, excludeGroup) {
			continue
		}
		if len(excludeTarget) > 0 && anyTargetAddrMatch(t.Targets, excludeTarget) {
			continue
		}
		if len(excludeLabel) > 0 && anyLabelMatch(t.Labels, excludeLabel) {
			continue
		}

		// 展开为 Prometheus HTTP SD 格式
		for _, addr := range t.Targets {
			labels := make(map[string]string, len(t.Labels)+1)
			for k, v := range t.Labels {
				labels[k] = v
			}
			// TargetLabels 优先级高于全局 Labels
			if tl, ok := t.TargetLabels[addr]; ok {
				for k, v := range tl {
					labels[k] = v
				}
			}
			labels["__meta_group"] = groupName
			result = append(result, types.PrometheusSDItem{
				Targets: []string{addr},
				Labels:  labels,
			})
		}
	}

	if result == nil {
		result = []types.PrometheusSDItem{}
	}

	return result, nil
}

// -----------------------------------------------------------------------
// SD 过滤工具函数
// -----------------------------------------------------------------------

// splitAndTrim 将逗号分隔的字符串拆成切片，忽略空项
func splitAndTrim(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// anyContains 判断 value 是否包含 filters 中任意一项（大小写不敏感）
func anyContains(value string, filters []string) bool {
	v := strings.ToLower(value)
	for _, f := range filters {
		if strings.Contains(v, strings.ToLower(f)) {
			return true
		}
	}
	return false
}

// anyTargetAddrMatch 判断 addresses 中是否有任意地址包含 filters 中任意一项
func anyTargetAddrMatch(addresses, filters []string) bool {
	for _, addr := range addresses {
		if anyContains(addr, filters) {
			return true
		}
	}
	return false
}

// anyLabelMatch 判断 labels 是否匹配 filters 中任意一项
// filter 格式: "key:value"（key 和 value 都必须匹配）或 "key"（只检查 key 是否存在）
func anyLabelMatch(labels map[string]string, filters []string) bool {
	for _, f := range filters {
		if strings.Contains(f, ":") {
			parts := strings.SplitN(f, ":", 2)
			key := strings.TrimSpace(parts[0])
			val := strings.TrimSpace(parts[1])
			if v, ok := labels[key]; ok && strings.EqualFold(v, val) {
				return true
			}
		} else {
			if _, ok := labels[strings.TrimSpace(f)]; ok {
				return true
			}
		}
	}
	return false
}
