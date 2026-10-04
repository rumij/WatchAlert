package zabbix

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"watchAlert/internal/cache"
	appctx "watchAlert/internal/ctx"
	"watchAlert/internal/models"
	"watchAlert/internal/repo"
	"watchAlert/pkg/provider"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type zabbixTestAlertCache struct {
	cache.AlertCacheInterface
	events map[string]*models.AlertCurEvent
}

func (c *zabbixTestAlertCache) GetAllEvents(models.AlertEventCacheKey) (map[string]*models.AlertCurEvent, error) {
	return c.events, nil
}

func (c *zabbixTestAlertCache) PushAlertEvent(event *models.AlertCurEvent) {
	c.events[event.Fingerprint] = event
}

func (c *zabbixTestAlertCache) GetEventFromCache(_, _, fingerprint string) (models.AlertCurEvent, error) {
	if event, ok := c.events[fingerprint]; ok {
		return *event, nil
	}
	return models.AlertCurEvent{}, nil
}

type zabbixTestFaultCenterCache struct {
	cache.FaultCenterCacheInterface
}

func (*zabbixTestFaultCenterCache) PushFaultCenterInfo(models.FaultCenter) {}

type zabbixTestEntryCache struct {
	cache.InterEntryCache
	alerts *zabbixTestAlertCache
}

func (c *zabbixTestEntryCache) Alert() cache.AlertCacheInterface { return c.alerts }
func (c *zabbixTestEntryCache) FaultCenter() cache.FaultCenterCacheInterface {
	return &zabbixTestFaultCenterCache{}
}

type zabbixTestRepo struct {
	repo.InterEntryRepo
	db *gorm.DB
}

func (r *zabbixTestRepo) DB() *gorm.DB { return r.db }
func (r *zabbixTestRepo) FaultCenter() repo.InterFaultCenterRepo {
	return &zabbixTestFaultCenterRepo{}
}

type zabbixTestFaultCenterRepo struct{ repo.InterFaultCenterRepo }

func (*zabbixTestFaultCenterRepo) Get(tenantID, id, _ string) (models.FaultCenter, error) {
	return models.FaultCenter{TenantId: tenantID, ID: id}, nil
}

func TestSyncOnceZabbixLifecycle(t *testing.T) {
	active, failHost, failRecovery := true, true, false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Method string `json:"method"`
			ID     uint64 `json:"id"`
			Params struct {
				EventIDs []string `json:"eventids"`
			} `json:"params"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			return
		}
		result := `[]`
		switch request.Method {
		case "problem.get":
			if active {
				result = `[{"eventid":"1001","objectid":"2001","clock":"1700000000","name":"link down","severity":"4"}]`
			}
		case "trigger.get":
			result = `[{"triggerid":"2001","hosts":[{"hostid":"3001","name":"core-sw"}]}]`
		case "host.get":
			if failHost {
				_, _ = w.Write([]byte(`{"jsonrpc":"2.0","error":{"code":-32603,"message":"host lookup failed"},"id":` + strconv.FormatUint(request.ID, 10) + `}`))
				return
			}
			result = `[{"hostid":"3001","name":"core-sw","interfaces":[{"main":"1","useip":"1","ip":"10.0.0.1"}],"inventory":[]}]`
		case "event.get":
			if len(request.Params.EventIDs) > 0 {
				if failRecovery {
					_, _ = w.Write([]byte(`{"jsonrpc":"2.0","error":{"code":-32603,"message":"recovery lookup failed"},"id":` + strconv.FormatUint(request.ID, 10) + `}`))
					return
				}
				if request.Params.EventIDs[0] == "1001" {
					result = `[{"eventid":"1001","r_eventid":"1002"}]`
				} else {
					result = `[{"eventid":"1002","clock":"1700000100"}]`
				}
			}
		default:
			t.Errorf("unexpected Zabbix method: %s", request.Method)
		}
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","result":` + result + `,"id":` + strconv.FormatUint(request.ID, 10) + `}`))
	}))
	defer server.Close()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	defer sqlDB.Close()
	if err := db.Exec("CREATE TABLE alert_rules (tenant_id TEXT, datasource_type TEXT)").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.ZabbixSyncState{}); err != nil {
		t.Fatal(err)
	}
	datasource := models.AlertDataSource{
		TenantId: "tenant-1", ID: "ds-1", Type: provider.ZabbixDsProviderName,
		HTTP:         models.HTTP{URL: server.URL},
		ZabbixConfig: models.DsZabbixConfig{APIToken: "test-token", FaultCenterId: "fc-1"},
	}
	client, err := provider.NewZabbixClient(datasource)
	if err != nil {
		t.Fatal(err)
	}
	alerts := &zabbixTestAlertCache{events: make(map[string]*models.AlertCurEvent)}
	s := &Syncer{
		ctx:    &appctx.Context{Ctx: context.Background(), DB: &zabbixTestRepo{db: db}, Redis: &zabbixTestEntryCache{alerts: alerts}},
		status: make(map[string]SyncStatus),
	}
	s.syncOnce(context.Background(), datasource, client)
	if len(alerts.events) != 0 || s.Status(datasource.ID).LastError == "" {
		t.Fatalf("failed host enrichment must not create an alert: %+v", s.Status(datasource.ID))
	}

	failHost = false
	s.syncOnce(context.Background(), datasource, client)
	event := alerts.events["zabbix:ds-1:1001"]
	if event == nil || event.IsRecovered || event.Labels["zabbix_host_ips"] != "10.0.0.1" || event.Labels["zabbix_inventory"] != nil || s.Status(datasource.ID).LastError != "" {
		t.Fatalf("active event was not enriched and synchronized: event=%+v status=%+v", event, s.Status(datasource.ID))
	}

	active, failRecovery = false, true
	s.syncOnce(context.Background(), datasource, client)
	if event.IsRecovered || s.Status(datasource.ID).LastError == "" {
		t.Fatalf("failed recovery lookup must retain active event: event=%+v status=%+v", event, s.Status(datasource.ID))
	}

	failRecovery = false
	s.syncOnce(context.Background(), datasource, client)
	if !event.IsRecovered || event.RecoverTime != 1700000100 || s.Status(datasource.ID).RecoveredProblems != 1 || s.Status(datasource.ID).LastError != "" {
		t.Fatalf("verified recovery was not synchronized: event=%+v status=%+v", event, s.Status(datasource.ID))
	}
}

