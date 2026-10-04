package templates

import (
	"strings"
	"testing"
	"watchAlert/internal/models"

	"github.com/bytedance/sonic"
)

func TestFeishuJSONCardEscapesZabbixAlertContent(t *testing.T) {
	enabled := true
	alert := models.AlertCurEvent{
		RuleName:         "Huawei VRP: interface down",
		Severity:         "P2",
		Fingerprint:      "zabbix:ds-1:1001",
		Annotations:      "first line\nquoted \"value\" and \\ path",
		FirstTriggerTime: 1700000000,
		Labels:           map[string]interface{}{"instance": "core-sw-01"},
	}
	tmpl := models.NoticeTemplateExample{
		EnableFeiShuJsonCard: &enabled,
		TemplateFiring: `{
          "schema":"2.0",
          "header":{"template":"red","title":{"tag":"plain_text","content":"alert"}},
          "body":{"elements":[{"tag":"markdown","content":"${rule_name}\n${labels.instance}\n${annotations}"}]}
        }`,
	}

	rendered := feishuTemplate(alert, tmpl)
	var payload map[string]interface{}
	if err := sonic.Unmarshal([]byte(rendered), &payload); err != nil {
		t.Fatalf("rendered card is not valid JSON: %v\n%s", err, rendered)
	}
	card, ok := payload["card"].(map[string]interface{})
	if !ok {
		t.Fatalf("card body missing from rendered payload: %#v", payload)
	}
	body, ok := card["body"].(map[string]interface{})
	if !ok || body["elements"] == nil {
		t.Fatalf("card elements missing after rendering: %#v", card)
	}
	elements, ok := body["elements"].([]interface{})
	if !ok || len(elements) == 0 {
		t.Fatalf("card elements are invalid after rendering: %#v", body)
	}
	firstElement, ok := elements[0].(map[string]interface{})
	if !ok {
		t.Fatalf("first card element is invalid: %#v", elements[0])
	}
	content, ok := firstElement["content"].(string)
	if !ok || !containsAll(content, []string{
		"Huawei VRP",
		"core-sw-01",
		"first line\nquoted \"value\" and \\ path",
	}) {
		t.Fatalf("rendered card lost alert fields: %q", content)
	}
}

func containsAll(value string, expected []string) bool {
	for _, item := range expected {
		if !strings.Contains(value, item) {
			return false
		}
	}
	return true
}
