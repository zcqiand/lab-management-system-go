package service

// auth_service_test.go — 认证域 feature 测试（镜像 rails test/services/auth/auth_service_test
// 语义，REQ-2026-004 Task 5）。saas IdP 用 fake 注入（SaasAPI 缝），JWT/快照/目录用真实现。
//
// 功能 ID：见各测试 fn: 注解（trace 从测试函数名扫 // fn:）。M01.F05.I01 login 已
// 废弃——不挂 ID（禁给废弃/skip 测试挂功能 ID）。

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"strings"
	"testing"

	"lab-management-system-go/internal/impl/auth"
	"lab-management-system-go/internal/impl/errors"
	"lab-management-system-go/internal/impl/saas"
)

// ---- fake IdP ----

type fakeSaas struct {
	userLogin    func(ctx context.Context, u, p string) (*saas.LoginResponse, error)
	serviceLogin func(ctx context.Context, u, p string) (*saas.TokenResponse, error)
	token        func(ctx context.Context, req saas.TokenRequest) (*saas.TokenResponse, error)
	whoami       func(ctx context.Context, at string) (map[string]any, error)
	myTenants    []any
	menus        []any
	platform     []any

	serviceLoginCalls int
}

func (f *fakeSaas) UserLogin(ctx context.Context, u, p string) (*saas.LoginResponse, error) {
	return f.userLogin(ctx, u, p)
}
func (f *fakeSaas) ServiceLogin(ctx context.Context, u, p string) (*saas.TokenResponse, error) {
	f.serviceLoginCalls++
	return f.serviceLogin(ctx, u, p)
}
func (f *fakeSaas) Token(ctx context.Context, req saas.TokenRequest) (*saas.TokenResponse, error) {
	return f.token(ctx, req)
}
func (f *fakeSaas) Whoami(ctx context.Context, at string) (map[string]any, error) {
	return f.whoami(ctx, at)
}
func (f *fakeSaas) ListMyTenants(context.Context, string) ([]any, error) {
	return f.myTenants, nil
}
func (f *fakeSaas) ListMyMenus(context.Context, string, string) ([]any, error) {
	return f.menus, nil
}
func (f *fakeSaas) ListPlatformTenants(context.Context, string) ([]any, error) {
	return f.platform, nil
}

const fakeMenuTree = `[
 {"id":"m1","code":"samples","title":"样品管理","type":"page","sortOrder":1,
  "children":[{"id":"m2","code":"sample-receipts","title":"","type":"page","sortOrder":2}]}]`

func fakeWhoamiUser() map[string]any {
	return map[string]any{"id": "saas-1", "username": "alice", "email": "alice@x.uk",
		"displayName": "", "currentTenantId": "uuid-1"}
}

// okLoginFake 账密成功/whoami/code 交换全通的假 IdP。
func okLoginFake(t *testing.T) *fakeSaas {
	t.Helper()
	usedRefresh := map[string]bool{}
	return &fakeSaas{
		userLogin: func(context.Context, string, string) (*saas.LoginResponse, error) {
			return &saas.LoginResponse{
				User:             fakeWhoamiUser(),
				AvailableTenants: []any{map[string]any{"tenantId": "uuid-1", "status": "active", "roleIds": []any{"admin"}}},
				AccessToken:      "saas-at", RefreshToken: "saas-rt", UserID: "saas-1", CurrentTenantID: "uuid-1",
			}, nil
		},
		serviceLogin: func(context.Context, string, string) (*saas.TokenResponse, error) {
			return &saas.TokenResponse{AccessToken: "svc-at", RefreshToken: "svc-rt"}, nil
		},
		token: func(_ context.Context, req saas.TokenRequest) (*saas.TokenResponse, error) {
			// rotate-once 语义：saas 侧 refresh token 单次有效，重放即拒
			if req.GrantType == "refresh_token" {
				if req.RefreshToken == "" || usedRefresh[req.RefreshToken] {
					return nil, saas.ErrInvalidGrant{}
				}
				usedRefresh[req.RefreshToken] = true
				return &saas.TokenResponse{AccessToken: "saas-at2", RefreshToken: "saas-rt2"}, nil
			}
			if req.GrantType == "authorization_code" && req.Code == "code-1" {
				return &saas.TokenResponse{AccessToken: "saas-at2", RefreshToken: "saas-rt2"}, nil
			}
			return nil, saas.ErrInvalidGrant{}
		},
		whoami: func(context.Context, string) (map[string]any, error) { return fakeWhoamiUser(), nil },
		myTenants: []any{
			map[string]any{"tenantId": "uuid-1", "roleIds": []any{"admin"}},
		},
		menus:    mustJSONSlice(t, fakeMenuTree),
		platform: []any{map[string]any{"id": "uuid-1", "name": "市住建工程质量检测中心", "tenantKey": "city-lab"}},
	}
}

