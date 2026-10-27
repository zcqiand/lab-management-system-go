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
	mux := Assemble(nil, authSvc)

	req := httptest.NewRequest("POST", "/api/auth/native-login", strings.NewReader(`{"username":"alice","password":"x"}`))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("Auth 已接线时出站失败应 502 SAAS_UNAVAILABLE，得 %d: %s", rec.Code, rec.Body.String())
	}
}