func TestSyncRouteRequiresVerifiedRecovery(t *testing.T) {
	response := `"error":{"code":-32603,"message":"temporary failure"}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			ID     uint64 `json:"id"`
			Params struct {
				EventIDs []string `json:"eventids"`
			} `json:"params"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			return
		}
		if response != "" {
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0",` + response + `,"id":` + strconv.FormatUint(request.ID, 10) + `}`))
			return
		}
		result := `[{"eventid":"1001","r_eventid":"1002"}]`
		if len(request.Params.EventIDs) == 1 && request.Params.EventIDs[0] == "1002" {
			result = `[{"eventid":"1002","clock":"1700000100"}]`
		}
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","result":` + result + `,"id":` + strconv.FormatUint(request.ID, 10) + `}`))
	}))
	defer server.Close()

	client, err := provider.NewZabbixClient(models.AlertDataSource{
		HTTP: models.HTTP{URL: server.URL}, ZabbixConfig: models.DsZabbixConfig{APIToken: "test-token"},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, scenario := range []struct {
		name     string
		response string
		wantOK   bool
		wantTime int64
	}{
		{name: "API failure", response: response, wantOK: false},
		{name: "no recovery relation", response: `"result":[{"eventid":"1001","r_eventid":"0"}]`, wantOK: true},
		{name: "verified recovery", wantOK: true, wantTime: 1700000100},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			response = scenario.response
			wantRecovered := 0
			if scenario.wantTime != 0 {
				wantRecovered = 1
			}
			alert := &models.AlertCurEvent{
				TenantId: "tenant-1", DatasourceType: provider.ZabbixDsProviderName,
				DatasourceId: "ds-1", EventId: "zbx-ds-1-1001", RuleGroupId: "zabbix",
				Fingerprint: "zabbix:ds-1:1001", Labels: map[string]interface{}{"zabbix_eventid": "1001"},
			}
			alerts := &zabbixTestAlertCache{events: map[string]*models.AlertCurEvent{alert.Fingerprint: alert}}
			s := &Syncer{ctx: &appctx.Context{Ctx: context.Background(), Redis: &zabbixTestEntryCache{alerts: alerts}}}
			_, recovered, _, ok := s.syncRoute(context.Background(), models.AlertDataSource{TenantId: "tenant-1", ID: "ds-1"}, eventRoute{faultCenter: models.FaultCenter{ID: "fc-1"}}, nil, nil, false, nil, nil, client)
			if ok != scenario.wantOK || recovered != wantRecovered {
				t.Fatalf("unexpected result: recovered=%d, ok=%v", recovered, ok)
			}
			if alert.IsRecovered != (scenario.wantTime != 0) || alert.RecoverTime != scenario.wantTime {
				t.Fatalf("unexpected recovery state: recovered=%v, time=%d", alert.IsRecovered, alert.RecoverTime)
			}
		})
	}
}

