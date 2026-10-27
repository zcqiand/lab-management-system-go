package harness

// assemble_auth_test.go — Task 5：Assemble 必须把认证域服务接进路由面（否则
// auth 端点全部 500 INTERNAL）。真 HTTPClient 指向不可达地址：出站失败走
// 502 SAAS_UNAVAILABLE 链路——非 500 即证明 Auth 已接线（错误分流归 service 测试）。

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"lab-management-system-go/internal/impl/auth"
	"lab-management-system-go/internal/impl/saas"
	"lab-management-system-go/internal/impl/service"
)

func TestAssembleWiresAuthService(t *testing.T) {
	t.Setenv("JWT_SIGNING_KEY", "test-signing-key-32-bytes-long!!")
	t.Setenv("JWT_ISSUER", "lab-management-system")
	t.Setenv("JWT_TTL_SECONDS", "3600")
	t.Setenv("JWT_REFRESH_TTL_SECONDS", "604800")
	t.Setenv("LAB_SAAS_SERVICE_USER", "alice")
	t.Setenv("LAB_SAAS_SERVICE_PASSWORD", "dev123456")

	// 指向不可达端口的真 client：出站失败应映射 502（而非 Auth==nil 的 500）
	client := saas.NewHTTPClient("http://127.0.0.1:1", "lab-management", "sec", "uuid-1", "svc-client")
	authSvc := service.NewAuthService(
		auth.NewDirectory(),
		auth.NewSnapshotCache(auth.SnapshotCacheTTL),
		auth.NewSnapshotCache(auth.SnapshotCacheTTL),
		client,
	)
	t.Setenv("LAB_CORS_ALLOWED_ORIGINS", "http://localhost:5201")
	mux := Assemble(nil, authSvc)

	req := httptest.NewRequest("POST", "/api/auth/native-login", strings.NewReader(`{"username":"alice","password":"x"}`))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("Auth 已接线时出站失败应 502 SAAS_UNAVAILABLE，得 %d: %s", rec.Code, rec.Body.String())
	}
}

// TestCORSPreflightAllowedOrigin 白名单 origin 预检放行（final review Critical#1：
// plan Task 4 漏交付 cors.go，LAB_CORS_ALLOWED_ORIGINS 全仓零消费者；rails cors.rb
// 镜像——origin 精确匹配 + expose Authorization + 预检 204）。
func TestCORSPreflightAllowedOrigin(t *testing.T) {
	t.Setenv("LAB_CORS_ALLOWED_ORIGINS", "http://localhost:5201,http://localhost:5202")
	h := Assemble(nil, nil)
	req := httptest.NewRequest("OPTIONS", "/api/receipts", nil)
	req.Header.Set("Origin", "http://localhost:5201")
	req.Header.Set("Access-Control-Request-Method", "POST")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 204 {
		t.Fatalf("白名单 origin 预检期望 204，得到 %d", rec.Code)
	}
	if ao := rec.Header().Get("Access-Control-Allow-Origin"); ao != "http://localhost:5201" {
		t.Fatalf("ACAO 应回显 origin: %q", ao)
	}
	if ae := rec.Header().Get("Access-Control-Expose-Headers"); ae != "Authorization" {
		t.Fatalf("expose 应含 Authorization: %q", ae)
	}
}

// TestCORSPreflightDisallowedOriginOriginHeaderAbsent 非白名单 origin 不出 CORS 头
// （rails rack-cors 语义：请求照常处理，仅无 ACAO——由浏览器拦）。
func TestCORSPreflightDisallowedOriginOriginHeaderAbsent(t *testing.T) {
	t.Setenv("LAB_CORS_ALLOWED_ORIGINS", "http://localhost:5201")
	h := Assemble(nil, nil)
	req := httptest.NewRequest("OPTIONS", "/api/receipts", nil)
	req.Header.Set("Origin", "http://evil.example")
	req.Header.Set("Access-Control-Request-Method", "POST")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if ao := rec.Header().Get("Access-Control-Allow-Origin"); ao != "" {
		t.Fatalf("非白名单 origin 不应得 ACAO: %q", ao)
	}
}

// TestCORSActualRequestGetsACAO 实际（非预检）跨域响应带 ACAO。
func TestCORSActualRequestGetsACAO(t *testing.T) {
	t.Setenv("LAB_CORS_ALLOWED_ORIGINS", "http://localhost:5201")
	h := Assemble(nil, nil)
	req := httptest.NewRequest("GET", "/health", nil)
	req.Header.Set("Origin", "http://localhost:5201")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if ao := rec.Header().Get("Access-Control-Allow-Origin"); ao != "http://localhost:5201" {
		t.Fatalf("实际请求应带 ACAO: %q", ao)
	}
}

// TestCORSMissingEnvFailFast 键缺失/空 → 装配期 fail-fast（suite-hard-rules §1）。
func TestCORSMissingEnvFailFast(t *testing.T) {
	t.Setenv("LAB_CORS_ALLOWED_ORIGINS", "")
	defer func() {
		if recover() == nil {
			t.Fatal("空 LAB_CORS_ALLOWED_ORIGINS 应 fail-fast panic")
		}
	}()
	Assemble(nil, nil)
}
