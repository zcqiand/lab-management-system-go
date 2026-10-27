package auth

// token_service_test.go — lab 自签 JWT 双 token（镜像 rails lab_auth/token_service）：
// access {sub, tenant_id?, typ=access} 1h；refresh {sub, saas_refresh_token, typ=refresh} 7d；
// typ 校验；key <32B fail-fast。

import (
	"strings"
	"testing"
	"time"
)

func withTokenEnv(t *testing.T) {
	t.Helper()
	t.Setenv("JWT_SIGNING_KEY", "test-signing-key-0123456789abcdef")
	t.Setenv("JWT_ISSUER", "lab-management-system")
	t.Setenv("JWT_TTL_SECONDS", "3600")
	t.Setenv("JWT_REFRESH_TTL_SECONDS", "604800")
}

func TestIssueAccessRoundTrip(t *testing.T) {
	withTokenEnv(t)
	svc := &TokenService{}
	tok, err := svc.IssueAccess("u1", "TENANT-001")
	if err != nil {
		t.Fatalf("issue access: %v", err)
	}
	claims, err := svc.VerifyTyp(tok, TypAccess)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if claims["sub"] != "u1" || claims["tenant_id"] != "TENANT-001" || claims["typ"] != TypAccess {
		t.Fatalf("claims 不符: %v", claims)
	}
	if _, hasAud := claims["aud"]; hasAud {
		t.Fatalf("lab token 不带 aud: %v", claims)
	}
}

func TestIssueAccessOmitsEmptyTenant(t *testing.T) {
	withTokenEnv(t)
	svc := &TokenService{}
	tok, _ := svc.IssueAccess("u1", "")
	claims, err := svc.VerifyTyp(tok, TypAccess)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if _, has := claims["tenant_id"]; has {
		t.Fatalf("空租户不得出 tenant_id claim: %v", claims)
	}
}

func TestIssueRefreshEmbedsSaasRefresh(t *testing.T) {
	withTokenEnv(t)
	svc := &TokenService{}
	tok, err := svc.IssueRefresh("u1", "saas-rt-1")
	if err != nil {
		t.Fatalf("issue refresh: %v", err)
	}
	claims, err := svc.VerifyTyp(tok, TypRefresh)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if claims["saas_refresh_token"] != "saas-rt-1" || claims["typ"] != TypRefresh {
		t.Fatalf("refresh claims 不符: %v", claims)
	}
	// refresh TTL 7d：exp - iat = 604800
	ttl := int64(claims["exp"].(float64)) - int64(claims["iat"].(float64))
	if ttl != 604800 {
		t.Fatalf("refresh TTL 应 604800，得 %d", ttl)
	}
}

func TestVerifyTypMismatchRejected(t *testing.T) {
	withTokenEnv(t)
	svc := &TokenService{}
	access, _ := svc.IssueAccess("u1", "t1")
	if _, err := svc.VerifyTyp(access, TypRefresh); err == nil {
		t.Fatal("access token 过 typ=refresh 校验必须拒")
	}
}

func TestVerifyTypExpiredRejected(t *testing.T) {
	withTokenEnv(t)
	svc := &TokenService{}
	tok, _ := svc.IssueRefresh("u1", "rt")
	// 手工签过期 refresh：exp 过去
	claims := map[string]any{"sub": "u1", "saas_refresh_token": "rt", "typ": TypRefresh,
		"iss": "lab-management-system", "iat": time.Now().Add(-time.Hour).Unix(),
		"exp": time.Now().Add(-time.Minute).Unix()}
	expired := sign("test-signing-key-0123456789abcdef",
		map[string]any{"alg": "HS256", "typ": "JWT"}, claims)
	_ = tok
	if _, err := svc.VerifyTyp(expired, TypRefresh); err == nil {
		t.Fatal("过期 refresh 必须拒")
	}
}

func TestShortKeyFailsFast(t *testing.T) {
	t.Setenv("JWT_SIGNING_KEY", "short")
	t.Setenv("JWT_ISSUER", "lab-management-system")
	t.Setenv("JWT_TTL_SECONDS", "3600")
	t.Setenv("JWT_REFRESH_TTL_SECONDS", "604800")
	svc := &TokenService{}
	if _, err := svc.IssueAccess("u1", "t1"); err == nil || !strings.Contains(err.Error(), "32") {
		t.Fatalf("key <32B 必须 fail-fast: %v", err)
	}
}