func setJWTEnv(t *testing.T) {
	t.Helper()
	t.Setenv("JWT_SIGNING_KEY", "test-signing-key-32-bytes-long!!")
	t.Setenv("JWT_ISSUER", "lab-management-system")
	t.Setenv("JWT_TTL_SECONDS", "3600")
	t.Setenv("JWT_REFRESH_TTL_SECONDS", "604800")
	// 密码登录路径经服务账号拉菜单快照（rails ENV.fetch 同键）
	t.Setenv("LAB_SAAS_SERVICE_USER", "alice")
	t.Setenv("LAB_SAAS_SERVICE_PASSWORD", "dev123456")
}

func newTestService(fake *fakeSaas) *AuthService {
	return NewAuthService(auth.NewDirectory(), auth.NewSnapshotCache(auth.SnapshotCacheTTL), auth.NewSnapshotCache(auth.SnapshotCacheTTL), fake)
}

func mustJSONSlice(t *testing.T, raw string) []any {
	t.Helper()
	var v any
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		t.Fatalf("测试夹具 JSON 非法: %v", err)
	}
	return v.([]any)
}

func asE(t *testing.T, err error) *errors.E {
	t.Helper()
	var e *errors.E
	if !stderrors.As(err, &e) {
		t.Fatalf("期望 *errors.E，得 %v", err)
	}
	return e
}

func str(m map[string]any, k string) string {
	s, _ := m[k].(string)
	return s
}

// ---- login / native-login（M01.F05.I01 已废弃，不挂 ID；native-login=M01.F05.I06） ----

// TestLoginProxyHappyPath 原生登录代理链全字段：token 重签 + tenant_key 富化 +
// 服务账号菜单快照 + membership 快照 + directory 落地。
// fn: M01.F05.I06
func TestLoginProxyHappyPath(t *testing.T) {
	setJWTEnv(t)
	fake := okLoginFake(t)
	svc := newTestService(fake)

	sess, err := svc.Login(context.Background(), "alice", "dev123456")
	if err != nil {
		t.Fatalf("登录不应错: %v", err)
	}
	// 会话四件：token/refreshToken/user/tenants
	token := str(sess, "token")
	if token == "" {
		t.Fatal("token 缺失")
	}
	claims, err := auth.Verify(token)
	if err != nil {
		t.Fatalf("token 应可验: %v", err)
	}
	if claims["sub"] != "saas-1" || claims["typ"] != auth.TypAccess {
		t.Fatalf("access claims 不符: %v", claims)
	}
	// tenant_id=tenant_key（2026-10-20 人裁：对外交互用 key）
	if claims["tenant_id"] != "city-lab" {
		t.Fatalf("tenant_id 应为富化 key city-lab: %v", claims["tenant_id"])
	}
	if _, hasAud := claims["aud"]; hasAud {
		t.Fatal("lab JWT 无 aud claim（家族有意差异）")
	}
	rt, err := auth.Verify(str(sess, "refreshToken"))
	if err != nil || rt["typ"] != auth.TypRefresh {
		t.Fatalf("refresh token 应可验且 typ=refresh: %v %v", err, rt)
	}
	// 密码登录路径 refresh 内嵌 dev-placeholder（rails session 第 4 参 nil 口径）
	if rt["saas_refresh_token"] != "dev-placeholder" {
		t.Fatalf("密码登录 refresh 应内嵌 dev-placeholder: %v", rt["saas_refresh_token"])
	}
	user := sess["user"].(map[string]any)
	// displayName 回退链：saas 空 displayName → username（家族记忆空串陷阱）
	if str(user, "id") != "saas-1" || str(user, "username") != "alice" || str(user, "displayName") != "alice" || str(user, "roleCode") != "admin" {
		t.Fatalf("user 形状不符: %v", user)
	}
	tenants := sess["tenants"].([]map[string]any)
	if len(tenants) != 1 || tenants[0]["tenant_id"] != "city-lab" || tenants[0]["name"] != "市住建工程质量检测中心" {
		t.Fatalf("tenants 应富化为 city-lab/真名: %v", tenants)
	}
	// 服务账号拉菜单快照（密码登录用户无 per-user saas refresh）
	if fake.serviceLoginCalls != 1 {
		t.Fatalf("菜单快照应经服务账号拉一次，得 %d", fake.serviceLoginCalls)
	}
	if snap, ok := svc.menuCache.Get("saas-1"); !ok || len(snap.([]map[string]any)) != 1 {
		t.Fatalf("菜单快照应已填充: %v %v", snap, ok)
	}
	// membership 快照已写
	if _, ok := svc.membership.Get("saas-1"); !ok {
		t.Fatal("membership 快照应已写")
	}
	// directory 已落（upsert 键=email 优先，ADR-0008；resolve_user 后续 me/switch 用）
	if u := svc.dir.FindByID("saas-1"); u == nil || u.Username != "alice@x.uk" {
		t.Fatalf("directory 应已 upsert（email 键）: %+v", u)
	}
}

