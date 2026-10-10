package zabbix

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"watchAlert/alert/process"
	"watchAlert/internal/ctx"
	"watchAlert/internal/models"
	"watchAlert/pkg/provider"

	"github.com/zeromicro/go-zero/core/logc"
	"gorm.io/gorm"
)

type SyncerInterface interface {
	RestartAll()
	Start(datasource models.AlertDataSource) error
	Stop(datasourceID string)
	StopAll()
	Status(datasourceID string) SyncStatus
}

type SyncStatus struct {
	Running           bool   `json:"running"`
	LastAttemptAt     int64  `json:"lastAttemptAt"`
	LastSuccessAt     int64  `json:"lastSuccessAt"`
	LastError         string `json:"lastError"`
	ActiveProblems    int    `json:"activeProblems"`
	RecoveredProblems int    `json:"recoveredProblems"`
	HistoryBackfilled int    `json:"historyBackfilled"`
}

type Syncer struct {
	ctx     *ctx.Context
	mu      sync.Mutex
	cancels map[string]context.CancelFunc
	status  map[string]SyncStatus
}

func NewSyncer(ctx *ctx.Context) SyncerInterface {
	return &Syncer{ctx: ctx, cancels: make(map[string]context.CancelFunc), status: make(map[string]SyncStatus)}
}

func (s *Syncer) RestartAll() {
	s.StopAll()
	list, err := s.ctx.DB.Datasource().List("", "", provider.ZabbixDsProviderName, "")
	if err != nil {
		logc.Errorf(s.ctx.Ctx, "加载 Zabbix 数据源失败: %v", err)
		return
	}
	for _, datasource := range list {
		if !datasource.GetEnabled() {
			continue
		}
		if err := s.Start(datasource); err != nil {
			logc.Errorf(s.ctx.Ctx, "启动 Zabbix 同步失败, datasource=%s(%s), err=%v", datasource.Name, datasource.ID, err)
		}
	}
}

func (s *Syncer) Start(datasource models.AlertDataSource) error {
	if datasource.Type != provider.ZabbixDsProviderName {
		return fmt.Errorf("unsupported datasource type: %s", datasource.Type)
	}
	if !datasource.GetEnabled() {
		s.Stop(datasource.ID)
		return nil
	}
	if datasource.ZabbixConfig.FaultCenterId == "" {
		return fmt.Errorf("Zabbix faultCenterId is required")
	}
	client, err := provider.NewZabbixClient(datasource)
	if err != nil {
		return err
	}
	if _, err := s.ctx.DB.FaultCenter().Get(datasource.TenantId, datasource.ZabbixConfig.FaultCenterId, ""); err != nil {
		return fmt.Errorf("Zabbix fault center %s not found: %w", datasource.ZabbixConfig.FaultCenterId, err)
	}

	s.Stop(datasource.ID)
	runCtx, cancel := context.WithCancel(s.ctx.Ctx)
	s.mu.Lock()
	s.cancels[datasource.ID] = cancel
	status := s.status[datasource.ID]
	status.Running = true
	status.LastError = ""
	s.status[datasource.ID] = status
	s.mu.Unlock()
	go s.run(runCtx, datasource, client)
	return nil
}

func (s *Syncer) Stop(datasourceID string) {
	s.mu.Lock()
	cancel := s.cancels[datasourceID]
	delete(s.cancels, datasourceID)
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	s.mu.Lock()
	status := s.status[datasourceID]
	status.Running = false
	s.status[datasourceID] = status
	s.mu.Unlock()
}

func (s *Syncer) StopAll() {
	s.mu.Lock()
	cancels := make([]context.CancelFunc, 0, len(s.cancels))
	for _, cancel := range s.cancels {
		cancels = append(cancels, cancel)
	}
	s.cancels = make(map[string]context.CancelFunc)
	s.mu.Unlock()
	for _, cancel := range cancels {
		cancel()
	}
	s.mu.Lock()
	for datasourceID, status := range s.status {
		status.Running = false
		s.status[datasourceID] = status
	}
	s.mu.Unlock()
}

func (s *Syncer) Status(datasourceID string) SyncStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status[datasourceID]
}

