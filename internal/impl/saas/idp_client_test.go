package saas

// idp_client_test.go — saas IdP 出站客户端（镜像 rails saas_auth_client/saas_me_client，
// REQ-2026-004 Task 4）。httptest fake IdP 断言：请求体形状、错误三分流
// （401→UnauthorizedClient / 其余 4xx→InvalidGrant / 5xx+网络→UpstreamUnavailable）、
// menus 按 appCode 取子树。mock-friendly：不依赖真 saas。

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func fakeIdp(t *testing.T, status int, body string, seen *map[string]any) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if seen != nil && (r.URL.Path == "/api/v1/auth/login" || r.URL.Path == "/api/v1/oauth/token") {
			_ = json.NewDecoder(r.Body).Decode(seen)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func testClient(base string) *HTTPClient {
	return NewHTTPClient(base, "lab-management", "lab-management-secret", "TENANT-001", "lab-management-service")
}

func TestUserLoginParsesLoginResponse(t *testing.T) {
	seen := map[string]any{}
	srv := fakeIdp(t, 200, `{"user":{"email":"alice@x.uk"},"availableTenants":[{"id":"t1"}],"accessToken":"at","refreshToken":"rt","userId":"u1","currentTenantId":"t1"}`, &seen)
	resp, err := testClient(srv.URL).UserLogin(context.Background(), "alice", "dev123456")
	if err != nil {
		t.Fatalf("user login: %v", err)
	}
	if resp.AccessToken != "at" || resp.RefreshToken != "rt" || resp.UserID != "u1" || resp.CurrentTenantID != "t1" {
		t.Fatalf("LoginResponse 字段不符: %+v", resp)
	}
	if resp.User == nil || resp.AvailableTenants == nil {
		t.Fatalf("user/availableTenants 应透传: %+v", resp)
	}
	// 请求体对齐 saas LoginRequest {username,password,clientId=OAuth client_id}
	if seen["username"] != "alice" || seen["password"] != "dev123456" || seen["clientId"] != "lab-management" {
		t.Fatalf("请求体不符: %v", seen)
	}
}

func TestUserLoginErrorTripartition(t *testing.T) {
	cases := []struct {
		name   string
		status int
		want   error
	}{
		{"saas 401", 401, ErrUnauthorizedClient{}},
		{"saas 403", 403, ErrInvalidGrant{}},
		{"saas 500", 500, ErrUpstreamUnavailable{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := fakeIdp(t, tc.status, `{"code":"X"}`, nil)
			_, err := testClient(srv.URL).UserLogin(context.Background(), "a", "b")
			if !errors.Is(err, tc.want) {
				t.Fatalf("期望 %T，得 %v", tc.want, err)
			}
		})
	}
}

func TestUserLoginConnectionRefusedIsUpstream(t *testing.T) {
	// 关口：起后立刻关——连接必拒
	srv := fakeIdp(t, 200, `{}`, nil)
	url := srv.URL
	srv.Close()
	_, err := testClient(url).UserLogin(context.Background(), "a", "b")
	if !errors.Is(err, ErrUpstreamUnavailable{}) {
		t.Fatalf("连接拒绝应为 UpstreamUnavailable，得 %v", err)
	}
}

func TestServiceLoginUsesServiceClientID(t *testing.T) {
	seen := map[string]any{}
	srv := fakeIdp(t, 200, `{"accessToken":"at","refreshToken":"rt","tokenType":"Bearer","expiresIn":3600,"scope":"read"}`, &seen)
	resp, err := testClient(srv.URL).ServiceLogin(context.Background(), "alice", "dev123456")
	if err != nil {
		t.Fatalf("service login: %v", err)
	}
	if resp.AccessToken != "at" || resp.ExpiresIn != 3600 {
		t.Fatalf("TokenResponse 不符: %+v", resp)
	}
	if seen["clientId"] != "lab-management-service" {
		t.Fatalf("service login 应走 service_client_id: %v", seen)
	}
}

func TestWhoamiAndMenus(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/me":
			_, _ = w.Write([]byte(`{"id":"u1","email":"a@x.uk","currentTenantId":"t1"}`))
		case "/api/v1/me/menus":
			_, _ = w.Write([]byte(`{"lab-management":[{"title":"菜单A"}],"other":[{"title":"X"}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	c := testClient(srv.URL)
	me, err := c.Whoami(context.Background(), "tok-1")
	if err != nil {
		t.Fatalf("whoami: %v", err)
	}
	if me["id"] != "u1" {
		t.Fatalf("whoami 不符: %v", me)
	}
	if gotAuth != "Bearer tok-1" {
		t.Fatalf("Authorization 头不符: %q", gotAuth)
	}
	menus, err := c.ListMyMenus(context.Background(), "tok-1", "lab-management")
	if err != nil {
		t.Fatalf("menus: %v", err)
	}
	b, _ := json.Marshal(menus)
	if !strings.Contains(string(b), "菜单A") || strings.Contains(string(b), `"X"`) {
		t.Fatalf("menus 应按 appCode 取子树: %s", b)
	}
}

func TestWhoami4xx5xxMapping(t *testing.T) {
	srv401 := fakeIdp(t, 401, `{}`, nil)
	if _, err := testClient(srv401.URL).Whoami(context.Background(), "t"); !errors.Is(err, ErrInvalidGrant{}) {
		t.Fatalf("whoami 401 应 InvalidGrant，得 %v", err)
	}
	srv503 := fakeIdp(t, 503, `{}`, nil)
	if _, err := testClient(srv503.URL).Whoami(context.Background(), "t"); !errors.Is(err, ErrUpstreamUnavailable{}) {
		t.Fatalf("whoami 503 应 Upstream，得 %v", err)
	}
}

func TestTokenExchangeBodyShape(t *testing.T) {
	seen := map[string]any{}
	srv := fakeIdp(t, 200, `{"accessToken":"at","refreshToken":"rt","tokenType":"Bearer","expiresIn":3600}`, &seen)
	c := testClient(srv.URL)
	_, err := c.Token(context.Background(), TokenRequest{GrantType: "authorization_code", Code: "ac_1", RedirectURI: "http://localhost/cb"})
	if err != nil {
		t.Fatalf("token: %v", err)
	}
	if seen["grantType"] != "authorization_code" || seen["clientId"] != "lab-management" ||
		seen["clientSecret"] != "lab-management-secret" || seen["tenantId"] != "TENANT-001" ||
		seen["code"] != "ac_1" || seen["redirectUri"] != "http://localhost/cb" {
		t.Fatalf("token 体不符: %v", seen)
	}
}
