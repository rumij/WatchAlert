package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"watchAlert/internal/models"
)

func TestNormalizeZabbixEndpoint(t *testing.T) {
	tests := map[string]string{
		"https://zabbix.example.com/zabbix/":                    "https://zabbix.example.com/zabbix/api_jsonrpc.php",
		"https://zabbix.example.com/zabbix/index.php":           "https://zabbix.example.com/zabbix/api_jsonrpc.php",
		"https://zabbix.example.com/zabbix/api_jsonrpc.php":     "https://zabbix.example.com/zabbix/api_jsonrpc.php",
		"https://zabbix.example.com/index.php?action=dashboard": "https://zabbix.example.com/api_jsonrpc.php",
	}
	for input, expected := range tests {
		got, err := normalizeZabbixEndpoint(input)
		if err != nil {
			t.Fatalf("normalize %s: %v", input, err)
		}
		if got != expected {
			t.Fatalf("normalize %s: got %s, want %s", input, got, expected)
		}
	}
}

func TestZabbixClientGetProblems(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/zabbix/api_jsonrpc.php" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("authorization header was not set")
		}
		var request ZabbixRPCRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		if request.Method != "problem.get" {
			t.Errorf("unexpected method: %s", request.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","result":[{"eventid":"101","objectid":"202","clock":"1700000000","name":"link down","severity":"4","acknowledged":"0","tags":[{"tag":"site","value":"sh"}]}],"id":1}`))
	}))
	defer server.Close()

	client, err := NewZabbixClient(models.AlertDataSource{
		HTTP:         models.HTTP{URL: server.URL + "/zabbix"},
		ZabbixConfig: models.DsZabbixConfig{APIToken: "test-token"},
	})
	if err != nil {
		t.Fatal(err)
	}
	problems, err := client.GetProblems(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) != 1 || problems[0].EventID != "101" || problems[0].Tags[0].Value != "sh" {
		t.Fatalf("unexpected problems: %#v", problems)
	}
}

func TestZabbixClientGetHostsWithOptionalInventory(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request ZabbixRPCRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			return
		}
		if request.Method != "host.get" {
			t.Errorf("unexpected method: %s", request.Method)
		}
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","result":[{"hostid":"1","inventory":[]},{"hostid":"2","inventory":{"type":"switch"}}],"id":1}`))
	}))
	defer server.Close()

	client, err := NewZabbixClient(models.AlertDataSource{
		HTTP: models.HTTP{URL: server.URL}, ZabbixConfig: models.DsZabbixConfig{APIToken: "test-token"},
	})
	if err != nil {
		t.Fatal(err)
	}
	hosts, err := client.GetHosts(context.Background(), []string{"1", "2"})
	if err != nil {
		t.Fatal(err)
	}
	if len(hosts) != 2 || string(hosts[0].Inventory) != `[]` || string(hosts[1].Inventory) != `{"type":"switch"}` {
		t.Fatalf("unexpected hosts: %#v", hosts)
	}
}

func TestZabbixClientReturnsRPCError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","error":{"code":-32602,"message":"Invalid params.","data":"permission denied"},"id":1}`))
	}))
	defer server.Close()
	client, err := NewZabbixClient(models.AlertDataSource{
		HTTP:         models.HTTP{URL: server.URL},
		ZabbixConfig: models.DsZabbixConfig{APIToken: "test-token"},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.GetProblems(context.Background())
	if err == nil || !strings.Contains(err.Error(), "permission denied") {
		t.Fatalf("expected RPC error, got %v", err)
	}
}

func TestZabbixClientRejectsIncompleteSuccessResponse(t *testing.T) {
	for _, response := range []string{
		`{"jsonrpc":"2.0","result":null,"id":1}`,
		`{"jsonrpc":"2.0","result": null ,"id":1}`,
		`{"jsonrpc":"2.0","result":[],"id":2}`,
		`{"result":[],"id":1}`,
	} {
		t.Run(response, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(response))
			}))
			defer server.Close()
			client, err := NewZabbixClient(models.AlertDataSource{
				HTTP: models.HTTP{URL: server.URL}, ZabbixConfig: models.DsZabbixConfig{APIToken: "test-token"},
			})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := client.GetProblems(context.Background()); err == nil {
				t.Fatal("incomplete response must not be interpreted as no active problems")
			}
		})
	}
}
