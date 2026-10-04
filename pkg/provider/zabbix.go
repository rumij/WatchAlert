package provider

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"
	"watchAlert/internal/models"
)

const ZabbixDsProviderName = "Zabbix"

type ZabbixClient struct {
	endpoint   string
	token      string
	httpClient *http.Client
	requestID  atomic.Uint64
}

type ZabbixRPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
	ID      uint64      `json:"id"`
}

type ZabbixRPCError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

type zabbixRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	Result  json.RawMessage `json:"result"`
	Error   *ZabbixRPCError `json:"error"`
	ID      uint64          `json:"id"`
}

type ZabbixTag struct {
	Tag   string `json:"tag"`
	Value string `json:"value"`
}

type ZabbixAcknowledgement struct {
	Clock       string `json:"clock"`
	UserID      string `json:"userid"`
	Message     string `json:"message"`
	Action      string `json:"action"`
	Username    string `json:"username"`
	Name        string `json:"name"`
	Surname     string `json:"surname"`
	OldSeverity string `json:"old_severity"`
	NewSeverity string `json:"new_severity"`
}

type ZabbixSuppression struct {
	MaintenanceID string `json:"maintenanceid"`
	SuppressUntil string `json:"suppress_until"`
	UserID        string `json:"userid"`
}

type ZabbixProblem struct {
	EventID         string                  `json:"eventid"`
	Source          string                  `json:"source"`
	Object          string                  `json:"object"`
	ObjectID        string                  `json:"objectid"`
	Clock           string                  `json:"clock"`
	NS              string                  `json:"ns"`
	RecoveryEventID string                  `json:"r_eventid"`
	CauseEventID    string                  `json:"cause_eventid"`
	Name            string                  `json:"name"`
	Acknowledged    string                  `json:"acknowledged"`
	Severity        string                  `json:"severity"`
	Value           string                  `json:"value"`
	OpData          string                  `json:"opdata"`
	Tags            []ZabbixTag             `json:"tags"`
	Acknowledges    []ZabbixAcknowledgement `json:"acknowledges"`
	SuppressionData []ZabbixSuppression     `json:"suppression_data"`
}

type ZabbixHostRef struct {
	HostID string `json:"hostid"`
	Host   string `json:"host"`
	Name   string `json:"name"`
}

type ZabbixItem struct {
	ItemID    string `json:"itemid"`
	HostID    string `json:"hostid"`
	Name      string `json:"name"`
	Key       string `json:"key_"`
	LastValue string `json:"lastvalue"`
	Units     string `json:"units"`
}

type ZabbixDependency struct {
	TriggerID   string `json:"triggerid"`
	Description string `json:"description"`
}

type ZabbixTrigger struct {
	TriggerID          string             `json:"triggerid"`
	Description        string             `json:"description"`
	Expression         string             `json:"expression"`
	RecoveryExpression string             `json:"recovery_expression"`
	Priority           string             `json:"priority"`
	Comments           string             `json:"comments"`
	OpData             string             `json:"opdata"`
	URL                string             `json:"url"`
	Status             string             `json:"status"`
	Value              string             `json:"value"`
	LastChange         string             `json:"lastchange"`
	ManualClose        string             `json:"manual_close"`
	Hosts              []ZabbixHostRef    `json:"hosts"`
	Items              []ZabbixItem       `json:"items"`
	Dependencies       []ZabbixDependency `json:"dependencies"`
	Tags               []ZabbixTag        `json:"tags"`
}

type ZabbixInterface struct {
	InterfaceID string `json:"interfaceid"`
	IP          string `json:"ip"`
	DNS         string `json:"dns"`
	Port        string `json:"port"`
	Type        string `json:"type"`
	Main        string `json:"main"`
	UseIP       string `json:"useip"`
}

type ZabbixNamedObject struct {
	GroupID    string `json:"groupid,omitempty"`
	TemplateID string `json:"templateid,omitempty"`
	Name       string `json:"name"`
}