func (s *Syncer) run(runCtx context.Context, datasource models.AlertDataSource, client *provider.ZabbixClient) {
	interval := time.Duration(datasource.ZabbixConfig.GetPollInterval()) * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	logc.Infof(s.ctx.Ctx, "Zabbix 同步已启动, datasource=%s(%s), interval=%s", datasource.Name, datasource.ID, interval)
	s.syncOnce(runCtx, datasource, client)
	for {
		select {
		case <-ticker.C:
			s.syncOnce(runCtx, datasource, client)
		case <-runCtx.Done():
			logc.Infof(s.ctx.Ctx, "Zabbix 同步已停止, datasource=%s(%s)", datasource.Name, datasource.ID)
			return
		}
	}
}

func (s *Syncer) syncOnce(runCtx context.Context, datasource models.AlertDataSource, client *provider.ZabbixClient) {
	s.markAttempt(datasource.ID)
	requestCtx, cancel := context.WithTimeout(runCtx, time.Duration(datasource.ZabbixConfig.GetTimeout())*time.Second*4)
	defer cancel()

	problems, err := client.GetProblems(requestCtx)
	if err != nil {
		logc.Errorf(s.ctx.Ctx, "Zabbix problem.get 失败, datasource=%s(%s), err=%v", datasource.Name, datasource.ID, err)
		s.markError(datasource.ID, err)
		return
	}
	recentProblems, scanTo, recentOK := s.getRecentProblemEvents(requestCtx, datasource, client)
	problemsForEnrichment := make([]provider.ZabbixProblem, 0, len(problems)+len(recentProblems))
	problemsForEnrichment = append(problemsForEnrichment, problems...)
	problemsForEnrichment = append(problemsForEnrichment, recentProblems...)

	triggerIDs := make([]string, 0, len(problemsForEnrichment))
	for _, problem := range problemsForEnrichment {
		if problem.ObjectID != "" {
			triggerIDs = append(triggerIDs, problem.ObjectID)
		}
	}
	triggers, triggerErr := client.GetTriggers(requestCtx, uniqueStrings(triggerIDs))
	if triggerErr != nil {
		logc.Errorf(s.ctx.Ctx, "Zabbix trigger.get 补全失败, datasource=%s(%s), err=%v", datasource.Name, datasource.ID, triggerErr)
		s.markError(datasource.ID, triggerErr)
		return
	}
	triggerByID := make(map[string]provider.ZabbixTrigger, len(triggers))
	var hostIDs []string
	for _, trigger := range triggers {
		triggerByID[trigger.TriggerID] = trigger
		for _, host := range trigger.Hosts {
			hostIDs = append(hostIDs, host.HostID)
		}
	}
	hosts, hostErr := client.GetHosts(requestCtx, uniqueStrings(hostIDs))
	if hostErr != nil {
		logc.Errorf(s.ctx.Ctx, "Zabbix host.get 补全失败, datasource=%s(%s), err=%v", datasource.Name, datasource.ID, hostErr)
		s.markError(datasource.ID, hostErr)
		return
	}
	hostByID := make(map[string]provider.ZabbixHost, len(hosts))
	for _, host := range hosts {
		hostByID[host.HostID] = host
	}

	routes, customRules, err := s.loadRoutes(datasource)
	if err != nil {
		logc.Errorf(s.ctx.Ctx, "加载 Zabbix 查询规则失败, datasource=%s(%s), err=%v", datasource.Name, datasource.ID, err)
		s.markError(datasource.ID, err)
		return
	}

	totalActive, totalRecovered, totalBackfilled := 0, 0, 0
	historyOK := true
	for _, route := range routes {
		active, recovered, backfilled, ok := s.syncRoute(requestCtx, datasource, route, problems, recentProblems, recentOK, triggerByID, hostByID, client)
		totalActive += active
		totalRecovered += recovered
		totalBackfilled += backfilled
		historyOK = historyOK && ok
	}
	if customRules {
		// Close events created by the legacy datasource-level route after the first
		// custom Zabbix rule is enabled, otherwise they would remain active forever.
		if err := s.recoverLegacyRoute(requestCtx, datasource, client); err != nil {
			logc.Errorf(s.ctx.Ctx, "清理 Zabbix 默认路由事件失败, datasource=%s(%s), err=%v", datasource.Name, datasource.ID, err)
			historyOK = false
		}
	}
	if recentOK && historyOK {
		if err := s.saveSyncState(datasource, scanTo); err != nil {
			s.markError(datasource.ID, err)
			return
		}
	} else {
		s.markError(datasource.ID, fmt.Errorf("Zabbix event synchronization incomplete; see preceding errors"))
		return
	}

	logc.Infof(s.ctx.Ctx, "Zabbix 同步完成, datasource=%s(%s), active=%d, recovered=%d, history_backfilled=%d", datasource.Name, datasource.ID, totalActive, totalRecovered, totalBackfilled)
	s.markSuccess(datasource.ID, totalActive, totalRecovered, totalBackfilled)
}