// TestLoginWrongPassword saas 4xx → 401 INVALID_CREDENTIALS（不透传 saas 内部码）。
// fn: M01.F05.I06
func TestLoginWrongPassword(t *testing.T) {
	setJWTEnv(t)
	fake := okLoginFake(t)
	fake.userLogin = func(context.Context, string, string) (*saas.LoginResponse, error) {
		return nil, saas.ErrUnauthorizedClient{}
	}
	svc := newTestService(fake)
	_, err := svc.Login(context.Background(), "alice", "wrong")
	e := asE(t, err)
	if e.Status != 401 || e.Code != "INVALID_CREDENTIALS" {
		t.Fatalf("saas 401 应转 401 INVALID_CREDENTIALS: %+v", e)
	}
}

// TestLoginSaasDown saas 5xx/不可达 → 502 SAAS_UNAVAILABLE（代理链分叉）。
// fn: M01.F05.I06
func TestLoginSaasDown(t *testing.T) {
	setJWTEnv(t)
	fake := okLoginFake(t)
	fake.userLogin = func(context.Context, string, string) (*saas.LoginResponse, error) {
		return nil, saas.ErrUpstreamUnavailable{}
	}
	svc := newTestService(fake)
	_, err := svc.Login(context.Background(), "alice", "dev123456")
	e := asE(t, err)
	if e.Status != 502 || e.Code != "SAAS_UNAVAILABLE" {
		t.Fatalf("saas 不可达应 502 SAAS_UNAVAILABLE: %+v", e)
	}
}

// TestLoginMissingFields 账密缺一 → 400（rails ArgumentError 同源）。
// fn: M01.F05.I06
func TestLoginMissingFields(t *testing.T) {
	setJWTEnv(t)
	svc := newTestService(okLoginFake(t))
	_, err := svc.Login(context.Background(), "  ", "")
	e := asE(t, err)
	if e.Status != 400 {
		t.Fatalf("缺账密应 400: %+v", e)
	}
}

// TestAvailableTenantsFiltered removed/无 tenantId 的平台租户应被过滤。
// fn: M01.F05.I06
func TestAvailableTenantsFiltered(t *testing.T) {
	setJWTEnv(t)
	fake := okLoginFake(t)
	fake.userLogin = func(context.Context, string, string) (*saas.LoginResponse, error) {
		return &saas.LoginResponse{
			User: fakeWhoamiUser(),
			AvailableTenants: []any{
				map[string]any{"tenantId": "", "status": "active"},
				map[string]any{"tenantId": "uuid-2", "status": "removed"},
				map[string]any{"tenantId": "uuid-1", "status": "active", "roleIds": []any{"admin"}},
			},
			UserID: "saas-1", CurrentTenantID: "",
		}, nil
	}
	svc := newTestService(fake)
	sess, err := svc.Login(context.Background(), "alice", "dev123456")
	if err != nil {
		t.Fatalf("登录不应错: %v", err)
	}
	tenants := sess["tenants"].([]map[string]any)
	if len(tenants) != 1 || tenants[0]["tenant_id"] == "" {
		t.Fatalf("removed/空租户应被滤掉: %v", tenants)
	}
	// currentTenantId 缺省落首租户
	claims, err := auth.Verify(str(sess, "token"))
	if err != nil || claims["tenant_id"] == "" {
		t.Fatalf("current 缺省应落首租户: %v %v", claims, err)
	}
}

