package mute

import (
	"fmt"
	"regexp"
	"strings"
	"time"
	"watchAlert/internal/ctx"
	models "watchAlert/internal/models"

	"github.com/zeromicro/go-zero/core/logc"
)

type MuteParams struct {
	RecoverNotify *bool
	IsRecovered   bool
	TenantId      string
	FaultCenterId string
	Labels        map[string]interface{}
}

// ParamsForEvent provides stable selectors even when the datasource has no labels.
// Reserved watchalert_* selectors always use the event's authoritative fields.
func ParamsForEvent(event *models.AlertCurEvent, recoverNotify *bool) MuteParams {
	labels := make(map[string]interface{}, len(event.Labels)+7)
	for key, value := range event.Labels {
		labels[key] = value
	}
	labels["watchalert_fingerprint"] = event.Fingerprint
	labels["watchalert_event_id"] = event.EventId
	labels["watchalert_datasource_id"] = event.DatasourceId
	labels["watchalert_datasource_type"] = event.DatasourceType
	labels["watchalert_rule_id"] = event.RuleId
	labels["watchalert_rule_name"] = event.RuleName
	labels["watchalert_severity"] = event.Severity
	return MuteParams{
		TenantId: event.TenantId, FaultCenterId: event.FaultCenterId,
		Labels: labels, IsRecovered: event.IsRecovered, RecoverNotify: recoverNotify,
	}
}

func IsMuted(mute MuteParams) bool {
	if IsSilence(mute) {
		return true
	}

	if RecoverNotify(mute) {
		return true
	}

	return false
}

// RecoverNotify 判断是否推送恢复通知
func RecoverNotify(mp MuteParams) bool {
	return mp.IsRecovered && mp.RecoverNotify != nil && !*mp.RecoverNotify
}

// IsSilence 判断是否静默
func IsSilence(mute MuteParams) bool {
	silenceCtx := ctx.Redis.Silence()
	// 获取静默列表中所有的id
	ids, err := silenceCtx.GetAlertMutes(mute.TenantId)
	if err != nil {
		logc.Errorf(ctx.Ctx, "%v", err)
		return false
	}

	// 根据ID获取到详细的静默规则
	now := time.Now().Unix()
	for _, id := range ids {
		muteRule, err := silenceCtx.WithIdGetMuteFromCache(mute.TenantId, id)
		if err != nil {
			logc.Errorf(ctx.Ctx, "%v", err)
			return false
		}

		if muteRule == nil || muteRule.TenantId != mute.TenantId || muteRule.StatusAt(now) != 1 {
			continue
		}
		if muteRule.FaultCenterId != "" && muteRule.FaultCenterId != mute.FaultCenterId {
			continue
		}

		if evalCondition(mute.Labels, muteRule.Labels) {
			return true
		}
	}

	return false
}

func evalCondition(metrics map[string]interface{}, muteLabels []models.SilenceLabel) bool {
	if len(muteLabels) == 0 {
		return false
	}
	for _, muteLabel := range muteLabels {
		value, exists := metrics[muteLabel.Key]
		if !exists {
			return false
		}

		var val string
		switch value.(type) {
		case string, bool, float64, float32, int, int64, int32, uint, uint64, uint32:
			val = fmt.Sprint(value)
		default:
			return false
		}

		var matched bool
		switch muteLabel.Operator {
		case "==", "=":
			matched = (val == muteLabel.Value)
		case "!=":
			matched = (val != muteLabel.Value)
		case "contains", "!contains":
			matched = strings.Contains(strings.ToLower(val), strings.ToLower(muteLabel.Value))
			if muteLabel.Operator == "!contains" {
				matched = !matched
			}
		case "=~", "!~":
			expression, err := regexp.Compile(muteLabel.Value)
			if err != nil {
				return false
			}
			matched = expression.MatchString(val)
			if muteLabel.Operator == "!~" {
				matched = !matched
			}
		default:
			matched = false
		}

		if !matched {
			return false // 只要有一个不匹配，就不静默
		}
	}

	return true
}
