// Package errors — 错误形状映射（镜像 rails render_error / ktor StatusPages 语义）：
// 错误体统一 {code,message}；423=空体；未知错误→500 INTERNAL 不泄露细节。
// service 层只构造本包的类型，handler/中间件只经 Write 落响应。
package errors

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// E 家族错误：Status/Code/Message 三元。Locked 走空体特例。
type E struct {
	Status  int
	Code    string
	Message string
}

func (e *E) Error() string { return fmt.Sprintf("%s: %s", e.Code, e.Message) }

// As 还原 *E（errors.As 薄包装，handler 判型用）。
func As(err error, target **E) bool {
	// 直接判型足够——service 层不 wrap 本类型；保留函数形态便于测试与未来扩展
	e, ok := err.(*E)
	if ok && target != nil {
		*target = e
	}
	return ok
}

func ErrInvalidCredentials() *E {
	return &E{Status: http.StatusUnauthorized, Code: "INVALID_CREDENTIALS", Message: "invalid credentials"}
}

func ErrForbidden(msg string) *E {
	return &E{Status: http.StatusForbidden, Code: "FORBIDDEN", Message: msg}
}

func ErrNotFound(msg string) *E {
	return &E{Status: http.StatusNotFound, Code: "NOT_FOUND", Message: msg}
}

func ErrBadRequest(msg string) *E {
	return &E{Status: http.StatusBadRequest, Code: "BAD_REQUEST", Message: msg}
}

// ErrLocked 账户锁定：423 空体（镜像 rails head :locked / springboot body(null)）。
func ErrLocked() *E {
	return &E{Status: http.StatusLocked, Code: "LOCKED", Message: ""}
}

// Write 把 err 落为 HTTP 响应：*E 按其形状；未知错误 500 INTERNAL 不泄露细节。
func Write(w http.ResponseWriter, err error) {
	var e *E
	if !As(err, &e) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"code": "INTERNAL", "message": "internal error"})
		return
	}
	if e.Status == http.StatusLocked {
		w.WriteHeader(e.Status) // 空体
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(e.Status)
	_ = json.NewEncoder(w).Encode(map[string]string{"code": e.Code, "message": e.Message})
}

// ErrSaasUnavailable saas IdP 不可达/5xx：502 SAAS_UNAVAILABLE（lab 代理链分叉，
// REQ-2026-023 镜像——4xx 才转 401 INVALID_CREDENTIALS，5xx/连不上不透传）。
func ErrSaasUnavailable() *E {
	return &E{Status: http.StatusBadGateway, Code: "SAAS_UNAVAILABLE", Message: "saas idp unavailable"}
}

// ErrMenusUnavailable 菜单快照 miss（重启/过期/未填充）→ 503 MENUS_UNAVAILABLE，
// demo 兜底已删（2026-08-27），前端回退静态菜单（rails ApplicationController 镜像）。
func ErrMenusUnavailable() *E {
	return &E{Status: http.StatusServiceUnavailable, Code: "MENUS_UNAVAILABLE", Message: "menu snapshot unavailable; re-login to refresh"}
}

// ErrInvalidGrant saas OAuth code 重放/过期 → 400 passthrough（lab-msw 契约对齐）。
func ErrInvalidGrant(msg string) *E {
	return &E{Status: http.StatusBadRequest, Code: "INVALID_GRANT", Message: msg}
}

// ErrSaasUpstream refresh/sso 链 saas 不可达 → 502 SAAS_UPSTREAM_ERROR
// （rails ApplicationController 双映射：登录链走 ErrSaasUnavailable，此二链专属）。
func ErrSaasUpstream() *E {
	return &E{Status: http.StatusBadGateway, Code: "SAAS_UPSTREAM_ERROR", Message: "saas upstream unavailable"}
}