type ZabbixHost struct {
	HostID          string              `json:"hostid"`
	Host            string              `json:"host"`
	Name            string              `json:"name"`
	Status          string              `json:"status"`
	Interfaces      []ZabbixInterface   `json:"interfaces"`
	Groups          []ZabbixNamedObject `json:"hostgroups"`
	ParentTemplates []ZabbixNamedObject `json:"parentTemplates"`
	Tags            []ZabbixTag         `json:"tags"`
	Inventory       json.RawMessage     `json:"inventory"`
}

type ZabbixEvent struct {
	EventID         string `json:"eventid"`
	Clock           string `json:"clock"`
	NS              string `json:"ns"`
	RecoveryEventID string `json:"r_eventid"`
}

func NewZabbixClient(datasource models.AlertDataSource) (*ZabbixClient, error) {
	endpoint, err := normalizeZabbixEndpoint(datasource.HTTP.URL)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(datasource.ZabbixConfig.APIToken) == "" {
		return nil, fmt.Errorf("Zabbix API token is required")
	}

	transport := &http.Transport{
		Proxy:               http.ProxyFromEnvironment,
		TLSClientConfig:     &tls.Config{InsecureSkipVerify: datasource.ZabbixConfig.InsecureSkipVerify}, // #nosec G402 -- explicitly configured for private deployments
		MaxIdleConns:        20,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
	}
	return &ZabbixClient{
		endpoint: endpoint,
		token:    datasource.ZabbixConfig.APIToken,
		httpClient: &http.Client{
			Timeout:   time.Duration(datasource.ZabbixConfig.GetTimeout()) * time.Second,
			Transport: transport,
		},
	}, nil
}

func normalizeZabbixEndpoint(rawURL string) (string, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return "", fmt.Errorf("Zabbix URL is required")
	}
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("invalid Zabbix URL: %s", rawURL)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("unsupported Zabbix URL scheme: %s", parsed.Scheme)
	}
	cleanPath := strings.TrimRight(parsed.Path, "/")
	if strings.HasSuffix(cleanPath, "/index.php") {
		cleanPath = strings.TrimSuffix(cleanPath, "/index.php")
	}
	if !strings.HasSuffix(cleanPath, "/api_jsonrpc.php") {
		cleanPath += "/api_jsonrpc.php"
	}
	parsed.Path = cleanPath
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String(), nil
}

func (z *ZabbixClient) call(ctx context.Context, method string, params interface{}, result interface{}) error {
	reqBody := ZabbixRPCRequest{JSONRPC: "2.0", Method: method, Params: params, ID: z.requestID.Add(1)}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshal Zabbix request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, z.endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create Zabbix request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json-rpc")
	req.Header.Set("Authorization", "Bearer "+z.token)

	resp, err := z.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("call Zabbix %s: %w", method, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return fmt.Errorf("read Zabbix %s response: %w", method, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Zabbix %s returned HTTP %d: %s", method, resp.StatusCode, strings.TrimSpace(string(data)))
	}

	var rpcResp zabbixRPCResponse
	if err := json.Unmarshal(data, &rpcResp); err != nil {
		return fmt.Errorf("decode Zabbix %s response: %w", method, err)
	}
	if rpcResp.Error != nil {
		return fmt.Errorf("Zabbix %s error %d: %s (%s)", method, rpcResp.Error.Code, rpcResp.Error.Message, strings.TrimSpace(string(rpcResp.Error.Data)))
	}
	rawResult := bytes.TrimSpace(rpcResp.Result)
	if rpcResp.JSONRPC != "2.0" || rpcResp.ID != reqBody.ID || len(rawResult) == 0 || bytes.Equal(rawResult, []byte("null")) {
		return fmt.Errorf("Zabbix %s returned an invalid JSON-RPC response", method)
	}
	if result == nil {
		return nil
	}
	if err := json.Unmarshal(rpcResp.Result, result); err != nil {
		return fmt.Errorf("decode Zabbix %s result: %w", method, err)
	}
	return nil
}

func (z *ZabbixClient) Check() (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), z.httpClient.Timeout)
	defer cancel()
	var result []struct {
		EventID string `json:"eventid"`
	}
	err := z.call(ctx, "problem.get", map[string]interface{}{"output": []string{"eventid"}, "limit": 1}, &result)
	return err == nil, err
}

