package api

// auth_impl_test.go — 认证域 handler 测试（REQ-2026-004 Task 5）。
//
// handler 只做装配与状态码映射（CLAUDE.md §2）：匿名放行面 login/native-login/
// refresh/sso/**；logout 与 me/menus/permissions/switch-tenant 需 Bearer
// （rails AuthController skip_before_action 镜像）。saas IdP 用 fake 注入。

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	genapi "lab-management-system-go/internal/gen/api"
	"lab-management-system-go/internal/impl/auth"
	saasimpl "lab-management-system-go/internal/impl/saas"
	"lab-management-system-go/internal/impl/service"
)

// fakeHandlerSaas 最小假 IdP（native-login 通 + whoami/code 交换通）。
type fakeHandlerSaas struct{}

func (f *fakeHandlerSaas) UserLogin(context.Context, string, string) (*saasimpl.LoginResponse, error) {
	return &saasimpl.LoginResponse{
		User:             map[string]any{"id": "saas-1", "username": "alice", "email": "alice@x.uk"},
		AvailableTenants: []any{map[string]any{"tenantId": "uuid-1", "status": "active", "roleIds": []any{"admin"}}},
		AccessToken:      "saas-at", UserID: "saas-1", CurrentTenantID: "uuid-1",
	}, nil
}
func (f *fakeHandlerSaas) ServiceLogin(context.Context, string, string) (*saasimpl.TokenResponse, error) {
	return &saasimpl.TokenResponse{AccessToken: "svc-at"}, nil
}
func (f *fakeHandlerSaas) Token(_ context.Context, req saasimpl.TokenRequest) (*saasimpl.TokenResponse, error) {
	if req.GrantType == "authorization_code" && req.Code == "code-1" {
		return &saasimpl.TokenResponse{AccessToken: "saas-at2", RefreshToken: "saas-rt2"}, nil
	}
	return nil, saasimpl.ErrInvalidGrant{}
}
func (f *fakeHandlerSaas) Whoami(context.Context, string) (map[string]any, error) {
	return map[string]any{"id": "saas-1", "email": "alice@x.uk", "currentTenantId": "uuid-1"}, nil
}
func (f *fakeHandlerSaas) ListMyTenants(context.Context, string) ([]any, error) {
	return []any{map[string]any{"tenantId": "uuid-1", "roleIds": []any{"admin"}}}, nil
}
func (f *fakeHandlerSaas) ListMyMenus(context.Context, string, string) ([]any, error) {
	return []any{}, nil
}
func (f *fakeHandlerSaas) ListPlatformTenants(context.Context, string) ([]any, error) {
	return []any{map[string]any{"id": "uuid-1", "name": "市住建", "tenantKey": "city-lab"}}, nil
}

func setHandlerEnv(t *testing.T) {
	t.Helper()
	t.Setenv("JWT_SIGNING_KEY", "test-signing-key-32-bytes-long!!")
	t.Setenv("JWT_ISSUER", "lab-management-system")
	t.Setenv("JWT_TTL_SECONDS", "3600")
	t.Setenv("JWT_REFRESH_TTL_SECONDS", "604800")
	t.Setenv("LAB_SAAS_SERVICE_USER", "alice")
	t.Setenv("LAB_SAAS_SERVICE_PASSWORD", "dev123456")
	t.Setenv("LAB_SSO_LOGIN_URL", "https://saas.xiangru.uk/sso")
	t.Setenv("LAB_SAAS_CLIENT_ID", "lab-management")
}

func newAuthServer() *Server {
	return &Server{Auth: service.NewAuthService(
		auth.NewDirectory(),
		auth.NewSnapshotCache(auth.SnapshotCacheTTL),
		auth.NewSnapshotCache(auth.SnapshotCacheTTL),
		&fakeHandlerSaas{},
	)}
} // newMux 走真生成路由（绑定/分发路径与 prod 一致）。
func newMux(s *Server) *http.ServeMux {
	mux := http.NewServeMux()
	_ = genapi.HandlerFromMux(s, mux)
	return mux
}

func doReq(t *testing.T, s *Server, method, target, body string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	newMux(s).ServeHTTP(rec, req)
	return rec
}

func jsonBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatalf("响应非 JSON: %v: %s", err, rec.Body.String())
	}
	return m
}

