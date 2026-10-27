// Package saas — saas IdP 出站客户端（REQ-2026-004 Task 4）。
//
// 镜像 rails saas_auth_client.rb / saas_me_client.rb（lab-springboot SaasAuthClient
// 同谱系）：账密校验代理 + 服务账号登录 + OAuth token 交换 + /me 系快照读取。
// 错误三分流（上游语义原样保留，service 层负责转 401/502）：
//
//	401        → ErrUnauthorizedClient（saas 明确拒绝该 client/凭据）
//	其余 4xx   → ErrInvalidGrant
//	5xx + 网络 → ErrUpstreamUnavailable
package saas

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// 三类上游错误（空 struct：errors.Is 按值相等解包判定）。
type (
	// ErrUnauthorizedClient saas 401。
	ErrUnauthorizedClient struct{}
	// ErrInvalidGrant saas 其余 4xx。
	ErrInvalidGrant struct{}
	// ErrUpstreamUnavailable saas 5xx / 连接失败 / 超时。
	ErrUpstreamUnavailable struct{}
)

func (ErrUnauthorizedClient) Error() string  { return "saas 401 unauthorized client" }
func (ErrInvalidGrant) Error() string        { return "saas invalid grant" }
func (ErrUpstreamUnavailable) Error() string { return "saas upstream unavailable" }

// 401 分流选择子（rails 双客户端语义镜像：auth 登录/token 401→UnauthorizedClient，
// me 系 401→InvalidGrant）。
var (
	errUnauthorizedClientOn401 error = ErrUnauthorizedClient{}
	errInvalidGrantOn401       error = ErrInvalidGrant{}
)

// LoginResponse saas /api/v1/auth/login 完整响应（原生登录代理用；REQ-2026-023）。
// saas user 无 displayName、availableTenants 无租户名（富化走 ListPlatformTenants）。
type LoginResponse struct {
	User             map[string]any `json:"-"`
	AvailableTenants []any          `json:"-"`
	AccessToken      string
	RefreshToken     string
	UserID           string
	CurrentTenantID  string
}

// TokenResponse saas token 端点响应（service_login / token 交换共用）。
type TokenResponse struct {
	AccessToken  string
	RefreshToken string
	TokenType    string
	ExpiresIn    float64
	Scope        string
}

// TokenRequest OAuth token 交换入参（grant_type=authorization_code / refresh_token）。
type TokenRequest struct {
	GrantType    string
	Code         string
	RefreshToken string
	RedirectURI  string
}

// HTTPClient IdP 客户端真实现（base 等五键由构造方注入；env 读取在装配层）。
type HTTPClient struct {
	base            string
	clientID        string
	clientSecret    string
	defaultTenantID string
	serviceClientID string
	http            *http.Client
}

// NewHTTPClient 构造（五键 fail-fast 校验在 config 层；这里只收值）。
func NewHTTPClient(base, clientID, clientSecret, defaultTenantID, serviceClientID string) *HTTPClient {
	return &HTTPClient{
		base:            strings.TrimRight(base, "/"),
		clientID:        clientID,
		clientSecret:    clientSecret,
		defaultTenantID: defaultTenantID,
		serviceClientID: serviceClientID,
		http:            &http.Client{Timeout: 10 * time.Second},
	}
}

// UserLogin 原生登录账密校验代理——clientId 取 OAuth client_id（LAB_SAAS_CLIENT_ID，
// 与 nextjs/springboot/aspnetcore 出站语义对齐；服务账号路径用 service_client_id）。
func (c *HTTPClient) UserLogin(ctx context.Context, username, password string) (*LoginResponse, error) {
	h, err := c.postJSON(ctx, "/api/v1/auth/login", map[string]any{
		"username": username, "password": password, "clientId": c.clientID,
	})
	if err != nil {
		return nil, err
	}
	return &LoginResponse{
		User:             toMap(h["user"]),
		AvailableTenants: toSlice(h["availableTenants"]),
		AccessToken:      str(h["accessToken"]),
		RefreshToken:     str(h["refreshToken"]),
		UserID:           str(h["userId"]),
		CurrentTenantID:  str(h["currentTenantId"]),
	}, nil
}

// ServiceLogin 服务账号密码登录（菜单快照/富化用；clientId=service_client_id）。
func (c *HTTPClient) ServiceLogin(ctx context.Context, username, password string) (*TokenResponse, error) {
	h, err := c.postJSON(ctx, "/api/v1/auth/login", map[string]any{
		"username": username, "password": password, "clientId": c.serviceClientID,
	})
	if err != nil {
		return nil, err
	}
	return tokenResponse(h), nil
}

