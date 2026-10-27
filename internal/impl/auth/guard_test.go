package auth

// guard_test.go — Bearer 守卫（镜像 rails lab jwt_guard concern，REQ-2026-004 lab 分叉）：
// 无/坏 Bearer → 401 + WWW-Authenticate: Bearer realm="api"；
// tenant_id claim 缺省回退 directory 默认租户 TENANT-001（代码常量，
// 禁 LAB_SAAS_DEFAULT_TENANT_ID env 兜底——rails jwt_guard L38 明令）；claims 进 context。

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGuardMissingBearer401(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/auth/me", nil)
	Guard(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("无 Bearer 不应到 handler")
	}).ServeHTTP(rec, req)
	if rec.Code != 401 {
		t.Fatalf("期望 401，得到 %d", rec.Code)
	}
	if wa := rec.Header().Get("WWW-Authenticate"); wa != `Bearer realm="api"` {
		t.Fatalf("WWW-Authenticate 不符: %q", wa)
	}
}

func TestGuardBadToken401(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/auth/me", nil)
	req.Header.Set("Authorization", "Bearer not-a-jwt")
	Guard(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("坏 token 不应到 handler")
	}).ServeHTTP(rec, req)
	if rec.Code != 401 {
		t.Fatalf("期望 401，得到 %d", rec.Code)
	}
}

func TestGuardPassesClaimsViaContext(t *testing.T) {
	withJWTEnv(t)
	tok, err := Encode(map[string]any{"sub": "u1", "tenant_id": "TENANT-001"})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	reached := false
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	Guard(func(w http.ResponseWriter, r *http.Request) {
		reached = true
		claims, ok := ClaimsFrom(r.Context())
		if !ok || claims["sub"] != "u1" || claims["tenant_id"] != "TENANT-001" {
			t.Fatalf("claims 未进 context 或值不符: %v", claims)
		}
	}).ServeHTTP(rec, req)
	if !reached || rec.Code != 200 {
		t.Fatalf("合法 token 应放行: reached=%v code=%d", reached, rec.Code)
	}
}

// TestGuardTenantFallbackToDirectoryDefault：无 tenant_id claim 回退 TENANT-001。
func TestGuardTenantFallbackToDirectoryDefault(t *testing.T) {
	withJWTEnv(t)
	tok, _ := Encode(map[string]any{"sub": "u1"})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	Guard(func(w http.ResponseWriter, r *http.Request) {
		claims, _ := ClaimsFrom(r.Context())
		if claims["tenant_id"] != "TENANT-001" {
			t.Fatalf("缺 tenant_id 应回退 directory 默认租户 TENANT-001: %v", claims)
		}
	}).ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("回退后应放行，得 %d: %s", rec.Code, rec.Body.String())
	}
}

func TestClaimsFromEmptyContext(t *testing.T) {
	if _, ok := ClaimsFrom(context.Background()); ok {
		t.Fatal("空 context 不应有 claims")
	}
}
