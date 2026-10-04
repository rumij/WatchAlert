package middleware

import (
	"net/url"
	"strings"
	"testing"
)

func TestSanitizeAuditBodyRedactsNestedSecrets(t *testing.T) {
	body := []byte(`{"name":"zabbix","zabbixConfig":{"apiToken":"plain-token"},"Auth":{"pass":"plain-password"}}`)
	got := sanitizeAuditBody(body)
	if strings.Contains(got, "plain-token") || strings.Contains(got, "plain-password") {
		t.Fatalf("audit body leaked a secret: %s", got)
	}
	if !strings.Contains(got, "******") || !strings.Contains(got, "zabbix") {
		t.Fatalf("audit body lost expected content: %s", got)
	}
}

func TestSanitizeAuditBodyOmitsInvalidJSON(t *testing.T) {
	got := sanitizeAuditBody([]byte(`{"apiToken":"plain-token"`))
	if got != "[unparseable request body omitted]" {
		t.Fatalf("unexpected invalid JSON handling: %s", got)
	}
}

func TestSanitizeQueryParamsMasksSensitiveValues(t *testing.T) {
	got := sanitizeQueryParams(url.Values{
		"apiToken": {"plain-token"},
		"tenant":   {"default"},
	})
	if got.Get("apiToken") != "******" {
		t.Fatalf("apiToken was not redacted: %q", got.Get("apiToken"))
	}
	if got.Get("tenant") != "default" {
		t.Fatalf("non-sensitive query parameter changed: %q", got.Get("tenant"))
	}
}