// ---- refresh（M01.F05.I04） ----

// TestRefreshRotateOnce refresh 续期成功 + 旧 token 重放被拒（saas 侧轮换语义）。
// fn: M01.F05.I04
func TestRefreshRotateOnce(t *testing.T) {
	setJWTEnv(t)
	fake := okLoginFake(t)
	svc := newTestService(fake)

	// 预置：SSO 水合态（有 per-user saas refresh token）
	old, err := svc.tokens.IssueRefresh("saas-1", "saas-rt")
	if err != nil {
		t.Fatalf("预置 refresh 签发失败: %v", err)
	}
	svc.dir.SetSaasRefreshToken("saas-1", "saas-rt")

	sess, err := svc.Refresh(context.Background(), old)
	if err != nil {
		t.Fatalf("refresh 不应错: %v", err)
	}
	newRT := str(sess, "refreshToken")
	if newRT == "" || newRT == old {
		t.Fatal("refresh 应换发新 token")
	}
	if got := svc.dir.GetSaasRefreshToken("saas-1"); got != "saas-rt2" {
		t.Fatalf("rotate-once：新 saas refresh 应落目录，得 %q", got)
	}
	// 新 access 的 tenant_id=原 claim 透传（refresh 路径不走 tenant_key 重解析）
	claims, err := auth.Verify(str(sess, "token"))
	if err != nil {
		t.Fatalf("新 access 应可验: %v", err)
	}
	if claims["typ"] != auth.TypAccess {
		t.Fatalf("typ 应 access: %v", claims)
	}
	// 重放旧 lab refresh token：内嵌旧 saas rt，saas 拒 → 401
	_, err = svc.Refresh(context.Background(), old)
	e := asE(t, err)
	if e.Status != 401 || e.Code != "INVALID_CREDENTIALS" {
		t.Fatalf("旧 refresh 重放应 401: %+v", e)
	}
}

// TestRefreshMissingClaim 无 saas_refresh_token claim → 401。
// fn: M01.F05.I04
func TestRefreshMissingClaim(t *testing.T) {
	setJWTEnv(t)
	svc := newTestService(okLoginFake(t))
	token, err := svc.tokens.IssueRefresh("u1", "")
	if err != nil {
		t.Fatalf("签发失败: %v", err)
	}
	_, err = svc.Refresh(context.Background(), token)
	if e := asE(t, err); e.Status != 401 {
		t.Fatalf("缺 saas claim 应 401: %+v", e)
	}
}

// TestRefreshGarbage 非法/过期 token → 401。
// fn: M01.F05.I04
func TestRefreshGarbage(t *testing.T) {
	setJWTEnv(t)
	svc := newTestService(okLoginFake(t))
	for _, bad := range []string{"", "not-a-jwt"} {
		if _, err := svc.Refresh(context.Background(), bad); err == nil {
			t.Fatalf("非法 refresh %q 应报错", bad)
		}
	}
}

// TestRefreshSaasDown saas 不可达在 refresh 链 → 502 SAAS_UPSTREAM_ERROR（与登录链
// SAAS_UNAVAILABLE 分叉，rails ApplicationController 双映射镜像）。
// fn: M01.F05.I04
func TestRefreshSaasDown(t *testing.T) {
	setJWTEnv(t)
	fake := okLoginFake(t)
	fake.token = func(context.Context, saas.TokenRequest) (*saas.TokenResponse, error) {
		return nil, saas.ErrUpstreamUnavailable{}
	}
	svc := newTestService(fake)
	token, _ := svc.tokens.IssueRefresh("saas-1", "saas-rt")
	_, err := svc.Refresh(context.Background(), token)
	if e := asE(t, err); e.Status != 502 || e.Code != "SAAS_UPSTREAM_ERROR" {
		t.Fatalf("refresh 链 saas 不可达应 502 SAAS_UPSTREAM_ERROR: %+v", e)
	}
}

// ---- me（M00.F01.I01） ----

