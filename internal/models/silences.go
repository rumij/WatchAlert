package models

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

type AlertSilences struct {
	TenantId      string         `json:"tenantId"`
	Name          string         `json:"name"`
	ID            string         `json:"id"`
	Labels        []SilenceLabel `json:"labels" gorm:"labels;serializer:json"`
	StartsAt      int64          `json:"startsAt"`
	UpdateBy      string         `json:"updateBy"`
	EndsAt        int64          `json:"endsAt"`
	UpdateAt      int64          `json:"updateAt"`
	FaultCenterId string         `json:"faultCenterId"`
	Comment       string         `json:"comment"`
	Status        int            `json:"status"` // 0 未生效, 1 进行中, 2 已失效
}

type SilenceLabel struct {
	Key      string `json:"key"`
	Value    string `json:"value"`
	Operator string `json:"operator"`
}

// Older event pages submit numeric and boolean Zabbix labels as JSON scalars.
// Normalize those values to the strings used by silence matching.
func (label *SilenceLabel) UnmarshalJSON(data []byte) error {
	var raw struct {
		Key      string          `json:"key"`
		Value    json.RawMessage `json:"value"`
		Operator string          `json:"operator"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if len(raw.Value) == 0 || string(raw.Value) == "null" {
		return fmt.Errorf("静默条件缺少匹配值")
	}

	var value string
	switch raw.Value[0] {
	case '"':
		if err := json.Unmarshal(raw.Value, &value); err != nil {
			return err
		}
	case 't', 'f':
		var boolean bool
		if err := json.Unmarshal(raw.Value, &boolean); err != nil {
			return err
		}
		value = strconv.FormatBool(boolean)
	default:
		var number json.Number
		if err := json.Unmarshal(raw.Value, &number); err != nil {
			return fmt.Errorf("静默条件的匹配值只能是字符串、数字或布尔值: %w", err)
		}
		value = number.String()
	}
	*label = SilenceLabel{Key: raw.Key, Value: value, Operator: raw.Operator}
	return nil
}

// StatusAt uses a half-open window: startsAt <= now < endsAt.
func (s AlertSilences) StatusAt(now int64) int {
	if s.EndsAt <= now || s.EndsAt <= s.StartsAt {
		return 2
	}
	if now < s.StartsAt {
		return 0
	}
	return 1
}

func (s AlertSilences) Validate(now int64) error {
	if s.StartsAt <= 0 || s.EndsAt <= s.StartsAt || s.EndsAt <= now {
		return fmt.Errorf("请选择有效的静默起止时间，结束时间必须晚于开始时间和当前时间")
	}
	if len(s.Labels) == 0 {
		return fmt.Errorf("至少需要一个静默条件")
	}
	for _, label := range s.Labels {
		if strings.TrimSpace(label.Key) == "" {
			return fmt.Errorf("静默条件名称不能为空")
		}
		if strings.TrimSpace(label.Value) == "" {
			return fmt.Errorf("静默条件 %s 的匹配值不能为空", label.Key)
		}
		switch label.Operator {
		case "=", "==", "!=", "contains", "!contains":
		case "=~", "!~":
			if _, err := regexp.Compile(label.Value); err != nil {
				return fmt.Errorf("静默条件 %s 的正则表达式无效: %w", label.Key, err)
			}
		default:
			return fmt.Errorf("不支持的静默操作符: %s", label.Operator)
		}
	}
	return nil
}