// Token OAuth §4.1.3/§6 code 换 token 或 refresh_token 续（tenantId=default 租户）。
func (c *HTTPClient) Token(ctx context.Context, req TokenRequest) (*TokenResponse, error) {
	body := map[string]any{
		"grantType": req.GrantType, "clientId": c.clientID,
		"clientSecret": c.clientSecret, "tenantId": c.defaultTenantID,
	}
	if req.Code != "" {
		body["code"] = req.Code
	}
	if req.RefreshToken != "" {
		body["refreshToken"] = req.RefreshToken
	}
	if req.RedirectURI != "" {
		body["redirectUri"] = req.RedirectURI
	}
	h, err := c.postJSON(ctx, "/api/v1/oauth/token", body)
	if err != nil {
		return nil, err
	}
	return tokenResponse(h), nil
}

// Whoami saas /api/v1/me → {id, email, displayName, memberships, currentTenantId}。
func (c *HTTPClient) Whoami(ctx context.Context, accessToken string) (map[string]any, error) {
	return c.getJSON(ctx, "/api/v1/me", accessToken)
}

// ListMyTenants saas /api/v1/me/tenants（空 → 空切片）。
func (c *HTTPClient) ListMyTenants(ctx context.Context, accessToken string) ([]any, error) {
	v, err := c.getJSON(ctx, "/api/v1/me/tenants", accessToken)
	if err != nil {
		return nil, err
	}
	return toSlice(v), nil
}

// ListMyMenus saas /me/menus 返 Map<appCode, tree>，按 appCode 取子树（缺 → 空切片）。
func (c *HTTPClient) ListMyMenus(ctx context.Context, accessToken, appCode string) ([]any, error) {
	m, err := c.getJSON(ctx, "/api/v1/me/menus", accessToken)
	if err != nil {
		return nil, err
	}
	if m == nil {
		return []any{}, nil
	}
	return toSlice(m[appCode]), nil
}

// ListPlatformTenants 平台租户列表（display-name 富化；分页壳 {items,total}）。
func (c *HTTPClient) ListPlatformTenants(ctx context.Context, accessToken string) ([]any, error) {
	page, err := c.getJSON(ctx, "/api/v1/admin/tenants?page=0&pageSize=100", accessToken)
	if err != nil {
		return nil, err
	}
	if page == nil {
		return []any{}, nil
	}
	return toSlice(page["items"]), nil
}

// ---- 内部：POST/GET + 错误三分流（rails post_json/get_json 镜像） ----

func (c *HTTPClient) postJSON(ctx context.Context, path string, body map[string]any) (map[string]any, error) {
	buf, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+path, bytes.NewReader(buf))
	if err != nil {
		return nil, wrapUpstream(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	return doJSON(resp, err, errUnauthorizedClientOn401)
}

func (c *HTTPClient) getJSON(ctx context.Context, pathWithQuery, token string) (map[string]any, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+pathWithQuery, nil)
	if err != nil {
		return nil, wrapUpstream(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	// me 系 401 也归 InvalidGrant（rails SaasMeClient 无 UnauthorizedClient 分支的镜像）
	resp, err := c.http.Do(req)
	return doJSON(resp, err, errInvalidGrantOn401)
}

func doJSON(resp *http.Response, err error, on401 error) (map[string]any, error) {
	if err != nil {
		return nil, wrapUpstream(err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, wrapUpstream(err)
	}
	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		if len(raw) == 0 {
			return nil, nil
		}
		var h map[string]any
		if err := json.Unmarshal(raw, &h); err != nil {
			return nil, fmt.Errorf("saas 响应解析失败: %w", err)
		}
		return h, nil
	case resp.StatusCode == 401:
		return nil, fmt.Errorf("saas 401: %w", on401)
	case resp.StatusCode < 500:
		return nil, fmt.Errorf("saas %d: %w", resp.StatusCode, ErrInvalidGrant{})
	default:
		return nil, fmt.Errorf("saas %d: %w", resp.StatusCode, ErrUpstreamUnavailable{})
	}
}

func wrapUpstream(err error) error {
	return fmt.Errorf("saas connect failed: %w", ErrUpstreamUnavailable{})
}

func tokenResponse(h map[string]any) *TokenResponse {
	return &TokenResponse{
		AccessToken:  str(h["accessToken"]),
		RefreshToken: str(h["refreshToken"]),
		TokenType:    str(h["tokenType"]),
		ExpiresIn:    f64(h["expiresIn"]),
		Scope:        str(h["scope"]),
	}
}

func str(v any) string {
	s, _ := v.(string)
	return s
}

func f64(v any) float64 {
	f, _ := v.(float64)
	return f
}

func toSlice(v any) []any {
	s, ok := v.([]any)
	if !ok {
		return []any{}
	}
	return s
}

func toMap(v any) map[string]any {
	m, ok := v.(map[string]any)
	if !ok {
		return nil
	}
	return m
}
