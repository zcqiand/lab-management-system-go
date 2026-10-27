// Package service — 业务域服务层（镜像 rails app/services，REQ-2026-004 Task 5 起）。
//
// handler 只做装配与状态码映射（仓 CLAUDE.md 铁律），业务在本包。
package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"

	"lab-management-system-go/internal/impl/auth"
	familyerrors "lab-management-system-go/internal/impl/errors"
	"lab-management-system-go/internal/impl/saas"
)

// auth_service.go — 认证域服务（rails Auth::AuthService 1:1 镜像，ADR-0008）：
//   - JWT：auth.TokenService 出真签名（HS256，无 aud 家族差异）
//   - SSO：saas IdP 真调 /oauth/token；/me 系拿快照
//   - CSRF：state 前端生成后端透传（RFC 6749 §10.12），后端不校验
//   - refresh：lab refresh token 内嵌 saas refresh token（rotate-once）
//
// 菜单/租户快照：进程内 30min TTL（auth.SnapshotCache），SSO/refresh/密码登录时点填充。

// DemoPermissions msw 权限集（admin 全量 11 项，handlers-extra.ts 镜像）。
var DemoPermissions = []string{
	"contract:read", "contract:write", "sample:read", "sample:write",
	"report:read", "report:write", "report:issue", "inspection:read",
	"inspection:write", "audit:read", "*",
}

// LabAppCode lab 家族在 saas 注册的 appCode（seeds apps.json）。
const LabAppCode = "lab-management"

// SaasAPI saas IdP 出站端口（*saas.HTTPClient 满足；测试注入 fake 用缝）。
type SaasAPI interface {
	UserLogin(ctx context.Context, username, password string) (*saas.LoginResponse, error)
	ServiceLogin(ctx context.Context, username, password string) (*saas.TokenResponse, error)
	Token(ctx context.Context, req saas.TokenRequest) (*saas.TokenResponse, error)
	Whoami(ctx context.Context, accessToken string) (map[string]any, error)
	ListMyTenants(ctx context.Context, accessToken string) ([]any, error)
	ListMyMenus(ctx context.Context, accessToken, appCode string) ([]any, error)
	ListPlatformTenants(ctx context.Context, accessToken string) ([]any, error)
}

var _ SaasAPI = (*saas.HTTPClient)(nil)

// AuthService 认证域服务。目录与快照缓存跨请求存活（进程内单例语义由装配层保证）。
type AuthService struct {
	dir        *auth.Directory
	menuCache  *auth.SnapshotCache
	membership *auth.SnapshotCache
	tokens     auth.TokenService
	saas       SaasAPI
	mapper     *saas.MenuMapper
}

// NewAuthService 构造（DI 缝：saas 出站端口可注入 fake）。
func NewAuthService(dir *auth.Directory, menuCache, membership *auth.SnapshotCache, saasAPI SaasAPI) *AuthService {
	return &AuthService{
		dir:        dir,
		menuCache:  menuCache,
		membership: membership,
		tokens:     auth.TokenService{},
		saas:       saasAPI,
		mapper:     saas.NewMenuMapper(),
	}
}