// TestMeCachedHit 快照 hit 优先：tenants 同源登录、current=claim。
// fn: M00.F01.I01
func TestMeCachedHit(t *testing.T) {
	setJWTEnv(t)
	fake := okLoginFake(t)
	svc := newTestService(fake)
	if _, err := svc.Login(context.Background(), "alice", "dev123456"); err != nil {
		t.Fatalf("登录失败: %v", err)
	}
	out, err := svc.Me(map[string]any{"sub": "saas-1", "tenant_id": "city-lab"})
	if err != nil {
		t.Fatalf("me 不应错: %v", err)
	}
	if str(out, "currentTenantId") != "city-lab" {
		t.Fatalf("current 应取 claim: %v", out["currentTenantId"])
	}
	if len(out["tenants"].([]map[string]any)) != 1 {
		t.Fatalf("tenants 应同源快照: %v", out["tenants"])
	}
}

// TestMeEmptySnapshotIsHit 空快照也是 hit（tracker 5.111 家族裁定）：
// tenants=[] currentTenantId=claim，不得误判 miss 走降级。
// fn: M00.F01.I01
func TestMeEmptySnapshotIsHit(t *testing.T) {
	setJWTEnv(t)
	svc := newTestService(okLoginFake(t))
	svc.dir.Upsert("u1", "bob@x.uk", "Bob", "viewer")
	svc.membership.Put("u1", []map[string]any{})
	out, err := svc.Me(map[string]any{"sub": "u1", "tenant_id": "t9"})
	if err != nil {
		t.Fatalf("空快照 hit 不应错: %v", err)
	}
	if ts := out["tenants"].([]map[string]any); len(ts) != 0 {
		t.Fatalf("空快照应 tenants=[]: %v", ts)
	}
	if str(out, "currentTenantId") != "t9" {
		t.Fatalf("current 应=claim: %v", out["currentTenantId"])
	}
}

// TestMeCacheMissWithSaasRefresh 快照 miss + 持 saas refresh（SSO 用户）→ 401 refresh required。
// fn: M00.F01.I01
func TestMeCacheMissWithSaasRefresh(t *testing.T) {
	setJWTEnv(t)
	svc := newTestService(okLoginFake(t))
	svc.dir.Upsert("u1", "bob@x.uk", "Bob", "viewer")
	svc.dir.SetSaasRefreshToken("u1", "rt")
	_, err := svc.Me(map[string]any{"sub": "u1"})
	if e := asE(t, err); e.Status != 401 {
		t.Fatalf("SSO 用户快照 miss 应 401 refresh required: %+v", e)
	}
}

// TestMeDirectoryFallback 快照 miss + 无 saas refresh（密码登录用户）→ 目录降级。
// fn: M00.F01.I01
func TestMeDirectoryFallback(t *testing.T) {
	setJWTEnv(t)
	svc := newTestService(okLoginFake(t))
	svc.dir.Upsert("u1", "bob@x.uk", "Bob", "viewer")
	out, err := svc.Me(map[string]any{"sub": "u1"})
	if err != nil {
		t.Fatalf("目录降级不应错: %v", err)
	}
	if ts := out["tenants"].([]map[string]any); len(ts) != 3 {
		t.Fatalf("应降级目录三租户: %v", ts)
	}
	if str(out, "currentTenantId") != "TENANT-001" {
		t.Fatalf("无 claim 缺省目录默认租户: %v", out["currentTenantId"])
	}
}

// TestMeUnknownUser sub 不在目录 → 401。
// fn: M00.F01.I01
func TestMeUnknownUser(t *testing.T) {
	setJWTEnv(t)
	svc := newTestService(okLoginFake(t))
	_, err := svc.Me(map[string]any{"sub": "nobody"})
	if e := asE(t, err); e.Status != 401 {
		t.Fatalf("未知用户应 401: %+v", e)
	}
	_, err = svc.Me(map[string]any{})
	if e := asE(t, err); e.Status != 401 {
		t.Fatalf("缺 sub 应 401: %+v", e)
	}
}

// ---- switch-tenant（M00.F02.I01） ----