type eventRoute struct {
	faultCenter models.FaultCenter
	rule        *models.AlertRule
}

func (s *Syncer) loadRoutes(datasource models.AlertDataSource) ([]eventRoute, bool, error) {
	var rules []models.AlertRule
	if err := s.ctx.DB.DB().Where("tenant_id = ? AND datasource_type = ?", datasource.TenantId, provider.ZabbixDsProviderName).Find(&rules).Error; err != nil {
		return nil, false, err
	}

	routes := make([]eventRoute, 0)
	associatedRules := 0
	for i := range rules {
		if !containsString(rules[i].DatasourceIdList, datasource.ID) {
			continue
		}
		associatedRules++
		if rules[i].Enabled == nil || !*rules[i].Enabled {
			continue
		}
		faultCenter, err := s.ctx.DB.FaultCenter().Get(datasource.TenantId, rules[i].FaultCenterId, "")
		if err != nil {
			return nil, true, fmt.Errorf("rule %s fault center %s: %w", rules[i].RuleId, rules[i].FaultCenterId, err)
		}
		s.ctx.Redis.FaultCenter().PushFaultCenterInfo(faultCenter)
		rule := rules[i]
		routes = append(routes, eventRoute{faultCenter: faultCenter, rule: &rule})
	}
	if associatedRules > 0 {
		return routes, true, nil
	}

	faultCenter, err := s.ctx.DB.FaultCenter().Get(datasource.TenantId, datasource.ZabbixConfig.FaultCenterId, "")
	if err != nil {
		return nil, false, fmt.Errorf("default fault center %s: %w", datasource.ZabbixConfig.FaultCenterId, err)
	}
	s.ctx.Redis.FaultCenter().PushFaultCenterInfo(faultCenter)
	return []eventRoute{{faultCenter: faultCenter}}, false, nil
}

func (s *Syncer) syncRoute(
	runCtx context.Context,
	datasource models.AlertDataSource,
	route eventRoute,
	problems []provider.ZabbixProblem,
	recentProblems []provider.ZabbixProblem,
	recentOK bool,
	triggerByID map[string]provider.ZabbixTrigger,
	hostByID map[string]provider.ZabbixHost,
	client *provider.ZabbixClient,
) (int, int, int, bool) {
	active := make(map[string]struct{}, len(problems))
	matchedProblems := make([]provider.ZabbixProblem, 0, len(problems))
	for _, problem := range problems {
		trigger := triggerByID[problem.ObjectID]
		if route.rule != nil && !matchesZabbixRule(route.rule.ZabbixRuleConfig, problem, trigger, hostByID, time.Now().Unix()) {
			continue
		}
		matchedProblems = append(matchedProblems, problem)
		fp := routeFingerprint(datasource.ID, problem.EventID, route.rule)
		active[fp] = struct{}{}
		cached, _ := s.ctx.Redis.Alert().GetEventFromCache(datasource.TenantId, route.faultCenter.ID, fp)
		event := buildEventForRule(datasource, route.faultCenter, problem, trigger, hostByID, cached, route.rule)
		s.ctx.Redis.Alert().PushAlertEvent(&event)
	}

	allEvents, err := s.ctx.Redis.Alert().GetAllEvents(models.BuildAlertEventCacheKey(datasource.TenantId, route.faultCenter.ID))
	if err != nil {
		return len(matchedProblems), 0, 0, false
	}
	missing := make([]string, 0)
	missingByEventID := make(map[string]*models.AlertCurEvent)
	visibleProblemIDs := make(map[string]struct{}, len(problems))
	for _, problem := range problems {
		visibleProblemIDs[problem.EventID] = struct{}{}
	}
	for _, event := range allEvents {
		if event.DatasourceType != provider.ZabbixDsProviderName || event.DatasourceId != datasource.ID || event.IsRecovered || !eventBelongsToRoute(event, route.rule) {
			continue
		}
		if _, ok := active[event.Fingerprint]; ok {
			continue
		}
		externalID := zabbixExternalEventID(event, datasource.ID)
		if externalID != "" {
			missing = append(missing, externalID)
			missingByEventID[externalID] = event
		}
	}

	recoveryTimes, err := s.getRecoveryTimes(runCtx, client, missing)
	if err != nil {
		return len(matchedProblems), 0, 0, false
	}
	now := time.Now().Unix()
	recovered := 0
	for eventID, event := range missingByEventID {
		recoverAt := recoveryTimes[eventID]
		if recoverAt == 0 {
			// A still-visible problem may stop matching a rule after a filter
			// change. Only that case permits a synthetic recovery time.
			if _, visible := visibleProblemIDs[eventID]; !visible {
				continue
			}
			recoverAt = now
		}
		event.Status = models.StateRecovered
		event.IsRecovered = true
		event.RecoverTime = recoverAt
		event.LastEvalTime = now
		s.ctx.Redis.Alert().PushAlertEvent(event)
		recovered++
	}

	backfilled := 0
	historyOK := true
	if recentOK {
		matchedRecent := make([]provider.ZabbixProblem, 0, len(recentProblems))
		for _, problem := range recentProblems {
			if route.rule == nil || matchesZabbixRule(route.rule.ZabbixRuleConfig, problem, triggerByID[problem.ObjectID], hostByID, now) {
				matchedRecent = append(matchedRecent, problem)
			}
		}
		backfilled, historyOK = s.recordRecoveredHistory(datasource, route.faultCenter, matchedRecent, active, allEvents, triggerByID, hostByID, client, runCtx, route.rule)
	}
	return len(matchedProblems), recovered, backfilled, historyOK
}