// Login 密码登录（native-login 同口径，M01.F05.I06）：账密校验代理 saas IdP
// （REQ-2026-023，凭据真源=saas 库）。错误映射：saas 4xx → 401 INVALID_CREDENTIALS
// （不透传）；5xx/不可达 → 502 SAAS_UNAVAILABLE。
func (s *AuthService) Login(ctx context.Context, username, password string) (map[string]any, error) {
	username = trimSpace(username)
	if username == "" || password == "" {
		return nil, familyerrors.ErrBadRequest("username and password are required")
	}
	saasLogin, err := s.saas.UserLogin(ctx, username, password)
	if err != nil {
		switch {
		case errors.Is(err, saas.ErrUnauthorizedClient{}), errors.Is(err, saas.ErrInvalidGrant{}):
			return nil, familyerrors.ErrInvalidCredentials()
		case errors.Is(err, saas.ErrUpstreamUnavailable{}):
			return nil, familyerrors.ErrSaasUnavailable()
		default:
			return nil, err
		}
	}
	saasUser := orEmptyMap(saasLogin.User)
	// 平台租户名富化 hoist：tenants 映射与会话 tenant_key 解析同源（2026-10-20 人裁）
	nameBy := s.fetchTenantNames(ctx, firstPresence(saasLogin.UserID, strVal(saasUser["id"])), saasLogin.AccessToken)
	tenants := s.tenantsFrom(activeAvailableTenants(saasLogin.AvailableTenants), nameBy)
	user := loginUser(saasLogin, saasUser, tenants)
	// 密码登录用户无 per-user saas refresh → 服务账号拉菜单快照（失败不阻塞登录）
	s.cacheMenusWithServiceAccount(ctx, user.ID)
	s.membership.Put(user.ID, tenants)
	// 目录落地（email 缺失降级 username，ADR-0008）——me/switch 的 resolve_user 依赖
	s.dir.Upsert(user.ID, firstPresence(strVal(saasUser["email"]), user.Username), user.DisplayName, user.RoleCode)
	// lab 自家 JWT 重签（tenant_id=saas currentTenantId ?? 首租户 → tenant_key）；
	// refresh 维持 dev-placeholder（不透传 saas refreshToken）
	current := saasLogin.CurrentTenantID
	if current == "" && len(tenants) > 0 {
		current = tenants[0]["tenant_id"].(string)
	}
	return s.session(user, resolveTenantKey(current, nameBy), tenants, "")
}

// Refresh 刷新 token（M01.F05.I04）：lab refresh 验签 → 内嵌 saas rt 续 → 水合会话。
// saas 4xx → 401；saas 不可达 → 502 SAAS_UPSTREAM_ERROR（与登录链 SAAS_UNAVAILABLE 分叉）。
func (s *AuthService) Refresh(ctx context.Context, refreshToken string) (map[string]any, error) {
	if refreshToken == "" {
		return nil, familyerrors.ErrInvalidCredentials()
	}
	claims, err := s.tokens.VerifyTyp(refreshToken, auth.TypRefresh)
	if err != nil {
		return nil, familyerrors.ErrInvalidCredentials()
	}
	saasRefresh := strVal(claims["saas_refresh_token"])
	if saasRefresh == "" {
		return nil, familyerrors.ErrInvalidCredentials()
	}
	t, err := s.saas.Token(ctx, saas.TokenRequest{GrantType: "refresh_token", RefreshToken: saasRefresh})
	if err != nil {
		switch {
		case errors.Is(err, saas.ErrUnauthorizedClient{}), errors.Is(err, saas.ErrInvalidGrant{}):
			return nil, familyerrors.ErrInvalidCredentials()
		case errors.Is(err, saas.ErrUpstreamUnavailable{}):
			return nil, familyerrors.ErrSaasUpstream()
		default:
			return nil, err
		}
	}
	// refresh 路径 claim 原样透传（claim 已是签发时点的 key）；水合阶段 saas 错误
	// 与 Token 阶段同款分流（final review Important#3：sentinel 原样上抛会落 500）
	out, err := s.hydrateAndSession(ctx, strVal(claims["tenant_id"]), t, false)
	if err != nil {
		return nil, s.mapSaasErr(err, familyerrors.ErrInvalidCredentials())
	}
	return out, nil
}

// mapSaasErr saas sentinel → 家族错误形状（InvalidGrant/UnauthorizedClient→401、
// UpstreamUnavailable→502、其余原样——与 Token 调用点 switch 同形）。
func (s *AuthService) mapSaasErr(err error, credErr error) error {
	switch {
	case errors.Is(err, saas.ErrUnauthorizedClient{}), errors.Is(err, saas.ErrInvalidGrant{}):
		return credErr
	case errors.Is(err, saas.ErrUpstreamUnavailable{}):
		return familyerrors.ErrSaasUpstream()
	default:
		return err
	}
}

