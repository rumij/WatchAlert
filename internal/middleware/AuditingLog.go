package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"strings"
	"time"
	"watchAlert/internal/ctx"
	"watchAlert/internal/models"
	"watchAlert/pkg/response"
	"watchAlert/pkg/tools"

	"github.com/gin-gonic/gin"
	"github.com/zeromicro/go-zero/core/logc"
)

func AuditingLog() gin.HandlerFunc {
	return func(context *gin.Context) {
		// Operation user
		var username string
		createBy := tools.GetUser(context.Request.Header.Get("Authorization"))
		if createBy != "" {
			username = createBy
		} else {
			username = "用户未登录"
		}

		// Response log
		body := context.Request.Body
		readBody, err := io.ReadAll(body)
		if err != nil {
			logc.Error(ctx.DO().Ctx, err)
			return
		}
		// 将 body 数据放回请求中
		context.Request.Body = ioutil.NopCloser(bytes.NewBuffer(readBody))

		// 获取请求的完整API路径
		fullPath := context.Request.URL.Path

		tid := context.Request.Header.Get(TenantIDHeaderKey)
		if tid == "" {
			response.Fail(context, "租户ID不能为空", "failed")
			context.Abort()
			return
		}

		// 当请求处理完成后才会执行 Next() 后面的代码
		context.Next()

		ps := models.AuditEventMap
		auditLog := models.AuditLog{
			TenantId:   tid,
			ID:         tools.RandId(),
			Username:   username,
			IPAddress:  context.ClientIP(),
			Method:     context.Request.Method,
			Path:       fullPath,
			CreatedAt:  time.Now().Unix(),
			StatusCode: context.Writer.Status(),
			Body:       sanitizeAuditBody(readBody),
			AuditType:  ps[fullPath],
		}

		c := ctx.DO()
		err = c.DB.AuditLog().Create(auditLog)
		if err != nil {
			response.Fail(context, "审计日志写入数据库失败, "+err.Error(), "failed")
			context.Abort()
			return
		}
	}
}

func sanitizeAuditBody(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	var value interface{}
	if err := json.Unmarshal(body, &value); err != nil {
		return "[unparseable request body omitted]"
	}
	redactSensitiveValues(value)
	sanitized, err := json.Marshal(value)
	if err != nil {
		return "[request body omitted]"
	}
	return string(sanitized)
}

func redactSensitiveValues(value interface{}) {
	switch current := value.(type) {
	case map[string]interface{}:
		for key, child := range current {
			if isSensitiveAuditKey(key) {
				current[key] = "******"
				continue
			}
			redactSensitiveValues(child)
		}
	case []interface{}:
		for _, child := range current {
			redactSensitiveValues(child)
		}
	}
}

func isSensitiveAuditKey(key string) bool {
	normalized := strings.NewReplacer("_", "", "-", "").Replace(strings.ToLower(key))
	switch normalized {
	case "pass", "password", "apitoken", "accesstoken", "refreshtoken", "secret", "secretkey", "accesskeysecret":
		return true
	default:
		return false
	}
}