func (s *Syncer) recoverLegacyRoute(runCtx context.Context, datasource models.AlertDataSource, client *provider.ZabbixClient) error {
	faultCenter, err := s.ctx.DB.FaultCenter().Get(datasource.TenantId, datasource.ZabbixConfig.FaultCenterId, "")
	if err != nil {
		return err
	}
	events, err := s.ctx.Redis.Alert().GetAllEvents(models.BuildAlertEventCacheKey(datasource.TenantId, faultCenter.ID))
	if err != nil {
		return err
	}
	missingByEventID := make(map[string]*models.AlertCurEvent)
	var missing []string
	for _, event := range events {
		if event.DatasourceType != provider.ZabbixDsProviderName || event.DatasourceId != datasource.ID || event.IsRecovered || event.RuleGroupId != "zabbix" {
			continue
		}
		externalID := zabbixExternalEventID(event, datasource.ID)
		if externalID != "" {
			missing = append(missing, externalID)
			missingByEventID[externalID] = event
		}
	}
	recoveryTimes, err := s.getRecoveryTimes(runCtx, client, missing)
	if err != nil {
		return err
	}
	now := time.Now().Unix()
	for eventID, event := range missingByEventID {
		recoverAt := recoveryTimes[eventID]
		if recoverAt == 0 {
			recoverAt = now
		}
		event.Status = models.StateRecovered
		event.IsRecovered = true
		event.RecoverTime = recoverAt
		event.LastEvalTime = now
		s.ctx.Redis.Alert().PushAlertEvent(event)
	}
	return nil
}

func (s *Syncer) markAttempt(datasourceID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	status := s.status[datasourceID]
	status.LastAttemptAt = time.Now().Unix()
	s.status[datasourceID] = status
}

func (s *Syncer) markError(datasourceID string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	status := s.status[datasourceID]
	status.LastError = err.Error()
	s.status[datasourceID] = status
}

func (s *Syncer) markSuccess(datasourceID string, active, recovered, backfilled int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	status := s.status[datasourceID]
	status.LastSuccessAt = time.Now().Unix()
	status.LastError = ""
	status.ActiveProblems = active
	status.RecoveredProblems = recovered
	status.HistoryBackfilled = backfilled
	s.status[datasourceID] = status
}