// Logout 登出（无状态 JWT，服务端无 session store）。
func (s *AuthService) Logout() {}

// Me 当前会话（M00.F01.I01）：快照 hit 优先（空快照也是 hit，tracker 5.111）；
// miss 且持 saas refresh（SSO 用户）→ 401 refresh required；miss 且无 refresh →
// demo 目录降级。
func (s *AuthService) Me(claims map[string]any) (map[string]any, error) {
	user, err := s.resolveUser(claims)
	if err != nil {
		return nil, err
	}
	if cached, ok := s.membership.Get(user.ID); ok {
		tenants := cached.([]map[string]any)
		current := firstPresence(strVal(claims["tenant_id"]), firstTenantID(tenants))
		return meStruct(user, tenants, current), nil
	}
	if s.dir.GetSaasRefreshToken(user.ID) != "" {
		return nil, familyerrors.ErrInvalidCredentials()
	}
	current := firstPresence(strVal(claims["tenant_id"]), s.dir.DefaultTenant().TenantID)
	return meStruct(user, directoryTenants(s.dir, user), current), nil
}

// SwitchTenant 选租户换发（M00.F02.I01）：归属=当前会话租户集（membership 快照，
// login/sso/refresh 写入），demo 目录只作快照 miss 兜底——写死 demo 字面量会把
// saas UUID 租户 404。
func (s *AuthService) SwitchTenant(ctx context.Context, claims map[string]any, tenantID string) (map[string]any, error) {
	user, err := s.resolveUser(claims)
	if err != nil {
		return nil, err
	}
	known := directoryTenants(s.dir, user)
	if cached, ok := s.membership.Get(user.ID); ok {
		if ts := cached.([]map[string]any); len(ts) > 0 {
			known = ts
		}
	}
	for _, t := range known {
		if t["tenant_id"].(string) == tenantID {
			return s.session(user, tenantID, known, "")
		}
	}
	return nil, familyerrors.ErrNotFound("Tenant not found")
}

// Menus 动态菜单（M01.F04.I01）：miss/空 → 503 MENUS_UNAVAILABLE（demo 兜底已删，
// 前端回退静态菜单）。
func (s *AuthService) Menus(claims map[string]any) (any, error) {
	sub := strVal(claims["sub"])
	v, ok := s.menuCache.Get(sub)
	if ok {
		if snapshot, ok2 := v.([]map[string]any); ok2 && len(snapshot) > 0 {
			return snapshot, nil
		}
	}
	return nil, familyerrors.ErrMenusUnavailable()
}

// Permissions 权限集（M01.F04.I02，msw admin 全量）。
func (s *AuthService) Permissions() map[string]any {
	return map[string]any{"permissions": DemoPermissions}
}

// SsoAuthorize SSO 跳板（M01.F05.I02）：直接回 saas 登录页 URL（带
// redirect_uri+state+client_id）；redirect_uri 缺失 fail-fast（ADR-0019）。
func (s *AuthService) SsoAuthorize(businessRedirect, frontendState string) (map[string]any, error) {
	if businessRedirect == "" {
		return nil, familyerrors.ErrBadRequest("missing redirect_uri")
	}
	loginURL := os.Getenv("LAB_SSO_LOGIN_URL")
	clientID := os.Getenv("LAB_SAAS_CLIENT_ID")
	if loginURL == "" || clientID == "" {
		return nil, fmt.Errorf("缺少环境变量 LAB_SSO_LOGIN_URL / LAB_SAAS_CLIENT_ID（禁默认值兜底）")
	}
	authorizeURL := loginURL + "/login" +
		"?redirect_uri=" + url.QueryEscape(businessRedirect) +
		"&state=" + url.QueryEscape(frontendState) +
		"&client_id=" + url.QueryEscape(clientID)
	return map[string]any{"authorizeUrl": authorizeURL, "state": frontendState}, nil
}