func (z *ZabbixClient) GetProblems(ctx context.Context) ([]ZabbixProblem, error) {
	var result []ZabbixProblem
	err := z.call(ctx, "problem.get", map[string]interface{}{
		"source":                0,
		"object":                0,
		"output":                []string{"eventid", "source", "object", "objectid", "clock", "ns", "r_eventid", "cause_eventid", "name", "acknowledged", "severity", "opdata"},
		"selectAcknowledges":    "extend",
		"selectTags":            "extend",
		"selectSuppressionData": "extend",
		"sortfield":             []string{"eventid"},
		"sortorder":             "ASC",
	}, &result)
	return result, err
}

// GetProblemEventsSince returns trigger PROBLEM events, including events that
// have already recovered. It closes the gap left by polling problem.get alone.
func (z *ZabbixClient) GetProblemEventsSince(ctx context.Context, timeFrom int64) ([]ZabbixProblem, error) {
	var result []ZabbixProblem
	err := z.call(ctx, "event.get", map[string]interface{}{
		"source":                0,
		"object":                0,
		"value":                 1,
		"time_from":             timeFrom,
		"output":                []string{"eventid", "source", "object", "objectid", "clock", "ns", "r_eventid", "cause_eventid", "name", "acknowledged", "severity", "opdata", "value"},
		"selectAcknowledges":    "extend",
		"selectTags":            "extend",
		"selectSuppressionData": "extend",
		"sortfield":             []string{"clock", "eventid"},
		"sortorder":             "ASC",
	}, &result)
	return result, err
}

func (z *ZabbixClient) GetTriggers(ctx context.Context, triggerIDs []string) ([]ZabbixTrigger, error) {
	if len(triggerIDs) == 0 {
		return []ZabbixTrigger{}, nil
	}
	var result []ZabbixTrigger
	err := z.call(ctx, "trigger.get", map[string]interface{}{
		"triggerids":         triggerIDs,
		"output":             []string{"triggerid", "description", "expression", "recovery_expression", "priority", "comments", "opdata", "url", "status", "value", "lastchange", "manual_close"},
		"selectHosts":        []string{"hostid", "host", "name"},
		"selectItems":        []string{"itemid", "hostid", "name", "key_", "lastvalue", "units"},
		"selectDependencies": []string{"triggerid", "description"},
		"selectTags":         "extend",
	}, &result)
	return result, err
}

func (z *ZabbixClient) GetHosts(ctx context.Context, hostIDs []string) ([]ZabbixHost, error) {
	if len(hostIDs) == 0 {
		return []ZabbixHost{}, nil
	}
	var result []ZabbixHost
	err := z.call(ctx, "host.get", map[string]interface{}{
		"hostids":               hostIDs,
		"output":                []string{"hostid", "host", "name", "status"},
		"selectInterfaces":      "extend",
		"selectHostGroups":      []string{"groupid", "name"},
		"selectParentTemplates": []string{"templateid", "name"},
		"selectTags":            "extend",
		"selectInventory":       "extend",
	}, &result)
	return result, err
}

func (z *ZabbixClient) GetEvents(ctx context.Context, eventIDs []string) ([]ZabbixEvent, error) {
	if len(eventIDs) == 0 {
		return []ZabbixEvent{}, nil
	}
	var result []ZabbixEvent
	err := z.call(ctx, "event.get", map[string]interface{}{
		"eventids": eventIDs,
		"output":   []string{"eventid", "clock", "ns", "r_eventid"},
	}, &result)
	return result, err
}