func (s *Syncer) getRecentProblemEvents(runCtx context.Context, datasource models.AlertDataSource, client *provider.ZabbixClient) ([]provider.ZabbixProblem, int64, bool) {
	scanTo := time.Now().Unix()
	timeFrom := scanTo - datasource.ZabbixConfig.GetInitialLookback()
	var state models.ZabbixSyncState
	err := s.ctx.DB.DB().Where("datasource_id = ?", datasource.ID).First(&state).Error
	if err == nil && state.LastEventSyncAt > 0 {
		timeFrom = state.LastEventSyncAt - datasource.ZabbixConfig.GetOverlap()
	} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		logc.Errorf(s.ctx.Ctx, "读取 Zabbix 增量同步水位失败, datasource=%s(%s), err=%v", datasource.Name, datasource.ID, err)
		return nil, 0, false
	}
	if timeFrom < 0 {
		timeFrom = 0
	}
	events, err := client.GetProblemEventsSince(runCtx, timeFrom)
	if err != nil {
		logc.Errorf(s.ctx.Ctx, "Zabbix event.get 增量查询失败, datasource=%s(%s), time_from=%d, err=%v", datasource.Name, datasource.ID, timeFrom, err)
		return nil, 0, false
	}
	return events, scanTo, true
}

func (s *Syncer) saveSyncState(datasource models.AlertDataSource, scanTo int64) error {
	state := models.ZabbixSyncState{
		DatasourceId:    datasource.ID,
		TenantId:        datasource.TenantId,
		LastEventSyncAt: scanTo,
		UpdatedAt:       time.Now().Unix(),
	}
	if err := s.ctx.DB.DB().Save(&state).Error; err != nil {
		logc.Errorf(s.ctx.Ctx, "保存 Zabbix 增量同步水位失败, datasource=%s(%s), err=%v", datasource.Name, datasource.ID, err)
		return err
	}
	return nil
}

func (s *Syncer) recordRecoveredHistory(
	datasource models.AlertDataSource,
	faultCenter models.FaultCenter,
	recentProblems []provider.ZabbixProblem,
	active map[string]struct{},
	current map[string]*models.AlertCurEvent,
	triggerByID map[string]provider.ZabbixTrigger,
	hostByID map[string]provider.ZabbixHost,
	client *provider.ZabbixClient,
	runCtx context.Context,
	rule *models.AlertRule,
) (int, bool) {
	completed := make([]provider.ZabbixProblem, 0)
	for _, problem := range recentProblems {
		fp := routeFingerprint(datasource.ID, problem.EventID, rule)
		if problem.RecoveryEventID == "" || problem.RecoveryEventID == "0" {
			continue
		}
		if _, ok := active[fp]; ok {
			continue
		}
		if _, ok := current[fp]; ok {
			continue
		}
		completed = append(completed, problem)
	}
	if len(completed) == 0 {
		return 0, true
	}

	recoveryIDs := make([]string, 0, len(completed))
	for _, problem := range completed {
		recoveryIDs = append(recoveryIDs, problem.RecoveryEventID)
	}
	recoveryEvents, err := client.GetEvents(runCtx, uniqueStrings(recoveryIDs))
	if err != nil {
		logc.Errorf(s.ctx.Ctx, "Zabbix event.get 回补恢复时间失败, datasource=%s(%s), err=%v", datasource.Name, datasource.ID, err)
		return 0, false
	}
	recoveryTimes := make(map[string]int64, len(recoveryEvents))
	for _, recovery := range recoveryEvents {
		recoveryTimes[recovery.EventID] = parseUnix(recovery.Clock)
	}

	backfilled := 0
	allOK := true
	for _, problem := range completed {
		recoverAt := recoveryTimes[problem.RecoveryEventID]
		if recoverAt == 0 {
			allOK = false
			continue
		}
		eventID := "zbx-" + datasource.ID + "-" + problem.EventID
		if rule != nil {
			eventID += "-" + rule.RuleId
		}
		var count int64
		if err := s.ctx.DB.DB().Model(&models.AlertHisEvent{}).Where("tenant_id = ? AND event_id = ?", datasource.TenantId, eventID).Count(&count).Error; err != nil {
			logc.Errorf(s.ctx.Ctx, "检查 Zabbix 历史事件失败, event=%s, err=%v", problem.EventID, err)
			allOK = false
			continue
		}
		if count > 0 {
			continue
		}
		event := buildEventForRule(datasource, faultCenter, problem, triggerByID[problem.ObjectID], hostByID, models.AlertCurEvent{}, rule)
		event.Status = models.StateRecovered
		event.IsRecovered = true
		event.RecoverTime = recoverAt
		event.LastEvalTime = recoverAt
		if err := process.RecordAlertHisEvent(s.ctx, event); err != nil {
			logc.Errorf(s.ctx.Ctx, "回补 Zabbix 历史事件失败, event=%s, err=%v", problem.EventID, err)
			allOK = false
			continue
		}
		backfilled++
	}
	return backfilled, allOK
}