// SsoCallback SSO 回调（M01.F05.I03）：code 换 token → 水合会话。错误三分流镜像
// rails：InvalidGrant → 400 passthrough；UnauthorizedClient → 401；不可达 → 502。
func (s *AuthService) SsoCallback(ctx context.Context, code, redirectURI string) (map[string]any, error) {
	if code == "" {
		return nil, familyerrors.ErrBadRequest("missing code")
	}
	t, err := s.saas.Token(ctx, saas.TokenRequest{GrantType: "authorization_code", Code: code, RedirectURI: redirectURI})
	if err != nil {
		switch {
		case errors.Is(err, saas.ErrInvalidGrant{}):
			return nil, familyerrors.ErrInvalidGrant("saas invalid grant")
		case errors.Is(err, saas.ErrUnauthorizedClient{}):
			return nil, familyerrors.ErrInvalidCredentials()
		case errors.Is(err, saas.ErrUpstreamUnavailable{}):
			return nil, familyerrors.ErrSaasUpstream()
		default:
			return nil, err
		}
	}
	// callback 路径 current 经 tenant_key 解析（2026-10-20 人裁）；水合阶段 saas
	// 错误按本链三分流（InvalidGrant→400 passthrough、UnauthorizedClient→401、不可达→502）
	out, err := s.hydrateAndSession(ctx, "", t, true)
	if err != nil {
		if errors.Is(err, saas.ErrInvalidGrant{}) {
			return nil, familyerrors.ErrInvalidGrant("saas invalid grant")
		}
		return nil, s.mapSaasErr(err, familyerrors.ErrInvalidCredentials())
	}
	return out, nil
}

// ---- 内部：水合 / 映射 / 会话（rails private 段镜像） ----

// hydrateAndSession saas token 到手后的公共水合：whoami → 目录 upsert → 菜单快照
// → refresh token 存回（rotate-once）→ 平台租户名富化 → memberships 快照 → 签发。
// saas 错误原样上抛，由调用方按链路分流（refresh→401/502、callback→400/401/502）。
func (s *AuthService) hydrateAndSession(ctx context.Context, tenantID string, t *saas.TokenResponse, callback bool) (map[string]any, error) {
	saasUser, err := s.saas.Whoami(ctx, t.AccessToken)
	if err != nil {
		return nil, err
	}
	email := strVal(saasUser["email"])
	labUser := s.dir.FindByEmail(email)
	if labUser == nil {
		labUser = s.dir.Upsert(strVal(saasUser["id"]), email, strVal(saasUser["displayName"]), "viewer")
	}
	s.cacheMenus(ctx, labUser.ID, t.AccessToken)
	s.dir.SetSaasRefreshToken(labUser.ID, t.RefreshToken)
	nameBy := s.fetchTenantNames(ctx, labUser.ID, t.AccessToken)
	memberships, err := s.saas.ListMyTenants(ctx, t.AccessToken)
	if err != nil {
		return nil, err
	}
	tenants := s.tenantsFrom(memberships, nameBy)
	s.membership.Put(labUser.ID, tenants)
	current := tenantID
	if callback {
		current = resolveTenantKey(strVal(saasUser["currentTenantId"]), nameBy)
	}
	return s.session(labUser, current, tenants, t.RefreshToken)
}

// resolveUser sub → 目录用户（id → email → username 三段桥接，ADR-0008）。
func (s *AuthService) resolveUser(claims map[string]any) (*auth.CurrentUser, error) {
	sub := strVal(claims["sub"])
	if sub == "" {
		return nil, familyerrors.ErrInvalidCredentials()
	}
	if u := s.dir.FindByID(sub); u != nil {
		return u, nil
	}
	if u := s.dir.FindByEmail(sub); u != nil {
		return u, nil
	}
	if u := s.dir.FindByUsername(sub); u != nil {
		return u, nil
	}
	return nil, familyerrors.ErrInvalidCredentials()
}

