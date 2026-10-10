package consumer

import (
	"fmt"
	"slices"
	"strings"
	"time"
	"watchAlert/alert/mute"
	"watchAlert/alert/process"
	"watchAlert/internal/ctx"
	"watchAlert/internal/models"
	mediums "watchAlert/pkg/medium"
	"watchAlert/pkg/templates"
	"watchAlert/pkg/tools"

	"github.com/zeromicro/go-zero/core/logc"
	"golang.org/x/sync/errgroup"
)

// handleAlert 处理告警逻辑
func handleAlert(ctx *ctx.Context, processType string, faultCenter models.FaultCenter, noticeId string, alerts []*models.AlertCurEvent) error {
	curTime := time.Now().Unix()
	g := new(errgroup.Group)

	// 获取通知对象详细信息
	noticeData, err := getNoticeData(ctx, faultCenter.TenantId, noticeId)
	if err != nil {
		logc.Error(ctx.Ctx, fmt.Sprintf("Failed to get notice data: %v", err))
		return err
	}

	// Filter individual events before aggregation so one silence cannot hide
	// unrelated events, or leak through an unmuted representative event.
	severityGroups := groupUnmutedAlerts(faultCenter, alerts)

	// 告警聚合
	var aggregationEvents map[string][]*models.AlertCurEvent
	if processType == "alarm" {
		aggregationEvents = alarmAggregation(processType, faultCenter, severityGroups)
	} else {
		aggregationEvents = severityGroups
	}

	for severity, events := range aggregationEvents {
		sourceEvents := severityGroups[severity]
		g.Go(func() error {
			if events == nil {
				return nil
			}

			// 获取当前事件等级对应的路由配置
			routes := getNoticeRoutes(noticeData, severity)
			for _, event := range events {
				if event.Fingerprint == "" {
					continue
				}

				if len(routes) == 0 {
					logc.Infof(ctx.Ctx, "没有匹配的通知策略, 告警事件名称: %s, 告警等级: %s, 通知对象名称: %s", event.RuleName, event.Severity, noticeData.Name)
					continue
				}

				if mute.IsMuted(mute.ParamsForEvent(event, faultCenter.RecoverNotify)) {
					continue
				}

				// 只有真正匹配到通知策略且未被静默时，才记录本次通知时间。
				// 聚合通知需要同时更新原始事件，避免同一批事件被重复聚合发送。
				if processType == "alarm" && !event.IsRecovered {
					eventsToMark := []*models.AlertCurEvent{event}
					if faultCenter.GetAlarmAggregationType() == "Rule" {
						eventsToMark = sourceEvents
					}
					markEventsAsSent(ctx, eventsToMark, curTime)
				}

				for _, route := range routes {
					// 设置值班用户信息
					dutyUsers := getDutyUsers(ctx, noticeData, route.NoticeType)
					event.DutyUser = strings.Join(dutyUsers, " ")

					// 生成告警内容
					content := generateAlertContent(ctx, event, noticeData, route)

					// 构建邮件信息
					email := models.Email{
						Subject: route.Subject,
						To:      route.To,
						CC:      route.CC,
					}

					phone := models.Phone{
						To: route.To,
					}

					sms := models.SMS{
						To: route.To,
					}

					if len(dutyUsers) > 0 {
						switch route.NoticeType {
						case "Phone":
							phone.To = append(phone.To, dutyUsers...)
						case "SMS":
							sms.To = append(sms.To, dutyUsers...)
						case "Email":
							email.To = append(email.To, dutyUsers...)
						}
					}

					// 发送告警
					err := mediums.Sender(ctx, mediums.SendParams{
						TenantId:    event.TenantId,
						EventId:     event.EventId,
						RuleName:    event.RuleName,
						Severity:    event.Severity,
						NoticeType:  route.NoticeType,
						NoticeId:    noticeId,
						NoticeName:  noticeData.Name,
						IsRecovered: event.IsRecovered,
						Hook:        route.Hook,
						Headers:     route.Headers,
						Email:       email,
						Phone:       phone,
						SMS:         sms,
						Content:     content,
						Sign:        route.Sign,
					})
					if err != nil {
						logc.Error(ctx.Ctx, fmt.Sprintf("Failed to send alert: %v", err))
					}
				}
			}

			return nil
		})
	}

	return g.Wait()
}

func groupUnmutedAlerts(faultCenter models.FaultCenter, alerts []*models.AlertCurEvent) map[string][]*models.AlertCurEvent {
	groups := make(map[string][]*models.AlertCurEvent)
	for _, event := range alerts {
		if event == nil || event.Fingerprint == "" || isMutedEvent(event, faultCenter) {
			continue
		}
		groups[event.Severity] = append(groups[event.Severity], event)
	}
	return groups
}