func (s *Syncer) getRecoveryTimes(runCtx context.Context, client *provider.ZabbixClient, problemEventIDs []string) (map[string]int64, error) {
	result := make(map[string]int64)
	problemEvents, err := client.GetEvents(runCtx, problemEventIDs)
	if err != nil {
		logc.Errorf(s.ctx.Ctx, "Zabbix event.get 查询恢复关系失败: %v", err)
		return nil, err
	}
	var recoveryIDs []string
	for _, event := range problemEvents {
		if event.RecoveryEventID == "" || event.RecoveryEventID == "0" {
			continue
		}
		recoveryIDs = append(recoveryIDs, event.RecoveryEventID)
	}
	recoveryEvents, err := client.GetEvents(runCtx, uniqueStrings(recoveryIDs))
	if err != nil {
		logc.Errorf(s.ctx.Ctx, "Zabbix event.get 查询恢复时间失败: %v", err)
		return nil, err
	}
	recoveryTimes := make(map[string]int64, len(recoveryEvents))
	for _, event := range recoveryEvents {
		recoveryTimes[event.EventID] = parseUnix(event.Clock)
	}
	// One OK event can close several problems generated in Multiple mode.
	for _, problem := range problemEvents {
		if recoverAt := recoveryTimes[problem.RecoveryEventID]; recoverAt != 0 {
			result[problem.EventID] = recoverAt
		}
	}
	return result, nil
}

func buildEvent(datasource models.AlertDataSource, faultCenter models.FaultCenter, problem provider.ZabbixProblem, trigger provider.ZabbixTrigger, hostByID map[string]provider.ZabbixHost, cached models.AlertCurEvent) models.AlertCurEvent {
	return buildEventForRule(datasource, faultCenter, problem, trigger, hostByID, cached, nil)
}