// session 签发双 token + 用户/租户结构（saasRefreshToken 空 → dev-placeholder，
// 密码登录路径口径）。
func (s *AuthService) session(user *auth.CurrentUser, tenantID string, tenants []map[string]any, saasRefreshToken string) (map[string]any, error) {
	accessToken, err := s.tokens.IssueAccess(user.ID, tenantID)
	if err != nil {
		return nil, err
	}
	if saasRefreshToken == "" {
		saasRefreshToken = "dev-placeholder"
	}
	refreshToken, err := s.tokens.IssueRefresh(user.ID, saasRefreshToken)
	if err != nil {
		return nil, err
	}
	if tenants == nil {
		tenants = directoryTenants(s.dir, user)
	}
	return map[string]any{
		"token":        accessToken,
		"refreshToken": refreshToken,
		"user":         userStruct(user),
		"tenants":      tenants,
	}, nil
}

// tenantsFrom memberships → MyTenant map；命中富化 tenant_id=code=tenantKey
// （2026-10-20 tenant_key 人裁）/name=真名；miss 降级原 tenantId。
func (s *AuthService) tenantsFrom(memberships []any, nameBy map[string]any) []map[string]any {
	out := make([]map[string]any, 0, len(memberships))
	for _, m := range memberships {
		mm, ok := m.(map[string]any)
		if !ok {
			continue
		}
		tid := strVal(mm["tenantId"])
		name := mapAny(nameBy, tid)
		if key := strVal(name["tenantKey"]); key != "" {
			tid = key
		}
		roleIDs := []string{}
		if rs, ok := mm["roleIds"].([]any); ok {
			for _, r := range rs {
				if role := strVal(r); role != "" {
					roleIDs = append(roleIDs, role)
				}
			}
		}
		tname := strVal(name["name"])
		if tname == "" {
			tname = strVal(mm["tenantId"])
		}
		out = append(out, map[string]any{"tenant_id": tid, "code": tid, "name": tname, "role_ids": roleIDs})
	}
	return out
}

// fetchTenantNames 平台租户名富化（best-effort：失败只 warn 返空，名字降级 tenantId）。
func (s *AuthService) fetchTenantNames(ctx context.Context, userID, accessToken string) map[string]any {
	if userID == "" || accessToken == "" {
		return map[string]any{}
	}
	list, err := s.saas.ListPlatformTenants(ctx, accessToken)
	if err != nil {
		slog.Warn("tenant name lookup failed", "user", userID, "err", err)
		return map[string]any{}
	}
	nameBy := map[string]any{}
	for _, t := range list {
		if tm, ok := t.(map[string]any); ok && strVal(tm["id"]) != "" {
			nameBy[strVal(tm["id"])] = tm
		}
	}
	return nameBy
}

// cacheMenus SSO/refresh 时点唯一持有 saas accessToken 的窗口，顺手拉菜单快照（失败不阻塞）。
func (s *AuthService) cacheMenus(ctx context.Context, userID, accessToken string) {
	if userID == "" || accessToken == "" {
		return
	}
	snapshot, err := s.saas.ListMyMenus(ctx, accessToken, LabAppCode)
	if err != nil {
		slog.Warn("menu snapshot fetch failed", "user", userID, "err", err)
		return
	}
	s.menuCache.Put(userID, s.mapper.Map(snapshot))
}