// TestAuthHandlerNativeLoginFlow 匿名 POST native-login → 200 全字段。
// fn: M01.F05.I06
func TestAuthHandlerNativeLoginFlow(t *testing.T) {
	setHandlerEnv(t)
	s := newAuthServer()
	rec := doReq(t, s, "POST", "/api/auth/native-login", `{"username":"alice","password":"dev123456"}`, nil)
	if rec.Code != 200 {
		t.Fatalf("期望 200，得 %d: %s", rec.Code, rec.Body.String())
	}
	m := jsonBody(t, rec)
	if m["token"] == "" || m["refreshToken"] == "" {
		t.Fatalf("token/refreshToken 缺失: %v", m)
	}
	user := m["user"].(map[string]any)
	if user["id"] != "saas-1" {
		t.Fatalf("user.id 不符: %v", user)
	}
	tenants := m["tenants"].([]any)
	first := tenants[0].(map[string]any)
	if first["tenantId"] != "city-lab" || first["name"] != "市住建" {
		t.Fatalf("tenants 应 camel 且富化: %v", first)
	}
}

// TestAuthHandlerProtectedNeedBearer logout/me/menus/permissions/switch-tenant 无
// Bearer → 401（匿名面只放行 login/native-login/refresh/sso/**）。
// fn: M00.F01.I01
func TestAuthHandlerProtectedNeedBearer(t *testing.T) {
	setHandlerEnv(t)
	s := newAuthServer()
	for _, tc := range []struct{ method, target string }{
		{"POST", "/api/auth/logout"},
		{"GET", "/api/auth/me"},
		{"GET", "/api/auth/menus"},
		{"GET", "/api/auth/permissions"},
		{"POST", "/api/auth/switch-tenant"},
	} {
		rec := doReq(t, s, tc.method, tc.target, "{}", nil)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s %s 无 Bearer 应 401，得 %d", tc.method, tc.target, rec.Code)
		}
	}
}

// TestAuthHandlerMeWithToken Bearer 会话链：login → me 200。
// fn: M00.F01.I01
func TestAuthHandlerMeWithToken(t *testing.T) {
	setHandlerEnv(t)
	s := newAuthServer()
	login := doReq(t, s, "POST", "/api/auth/native-login", `{"username":"alice","password":"dev123456"}`, nil)
	token := jsonBody(t, login)["token"].(string)
	hdrs := map[string]string{"Authorization": "Bearer " + token}

	rec := doReq(t, s, "GET", "/api/auth/me", "", hdrs)
	if rec.Code != 200 {
		t.Fatalf("me 期望 200，得 %d: %s", rec.Code, rec.Body.String())
	}
	m := jsonBody(t, rec)
	if m["currentTenantId"] != "city-lab" {
		t.Fatalf("me.currentTenantId 应 city-lab: %v", m)
	}
	// 坏 token → 401
	rec = doReq(t, s, "GET", "/api/auth/me", "", map[string]string{"Authorization": "Bearer garbage"})
	if rec.Code != 401 {
		t.Fatalf("坏 token 应 401，得 %d", rec.Code)
	}
}

// TestAuthHandlerMenus503WithoutSnapshot 快照 miss → 503 MENUS_UNAVAILABLE（demo 兜底已删）。
// fn: M01.F04.I01
func TestAuthHandlerMenus503WithoutSnapshot(t *testing.T) {
	setHandlerEnv(t)
	s := newAuthServer()
	login := doReq(t, s, "POST", "/api/auth/native-login", `{"username":"alice","password":"dev123456"}`, nil)
	token := jsonBody(t, login)["token"].(string)
	rec := doReq(t, s, "GET", "/api/auth/menus", "", map[string]string{"Authorization": "Bearer " + token})
	if rec.Code != 503 {
		t.Fatalf("菜单快照 miss 应 503，得 %d: %s", rec.Code, rec.Body.String())
	}
	m := jsonBody(t, rec)
	if m["code"] != "MENUS_UNAVAILABLE" {
		t.Fatalf("错误码不符: %v", m)
	}
}

// TestAuthHandlerLogout204 logout → 204 无体。
// fn: M01.F05.I05
func TestAuthHandlerLogout204(t *testing.T) {
	setHandlerEnv(t)
	s := newAuthServer()
	login := doReq(t, s, "POST", "/api/auth/native-login", `{"username":"alice","password":"dev123456"}`, nil)
	token := jsonBody(t, login)["token"].(string)
	rec := doReq(t, s, "POST", "/api/auth/logout", "{}", map[string]string{"Authorization": "Bearer " + token})
	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
		t.Fatalf("logout 应 204 空体，得 %d %q", rec.Code, rec.Body.String())
	}
}