func buildEventForRule(datasource models.AlertDataSource, faultCenter models.FaultCenter, problem provider.ZabbixProblem, trigger provider.ZabbixTrigger, hostByID map[string]provider.ZabbixHost, cached models.AlertCurEvent, rule *models.AlertRule) models.AlertCurEvent {
	now := time.Now().Unix()
	firstTrigger := parseUnix(problem.Clock)
	if firstTrigger == 0 {
		firstTrigger = now
	}
	labels := cloneLabels(datasource.Labels)
	if rule != nil {
		for key, value := range rule.ExternalLabels {
			labels[key] = value
		}
		labels["watchalert_rule_id"] = rule.RuleId
		labels["watchalert_rule_name"] = rule.RuleName
	}
	labels["source"] = "zabbix"
	labels["zabbix_eventid"] = problem.EventID
	labels["zabbix_triggerid"] = problem.ObjectID
	labels["zabbix_problem_name"] = problem.Name
	labels["zabbix_trigger_name"] = trigger.Description
	labels["zabbix_severity"] = problem.Severity
	labels["zabbix_severity_name"] = zabbixSeverityName(problem.Severity)
	labels["zabbix_acknowledged"] = problem.Acknowledged == "1"
	labels["zabbix_suppressed"] = len(problem.SuppressionData) > 0
	labels["zabbix_cause_eventid"] = problem.CauseEventID

	hostNames := make([]string, 0, len(trigger.Hosts))
	hostIDs := make([]string, 0, len(trigger.Hosts))
	IPs := make([]string, 0)
	groups := make([]string, 0)
	templates := make([]string, 0)
	for _, hostRef := range trigger.Hosts {
		hostIDs = append(hostIDs, hostRef.HostID)
		host := hostByID[hostRef.HostID]
		name := host.Name
		if name == "" {
			name = hostRef.Name
		}
		if name == "" {
			name = hostRef.Host
		}
		hostNames = append(hostNames, name)
		for _, iface := range host.Interfaces {
			if iface.Main == "1" {
				if iface.UseIP == "1" {
					IPs = append(IPs, iface.IP)
				} else {
					IPs = append(IPs, iface.DNS)
				}
			}
		}
		for _, group := range host.Groups {
			groups = append(groups, group.Name)
		}
		for _, template := range host.ParentTemplates {
			templates = append(templates, template.Name)
		}
		for _, tag := range host.Tags {
			labels["host_tag_"+tag.Tag] = tag.Value
		}
		if inventory := bytes.TrimSpace(host.Inventory); len(inventory) > 0 && inventory[0] == '{' {
			labels["zabbix_inventory"] = string(inventory)
		}
	}
	for _, tag := range problem.Tags {
		labels["zabbix_tag_"+tag.Tag] = tag.Value
	}
	labels["instance"] = strings.Join(uniqueStrings(hostNames), ",")
	labels["zabbix_hostids"] = strings.Join(uniqueStrings(hostIDs), ",")
	labels["zabbix_host_ips"] = strings.Join(uniqueStrings(IPs), ",")
	labels["zabbix_hostgroups"] = strings.Join(uniqueStrings(groups), ",")
	labels["zabbix_templates"] = strings.Join(uniqueStrings(templates), ",")
	labels["zabbix_item_count"] = len(trigger.Items)
	labels["zabbix_dependency_count"] = len(trigger.Dependencies)

	confirmState := cached.ConfirmState
	if problem.Acknowledged == "1" {
		confirmState.IsOk = true
		if ack, ok := latestAcknowledgement(problem.Acknowledges); ok {
			confirmState.ConfirmActionTime = parseUnix(ack.Clock)
			confirmState.ConfirmUsername = strings.TrimSpace(strings.Join([]string{ack.Username, ack.Name, ack.Surname}, " "))
			labels["zabbix_ack_message"] = ack.Message
		}
	}

	ruleGroupID := "zabbix"
	ruleID := problem.ObjectID
	ruleName := problem.Name
	eventID := "zbx-" + datasource.ID + "-" + problem.EventID
	eventFingerprint := fingerprint(datasource.ID, problem.EventID)
	repeatNoticeInterval := int64(0)
	effectiveTime := models.EffectiveTime{}
	if rule != nil {
		ruleGroupID = rule.RuleGroupId
		ruleID = rule.RuleId
		ruleName = rule.RuleName
		eventID += "-" + rule.RuleId
		eventFingerprint += ":" + rule.RuleId
		repeatNoticeInterval = rule.RepeatNoticeInterval
		effectiveTime = rule.EffectiveTime
	}

	return models.AlertCurEvent{
		TenantId:             datasource.TenantId,
		EventId:              eventID,
		RuleGroupId:          ruleGroupID,
		RuleId:               ruleID,
		RuleName:             ruleName,
		DatasourceType:       provider.ZabbixDsProviderName,
		DatasourceId:         datasource.ID,
		Fingerprint:          eventFingerprint,
		Severity:             mapSeverity(datasource.ZabbixConfig.SeverityMapping, problem.Severity),
		Labels:               labels,
		SearchQL:             fmt.Sprintf("event.get eventids=%s", problem.EventID),
		EvalInterval:         datasource.ZabbixConfig.GetPollInterval(),
		RepeatNoticeInterval: repeatNoticeInterval,
		EffectiveTime:        effectiveTime,
		Annotations:          buildAnnotations(problem, trigger, hostNames),
		FirstTriggerTime:     firstTrigger,
		LastEvalTime:         now,
		LastSendTime:         cached.LastSendTime,
		FaultCenterId:        faultCenter.ID,
		FaultCenter:          faultCenter,
		ConfirmState:         confirmState,
		Status:               models.StateAlerting,
	}
}

func matchesZabbixRule(config models.ZabbixRuleConfig, problem provider.ZabbixProblem, trigger provider.ZabbixTrigger, hostByID map[string]provider.ZabbixHost, now int64) bool {
	if len(config.Severities) > 0 && !containsString(config.Severities, problem.Severity) {
		return false
	}
	if query := strings.TrimSpace(config.TriggerName); query != "" {
		name := problem.Name
		if name == "" {
			name = trigger.Description
		}
		if !strings.Contains(strings.ToLower(name), strings.ToLower(query)) {
			return false
		}
	}
	if !matchesTriState(config.Acknowledged, problem.Acknowledged == "1") || !matchesTriState(config.Suppressed, len(problem.SuppressionData) > 0) {
		return false
	}
	if config.MinDuration > 0 {
		started := parseUnix(problem.Clock)
		if started == 0 || now-started < config.MinDuration {
			return false
		}
	}

	hostNames := make([]string, 0)
	groupNames := make([]string, 0)
	tags := append([]provider.ZabbixTag{}, problem.Tags...)
	tags = append(tags, trigger.Tags...)
	for _, hostRef := range trigger.Hosts {
		host := hostByID[hostRef.HostID]
		hostNames = append(hostNames, hostRef.Host, hostRef.Name, host.Host, host.Name)
		for _, group := range host.Groups {
			groupNames = append(groupNames, group.Name)
		}
		tags = append(tags, host.Tags...)
	}
	if !matchesAnyText(config.HostNames, hostNames) || !matchesAnyText(config.HostGroupNames, groupNames) {
		return false
	}
	for _, filter := range config.Tags {
		if strings.TrimSpace(filter.Tag) == "" {
			continue
		}
		matched := false
		for _, tag := range tags {
			if strings.EqualFold(strings.TrimSpace(tag.Tag), strings.TrimSpace(filter.Tag)) &&
				(strings.TrimSpace(filter.Value) == "" || strings.Contains(strings.ToLower(tag.Value), strings.ToLower(strings.TrimSpace(filter.Value)))) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	return true
}

func matchesTriState(value string, actual bool) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "yes", "true", "1":
		return actual
	case "no", "false", "0":
		return !actual
	default:
		return true
	}
}