// cacheMenusWithServiceAccount 密码登录路径：服务账号登 saas 换 token 再拉菜单（失败只 warn）。
func (s *AuthService) cacheMenusWithServiceAccount(ctx context.Context, userID string) {
	if userID == "" {
		return
	}
	serviceUser := os.Getenv("LAB_SAAS_SERVICE_USER")
	servicePass := os.Getenv("LAB_SAAS_SERVICE_PASSWORD")
	if serviceUser == "" || servicePass == "" {
		slog.Warn("service-account menu snapshot skipped: LAB_SAAS_SERVICE_USER/PASSWORD 未配置")
		return
	}
	t, err := s.saas.ServiceLogin(ctx, serviceUser, servicePass)
	if err != nil {
		slog.Warn("service-account menu snapshot failed", "user", userID, "err", err)
		return
	}
	s.cacheMenus(ctx, userID, t.AccessToken)
}

// loginUser saas user → lab CurrentUser 直构；id/username/displayName 三段 presence
// 回退链（saas 无 displayName 时为空串，家族记忆空串陷阱）；role 首租户首个 roleId。
func loginUser(saasLogin *saas.LoginResponse, saasUser map[string]any, tenants []map[string]any) *auth.CurrentUser {
	role := "member"
	if len(tenants) > 0 {
		if rs, ok := tenants[0]["role_ids"].([]string); ok && len(rs) > 0 {
			role = rs[0]
		}
	}
	return &auth.CurrentUser{
		ID:          firstPresence(strVal(saasUser["id"]), saasLogin.UserID, "unknown"),
		Username:    firstPresence(strVal(saasUser["username"]), strVal(saasUser["email"]), "unknown"),
		DisplayName: firstPresence(strVal(saasUser["displayName"]), strVal(saasUser["username"]), strVal(saasUser["email"]), "未知用户"),
		RoleCode:    role,
	}
}

// activeAvailableTenants 过滤 removed/无 tenantId 的平台租户。
func activeAvailableTenants(available []any) []any {
	out := make([]any, 0, len(available))
	for _, t := range available {
		tm, ok := t.(map[string]any)
		if !ok || strVal(tm["tenantId"]) == "" || strVal(tm["status"]) == "removed" {
			continue
		}
		out = append(out, tm)
	}
	return out
}

func resolveTenantKey(saasTenantID string, nameBy map[string]any) string {
	t := mapAny(nameBy, saasTenantID)
	if key := strVal(t["tenantKey"]); key != "" {
		return key
	}
	return saasTenantID
}

func directoryTenants(dir *auth.Directory, _ *auth.CurrentUser) []map[string]any {
	out := make([]map[string]any, 0, 3)
	for _, t := range dir.TenantsOf("") {
		out = append(out, map[string]any{
			"tenant_id": t.TenantID, "code": t.Code, "name": t.Name, "role_ids": t.RoleIDs,
		})
	}
	return out
}

func userStruct(u *auth.CurrentUser) map[string]any {
	return map[string]any{"id": u.ID, "username": u.Username, "displayName": u.DisplayName, "roleCode": u.RoleCode}
}

func meStruct(u *auth.CurrentUser, tenants []map[string]any, current string) map[string]any {
	return map[string]any{"user": userStruct(u), "tenants": tenants, "currentTenantId": current}
}

func firstTenantID(tenants []map[string]any) string {
	if len(tenants) == 0 {
		return ""
	}
	return tenants[0]["tenant_id"].(string)
}

func orEmptyMap(m map[string]any) map[string]any {
	if m == nil {
		return map[string]any{}
	}
	return m
}

func mapAny(m map[string]any, key string) map[string]any {
	if m == nil {
		return map[string]any{}
	}
	if v, ok := m[key].(map[string]any); ok {
		return v
	}
	return map[string]any{}
}

func firstPresence(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func strVal(v any) string {
	s, _ := v.(string)
	return s
}

func trimSpace(s string) string {
	start, end := 0, len(s)
	for start < end && isSpaceByte(s[start]) {
		start++
	}
	for end > start && isSpaceByte(s[end-1]) {
		end--
	}
	return s[start:end]
}

func isSpaceByte(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r' || b == '\v' || b == '\f'
}