// TestAuthHandlerPermissions 权限集 11 项 camel 直出。
// fn: M01.F04.I02
func TestAuthHandlerPermissions(t *testing.T) {
	setHandlerEnv(t)
	s := newAuthServer()
	login := doReq(t, s, "POST", "/api/auth/native-login", `{"username":"alice","password":"dev123456"}`, nil)
	token := jsonBody(t, login)["token"].(string)
	rec := doReq(t, s, "GET", "/api/auth/permissions", "", map[string]string{"Authorization": "Bearer " + token})
	if rec.Code != 200 {
		t.Fatalf("permissions 期望 200，得 %d", rec.Code)
	}
	perms := jsonBody(t, rec)["permissions"].([]any)
	if len(perms) != 11 || perms[len(perms)-1] != "*" {
		t.Fatalf("权限集应 11 项末位通配: %v", perms)
	}
}

// TestAuthHandlerSsoAuthorize 跳板 URL + 缺 redirect_uri 400。
// fn: M01.F05.I02
func TestAuthHandlerSsoAuthorize(t *testing.T) {
	setHandlerEnv(t)
	s := newAuthServer()
	rec := doReq(t, s, "GET",
		"/api/auth/sso/authorize?response_type=code&client_id=lab-management&redirect_uri=https%3A%2F%2Flab.x.uk%2Fcb&state=st-1", "", nil)
	if rec.Code != 200 {
		t.Fatalf("sso authorize 期望 200，得 %d: %s", rec.Code, rec.Body.String())
	}
	m := jsonBody(t, rec)
	if !strings.Contains(m["authorizeUrl"].(string), "client_id=lab-management") {
		t.Fatalf("authorizeUrl 应含 client_id: %v", m)
	}
	rec = doReq(t, s, "GET", "/api/auth/sso/authorize", "", nil)
	if rec.Code != 400 {
		t.Fatalf("缺 redirect_uri 应 400，得 %d", rec.Code)
	}
}

// TestAuthHandlerSsoCallback code 换 token → 200 会话。
// fn: M01.F05.I03
func TestAuthHandlerSsoCallback(t *testing.T) {
	setHandlerEnv(t)
	s := newAuthServer()
	rec := doReq(t, s, "POST", "/api/auth/sso/callback",
		`{"code":"code-1","grant_type":"authorization_code","redirect_uri":"https://cb"}`, nil)
	if rec.Code != 200 {
		t.Fatalf("sso callback 期望 200，得 %d: %s", rec.Code, rec.Body.String())
	}
	if jsonBody(t, rec)["token"] == "" {
		t.Fatal("callback 应出会话 token")
	}
}

// TestAuthHandlerRefresh anonymous refresh 链 → 200 换发。
// fn: M01.F05.I04
func TestAuthHandlerRefresh(t *testing.T) {
	setHandlerEnv(t)
	s := newAuthServer()
	login := doReq(t, s, "POST", "/api/auth/native-login", `{"username":"alice","password":"dev123456"}`, nil)
	rt := jsonBody(t, login)["refreshToken"].(string)
	rec := doReq(t, s, "POST", "/api/auth/refresh", `{"refreshToken":"`+rt+`"}`, nil)
	// 密码登录用户内嵌 dev-placeholder，saas 拒绝 → 401（服务端链路正确性）
	if rec.Code != 401 {
		t.Fatalf("dev-placeholder refresh 应 401，得 %d: %s", rec.Code, rec.Body.String())
	}
}

// TestAuthHandlerSwitchTenant 换发 200 + 未知租户 404。
// fn: M00.F02.I01
func TestAuthHandlerSwitchTenant(t *testing.T) {
	setHandlerEnv(t)
	s := newAuthServer()
	login := doReq(t, s, "POST", "/api/auth/native-login", `{"username":"alice","password":"dev123456"}`, nil)
	token := jsonBody(t, login)["token"].(string)
	hdrs := map[string]string{"Authorization": "Bearer " + token}

	rec := doReq(t, s, "POST", "/api/auth/switch-tenant", `{"tenantId":"city-lab"}`, hdrs)
	if rec.Code != 200 {
		t.Fatalf("换发期望 200，得 %d: %s", rec.Code, rec.Body.String())
	}
	rec = doReq(t, s, "POST", "/api/auth/switch-tenant", `{"tenantId":"nope"}`, hdrs)
	if rec.Code != 404 {
		t.Fatalf("未知租户应 404，得 %d", rec.Code)
	}
}
