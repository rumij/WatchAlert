package api

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestBindingFailureNeverRunsServiceOrAppendsSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name, method, url, body string
		handler                 gin.HandlerFunc
	}{
		{"notice create", "POST", "/", `{"name":42,"routes":[]}`, NoticeController.Create},
		{"silence list", "GET", "/?index=bad", "", SilenceController.List},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(tc.method, tc.url, strings.NewReader(tc.body))
			c.Request.Header.Set("Content-Type", "application/json")
			c.Set("TenantID", "test")
			// No initialized services: executing business logic here is a failure.
			tc.handler(c)
			if w.Code != 400 {
				t.Fatalf("status = %d", w.Code)
			}
			decoder := json.NewDecoder(w.Body)
			var response map[string]interface{}
			if err := decoder.Decode(&response); err != nil {
				t.Fatal(err)
			}
			if response["code"] != float64(400) {
				t.Fatalf("response: %v", response)
			}
			if err := decoder.Decode(&response); err != io.EOF {
				t.Fatalf("extra response: %v", err)
			}
		})
	}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	called := false
	Service(c, func() (interface{}, interface{}) { called = true; return "ok", nil })
	if !called || w.Code != 200 {
		t.Fatal("valid service call did not execute")
	}
}