func matchesAnyText(filters, values []string) bool {
	if len(filters) == 0 {
		return true
	}
	for _, filter := range filters {
		needle := strings.ToLower(strings.TrimSpace(filter))
		if needle == "" {
			continue
		}
		for _, value := range values {
			if strings.Contains(strings.ToLower(value), needle) {
				return true
			}
		}
	}
	return false
}

func containsString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func routeFingerprint(datasourceID, eventID string, rule *models.AlertRule) string {
	value := fingerprint(datasourceID, eventID)
	if rule != nil {
		value += ":" + rule.RuleId
	}
	return value
}

func eventBelongsToRoute(event *models.AlertCurEvent, rule *models.AlertRule) bool {
	if rule == nil {
		return event.RuleGroupId == "zabbix"
	}
	return event.RuleId == rule.RuleId
}

func zabbixExternalEventID(event *models.AlertCurEvent, datasourceID string) string {
	if event.Labels != nil {
		if value, ok := event.Labels["zabbix_eventid"].(string); ok && value != "" {
			return value
		}
	}
	return strings.TrimPrefix(event.EventId, "zbx-"+datasourceID+"-")
}

func buildAnnotations(problem provider.ZabbixProblem, trigger provider.ZabbixTrigger, hostNames []string) string {
	parts := []string{"Zabbix告警: " + problem.Name}
	if len(hostNames) > 0 {
		parts = append(parts, "主机: "+strings.Join(uniqueStrings(hostNames), ", "))
	}
	if problem.OpData != "" {
		parts = append(parts, "运维数据: "+problem.OpData)
	} else if trigger.OpData != "" {
		parts = append(parts, "运维数据: "+trigger.OpData)
	}
	if trigger.Expression != "" {
		parts = append(parts, "表达式: "+trigger.Expression)
	}
	if trigger.RecoveryExpression != "" {
		parts = append(parts, "恢复表达式: "+trigger.RecoveryExpression)
	}
	if trigger.Comments != "" {
		parts = append(parts, "说明: "+trigger.Comments)
	}
	return strings.Join(parts, "\n")
}

func mapSeverity(mapping map[string]string, severity string) string {
	if mapped := mapping[severity]; mapped != "" {
		return mapped
	}
	switch severity {
	case "5":
		return "P0"
	case "4":
		return "P1"
	default:
		return "P2"
	}
}

func zabbixSeverityName(severity string) string {
	return map[string]string{"0": "Not classified", "1": "Information", "2": "Warning", "3": "Average", "4": "High", "5": "Disaster"}[severity]
}

func fingerprint(datasourceID, eventID string) string {
	return "zabbix:" + datasourceID + ":" + eventID
}

func parseUnix(value string) int64 {
	parsed, _ := strconv.ParseInt(value, 10, 64)
	return parsed
}

func uniqueStrings(values []string) []string {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		if value != "" {
			set[value] = struct{}{}
		}
	}
	result := make([]string, 0, len(set))
	for value := range set {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func cloneLabels(source map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{}, len(source)+16)
	for key, value := range source {
		result[key] = value
	}
	return result
}

func latestAcknowledgement(acks []provider.ZabbixAcknowledgement) (provider.ZabbixAcknowledgement, bool) {
	if len(acks) == 0 {
		return provider.ZabbixAcknowledgement{}, false
	}
	latest := acks[0]
	for _, ack := range acks[1:] {
		if parseUnix(ack.Clock) > parseUnix(latest.Clock) {
			latest = ack
		}
	}
	return latest, true
}