func TestBuildEventKeepsZabbixIdentityAndOriginalSeverity(t *testing.T) {
	event := buildEvent(
		models.AlertDataSource{
			TenantId:     "tenant-1",
			ID:           "ds-1",
			Labels:       map[string]interface{}{"environment": "prod"},
			ZabbixConfig: models.DsZabbixConfig{PollInterval: 15},
		},
		models.FaultCenter{ID: "fc-1"},
		provider.ZabbixProblem{EventID: "1001", ObjectID: "2001", Clock: "1700000000", Name: "uplink down", Severity: "5"},
		provider.ZabbixTrigger{TriggerID: "2001", Hosts: []provider.ZabbixHostRef{{HostID: "3001", Name: "core-sw"}}},
		map[string]provider.ZabbixHost{"3001": {HostID: "3001", Name: "core-sw", Interfaces: []provider.ZabbixInterface{{Main: "1", UseIP: "1", IP: "10.0.0.1"}}, Inventory: json.RawMessage(`{"type":"switch"}`)}},
		models.AlertCurEvent{},
	)

	if event.EventId != "zbx-ds-1-1001" || event.Fingerprint != "zabbix:ds-1:1001" {
		t.Fatalf("unexpected identity: %#v", event)
	}
	if event.Severity != "P0" || event.Labels["zabbix_severity"] != "5" {
		t.Fatalf("severity was not mapped and preserved: %#v", event.Labels)
	}
	if event.Labels["instance"] != "core-sw" || event.Labels["zabbix_host_ips"] != "10.0.0.1" {
		t.Fatalf("host context missing: %#v", event.Labels)
	}
	if event.Labels["zabbix_inventory"] != `{"type":"switch"}` {
		t.Fatalf("host inventory missing: %#v", event.Labels)
	}
}

func TestMapSeverityCanBeOverridden(t *testing.T) {
	if got := mapSeverity(map[string]string{"2": "P1"}, "2"); got != "P1" {
		t.Fatalf("unexpected mapped severity: %s", got)
	}
}

func TestMatchesZabbixRuleFiltersEventContext(t *testing.T) {
	now := int64(1700003600)
	problem := provider.ZabbixProblem{
		EventID:      "1001",
		ObjectID:     "2001",
		Clock:        "1700000000",
		Name:         "Core uplink down",
		Severity:     "5",
		Acknowledged: "0",
		Tags:         []provider.ZabbixTag{{Tag: "site", Value: "Shanghai-DC"}},
	}
	trigger := provider.ZabbixTrigger{
		TriggerID: "2001",
		Hosts:     []provider.ZabbixHostRef{{HostID: "3001", Name: "core-sw-01"}},
	}
	hosts := map[string]provider.ZabbixHost{
		"3001": {
			HostID: "3001",
			Name:   "core-sw-01",
			Groups: []provider.ZabbixNamedObject{{GroupID: "4001", Name: "Network/Core"}},
		},
	}
	config := models.ZabbixRuleConfig{
		Severities:     []string{"4", "5"},
		HostNames:      []string{"core-sw"},
		HostGroupNames: []string{"network"},
		TriggerName:    "uplink",
		Tags:           []models.ZabbixTagFilter{{Tag: "site", Value: "shanghai"}},
		Acknowledged:   "no",
		Suppressed:     "no",
		MinDuration:    300,
	}

	if !matchesZabbixRule(config, problem, trigger, hosts, now) {
		t.Fatal("expected event to match all configured filters")
	}
	config.Acknowledged = "yes"
	if matchesZabbixRule(config, problem, trigger, hosts, now) {
		t.Fatal("unacknowledged event must not match acknowledged-only rule")
	}
}

func TestBuildEventForRuleUsesRuleIdentityAndRouting(t *testing.T) {
	enabled := true
	rule := models.AlertRule{
		RuleId:               "a-rule-1",
		RuleGroupId:          "group-1",
		RuleName:             "核心链路告警",
		RepeatNoticeInterval: 600,
		ExternalLabels:       map[string]string{"owner": "network"},
		Enabled:              &enabled,
	}
	event := buildEventForRule(
		models.AlertDataSource{TenantId: "tenant-1", ID: "ds-1", ZabbixConfig: models.DsZabbixConfig{PollInterval: 15}},
		models.FaultCenter{ID: "fc-2"},
		provider.ZabbixProblem{EventID: "1001", ObjectID: "2001", Clock: "1700000000", Name: "uplink down", Severity: "4"},
		provider.ZabbixTrigger{TriggerID: "2001"},
		map[string]provider.ZabbixHost{},
		models.AlertCurEvent{},
		&rule,
	)
	if event.RuleId != rule.RuleId || event.RuleGroupId != rule.RuleGroupId || event.FaultCenterId != "fc-2" {
		t.Fatalf("unexpected rule routing: %#v", event)
	}
	if event.Fingerprint != "zabbix:ds-1:1001:a-rule-1" || event.Labels["zabbix_eventid"] != "1001" {
		t.Fatalf("unexpected routed identity: %#v", event)
	}
	if event.Labels["owner"] != "network" || event.RepeatNoticeInterval != 600 {
		t.Fatalf("rule metadata missing: %#v", event)
	}
}