// TestSwitchTenant 命中快照换发 + 未知租户 404。
// fn: M00.F02.I01
func TestSwitchTenant(t *testing.T) {
	setJWTEnv(t)
	fake := okLoginFake(t)
	svc := newTestService(fake)
	if _, err := svc.Login(context.Background(), "alice", "dev123456"); err != nil {
		t.Fatalf("登录失败: %v", err)
	}
	// 登录租户集只有 city-lab；目录兜底三租户含 TENANT-002 → 快照优先（写死
	// demo 字面量会把 saas UUID 租户 404）
	_, err := svc.SwitchTenant(context.Background(), map[string]any{"sub": "saas-1"}, "uuid-9")
	e := asE(t, err)
	if e.Status != 404 {
		t.Fatalf("快照外租户应 404（快照优先于目录兜底）: %+v", e)
	}
	sess, err := svc.SwitchTenant(context.Background(), map[string]any{"sub": "saas-1"}, "city-lab")
	if err != nil {
		t.Fatalf("换发不应错: %v", err)
	}
	claims, err := auth.Verify(str(sess, "token"))
	if err != nil || claims["tenant_id"] != "city-lab" {
		t.Fatalf("换发后 tenant_id=目标: %v %v", claims, err)
	}
}

// ---- menus / permissions（M01.F04.I01/I02） ----

// TestMenusSnapshot 快照树形状（label=title 回退 code）+ miss → 503 MENUS_UNAVAILABLE。
// fn: M01.F04.I01
func TestMenusSnapshot(t *testing.T) {
	setJWTEnv(t)
	fake := okLoginFake(t)
	svc := newTestService(fake)
	_, err := svc.Menus(map[string]any{"sub": "saas-1"})
	if e := asE(t, err); e.Status != 503 || e.Code != "MENUS_UNAVAILABLE" {
		t.Fatalf("快照 miss 应 503 MENUS_UNAVAILABLE: %+v", e)
	}
	svc.menuCache.Put("saas-1", saas.NewMenuMapper().Map(mustJSONSlice(t, fakeMenuTree)))
	tree, err := svc.Menus(map[string]any{"sub": "saas-1"})
	if err != nil {
		t.Fatalf("快照 hit 不应错: %v", err)
	}
	nodes := tree.([]map[string]any)
	if len(nodes) != 1 || nodes[0]["label"] != "样品管理" || nodes[0]["id"] != "m1" {
		t.Fatalf("菜单树映射不符: %v", nodes)
	}
	kids := nodes[0]["children"].([]map[string]any)
	// 空 title 回退 code；type=page icon 兜底 file
	if kids[0]["label"] != "sample-receipts" || kids[0]["icon"] != "file" {
		t.Fatalf("子节点回退链不符: %v", kids[0])
	}
}

// TestPermissions msw 权限集 11 项。
// fn: M01.F04.I02
func TestPermissions(t *testing.T) {
	setJWTEnv(t)
	svc := newTestService(okLoginFake(t))
	out := svc.Permissions()
	perms := out["permissions"].([]string)
	if len(perms) != 11 {
		t.Fatalf("权限集应 11 项: %d", len(perms))
	}
	if perms[len(perms)-1] != "*" {
		t.Fatalf("末位通配: %v", perms)
	}
}

// ---- sso（M01.F05.I02/I03） ----

// TestSsoAuthorize 跳板 URL 拼 client_id/redirect_uri/state；缺 redirect_uri fail-fast。
// fn: M01.F05.I02
func TestSsoAuthorize(t *testing.T) {
	setJWTEnv(t)
	t.Setenv("LAB_SSO_LOGIN_URL", "https://saas.xiangru.uk/sso")
	t.Setenv("LAB_SAAS_CLIENT_ID", "lab-management")
	svc := newTestService(okLoginFake(t))
	out, err := svc.SsoAuthorize("https://lab.xiangru.uk/sso/cb", "st-1")
	if err != nil {
		t.Fatalf("sso authorize 不应错: %v", err)
	}
	url := str(out, "authorizeUrl")
	if !strings.HasPrefix(url, "https://saas.xiangru.uk/sso/login?redirect_uri=") ||
		!strings.Contains(url, "state=st-1") || !strings.Contains(url, "client_id=lab-management") {
		t.Fatalf("跳板 URL 形状不符: %s", url)
	}
	if str(out, "state") != "st-1" {
		t.Fatalf("state 透传: %v", out["state"])
	}
	if _, err := svc.SsoAuthorize("", "st-1"); err == nil {
		t.Fatal("缺 redirect_uri 应 fail-fast")
	}
}

