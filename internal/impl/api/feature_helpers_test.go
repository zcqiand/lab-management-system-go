package api

// feature_helpers_test.go — Feature 测试共享助手（域 Feature 测试共用，Task 5 起）。

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// doJSON 结构体 body 助手（marshal 后走 doReq 同一条生成路由分发）。
func doJSON(t *testing.T, s *Server, method, target string, body any, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("body marshal: %v", err)
		}
		reader = bytes.NewReader(raw)
	} else {
		reader = strings.NewReader("")
	}
	req := httptest.NewRequest(method, target, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	newMux(s).ServeHTTP(rec, req)
	return rec
}

var _ = http.MethodGet