// alarmAggregation 告警聚合
func alarmAggregation(processType string, faultCenter models.FaultCenter, alertGroups map[string][]*models.AlertCurEvent) map[string][]*models.AlertCurEvent {
	// 仅当 processType 为 "alarm" 时执行聚合
	if processType != "alarm" {
		return alertGroups
	}

	newAlertGroups := make(map[string][]*models.AlertCurEvent, len(alertGroups))
	switch faultCenter.GetAlarmAggregationType() {
	case "Rule":
		for severity, events := range alertGroups {
			newAlertGroups[severity] = withRuleGroupByAlerts(events)
		}
	default:
		return alertGroups
	}

	return newAlertGroups
}

// withRuleGroupByAlerts 聚合告警
func withRuleGroupByAlerts(alerts []*models.AlertCurEvent) []*models.AlertCurEvent {
	if len(alerts) <= 1 {
		return alerts
	}

	event := *alerts[0]
	event.Annotations += fmt.Sprintf("\n聚合 %d 条消息，详情请前往 WatchAlert 查看\n", len(alerts))
	return []*models.AlertCurEvent{&event}
}

func markEventsAsSent(ctx *ctx.Context, events []*models.AlertCurEvent, sendTime int64) {
	for _, event := range events {
		if event == nil || event.IsRecovered || event.Fingerprint == "" {
			continue
		}
		event.LastSendTime = sendTime
		ctx.Redis.Alert().PushAlertEvent(event)
	}
}

// getNoticeData 获取 Notice 数据
func getNoticeData(ctx *ctx.Context, tenantId, noticeId string) (models.AlertNotice, error) {
	return ctx.DB.Notice().Get(tenantId, noticeId)
}

// getNoticeRoutes 获取事件等级对应的路由配置
func getNoticeRoutes(notice models.AlertNotice, severity string) []models.Route {
	var routes []models.Route
	if notice.Routes != nil {
		for i, route := range notice.Routes {
			if process.NotInTheEffectiveTime(route.EffectiveTime) {
				logc.Infof(ctx.Ctx, "Notice %v route [%v] is not in effective time", notice.Name, i+1)
				continue
			}
			if slices.Contains(route.Severitys, severity) {
				routes = append(routes, route)
			}
		}
	}

	return routes
}

type WebhookContent struct {
	Alarm     *models.AlertCurEvent `json:"alarm"`
	DutyUsers []models.DutyUser     `json:"dutyUsers"`
}

// generateAlertContent 生成告警内容
func generateAlertContent(ctx *ctx.Context, alert *models.AlertCurEvent, noticeData models.AlertNotice, route models.Route) string {
	if route.NoticeType == "WebHook" || route.NoticeType == "SREFlow" {
		users, ok := ctx.DB.DutyCalendar().GetDutyUserData(*noticeData.GetDutyId(), time.Now().Format("2006-1-2"))
		if !ok || len(users) == 0 {
			logc.Error(ctx.Ctx, "Failed to get duty users, noticeName: ", noticeData.Name)
		}

		var dutyUsers = []models.DutyUser{}
		for _, user := range users {
			dutyUsers = append(dutyUsers, models.DutyUser{
				Email:    user.Email,
				Mobile:   user.Phone,
				UserId:   user.UserId,
				Username: user.UserName,
			})
		}
		content := WebhookContent{
			Alarm:     alert,
			DutyUsers: dutyUsers,
		}

		return tools.JsonMarshalToString(content)
	}

	template, err := templates.NewTemplate(ctx, *alert, route)
	if err != nil {
		logc.Error(ctx.Ctx, fmt.Sprintf("Failed to create template: %v", err))
		return ""
	}
	return template.CardContentMsg
}

func getDutyUsers(ctx *ctx.Context, noticeData models.AlertNotice, noticeType string) []string {
	var us []string
	users, ok := ctx.DB.DutyCalendar().GetDutyUserData(*noticeData.GetDutyId(), time.Now().Format("2006-1-2"))
	if ok {
		switch noticeType {
		case "FeiShu":
			for _, user := range users {
				us = append(us, fmt.Sprintf("<at id=%s></at>", user.DutyUserId))
			}
			return us
		case "DingDing":
			for _, user := range users {
				us = append(us, fmt.Sprintf("@%s", user.DutyUserId))
			}
			return us
		case "Email", "WeChat", "WebHook", "SREFlow":
			for _, user := range users {
				us = append(us, fmt.Sprintf("@%s", user.UserName))
			}
			return us
		case "Slack":
			for _, user := range users {
				us = append(us, fmt.Sprintf("<@%s>", user.DutyUserId))
			}
			return us
		case "Phone", "SMS":
			for _, user := range users {
				if user.Phone == "" {
					continue
				}
				us = append(us, user.Phone)
			}
			return us
		}
	}

	return []string{"暂无"}
}