// TestSsoAuthorizeMissingEnv LAB_SSO_LOGIN_URL 缺失 fail-fast（禁默认值兜底）。
// fn: M01.F05.I02
func TestSsoAuthorizeMissingEnv(t *testing.T) {
	setJWTEnv(t)
	t.Setenv("LAB_SSO_LOGIN_URL", "")
	t.Setenv("LAB_SAAS_CLIENT_ID", "lab-management")
	svc := newTestService(okLoginFake(t))
	if _, err := svc.SsoAuthorize("https://cb", "s"); err == nil {
		t.Fatal("缺 LAB_SSO_LOGIN_URL 应报错")
	}
}

// TestSsoCallback code 换 token → 水合会话；错误三分流镜像 rails。
// fn: M01.F05.I03
func TestSsoCallback(t *testing.T) {
	setJWTEnv(t)
	fake := okLoginFake(t)
	svc := newTestService(fake)
	sess, err := svc.SsoCallback(context.Background(), "code-1", "https://cb")
	if err != nil {
		t.Fatalf("sso callback 不应错: %v", err)
	}
	// 水合：directory 落地 + saas refresh 存回 + 菜单/membership 快照
	if u := svc.dir.FindByEmail("alice@x.uk"); u == nil {
		t.Fatal("SSO 用户应按 email 落目录")
	}
	if got := svc.dir.GetSaasRefreshToken("saas-1"); got != "saas-rt2" {
		t.Fatalf("saas refresh 应存回（rotate-once 载体）: %q", got)
	}
	if _, ok := svc.menuCache.Get("saas-1"); !ok {
		t.Fatal("菜单快照应已填")
	}
	// callback 路径 current 经 tenant_key 解析（whoami currentTenantId→key）
	claims, verr := auth.Verify(str(sess, "token"))
	if verr != nil || claims["tenant_id"] != "city-lab" {
		t.Fatalf("callback current 应解析为 tenant_key: %v %v", claims, verr)
	}
	rt, _ := auth.Verify(str(sess, "refreshToken"))
	if rt["saas_refresh_token"] != "saas-rt2" {
		t.Fatalf("SSO refresh 应内嵌 saas rt: %v", rt["saas_refresh_token"])
	}

	// InvalidGrant（code 重放/过期）→ 400 passthrough
	fake.token = func(context.Context, saas.TokenRequest) (*saas.TokenResponse, error) {
		return nil, saas.ErrInvalidGrant{}
	}
	_, err = svc.SsoCallback(context.Background(), "replayed", "https://cb")
	if e := asE(t, err); e.Status != 400 || e.Code != "INVALID_GRANT" {
		t.Fatalf("code 重放应 400 INVALID_GRANT: %+v", e)
	}

	// UnauthorizedClient → 401
	fake.token = func(context.Context, saas.TokenRequest) (*saas.TokenResponse, error) {
		return nil, saas.ErrUnauthorizedClient{}
	}
	_, err = svc.SsoCallback(context.Background(), "code-1", "https://cb")
	if e := asE(t, err); e.Status != 401 {
		t.Fatalf("UnauthorizedClient 应 401: %+v", e)
	}

	// UpstreamUnavailable → 502 SAAS_UPSTREAM_ERROR
	fake.token = func(context.Context, saas.TokenRequest) (*saas.TokenResponse, error) {
		return nil, saas.ErrUpstreamUnavailable{}
	}
	_, err = svc.SsoCallback(context.Background(), "code-1", "https://cb")
	if e := asE(t, err); e.Status != 502 || e.Code != "SAAS_UPSTREAM_ERROR" {
		t.Fatalf("sso 链 saas 不可达应 502 SAAS_UPSTREAM_ERROR: %+v", e)
	}
}

// TestSsoCallbackMissingCode 缺 body → 400（rails ArgumentError）。
// fn: M01.F05.I03
func TestSsoCallbackMissingCode(t *testing.T) {
	setJWTEnv(t)
	svc := newTestService(okLoginFake(t))
	_, err := svc.SsoCallback(context.Background(), "", "")
	if e := asE(t, err); e.Status != 400 {
		t.Fatalf("缺 code 应 400: %+v", e)
	}
}
