package auth

// jwt_test.go — 手写 HS256 安全面（lab 分叉：无 aud，镜像 rails lab_auth/token_service
// 「lab 契约 token 不带 audience」家族有意差异）：
// 正例 round-trip（签发无 aud claim）+ 负例（过期/签名错/alg≠HS256）+ env fail-fast。

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func withJWTEnv(t *testing.T) {
	t.Helper()
	t.Setenv("JWT_SIGNING_KEY", "test-signing-key-0123456789abcdef")
	t.Setenv("JWT_ISSUER", "lab-management")
	t.Setenv("JWT_TTL_SECONDS", "3600")
}

func TestEncodeVerifyRoundTrip(t *testing.T) {
	withJWTEnv(t)
	tok, err := Encode(map[string]any{"sub": "u1", "tenant_id": "t1", "typ": "access"})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	claims, err := Verify(tok)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if claims["sub"] != "u1" || claims["tenant_id"] != "t1" || claims["typ"] != "access" {
		t.Fatalf("claims 不符: %v", claims)
	}
	if claims["iss"] != "lab-management" {
		t.Fatalf("iss 未注入: %v", claims)
	}
	// lab 家族有意差异：签发**不带 aud claim**
	if _, hasAud := claims["aud"]; hasAud {
		t.Fatalf("lab token 不得带 aud claim: %v", claims)
	}
}

func sign(key string, header, payload map[string]any) string {
	h, _ := json.Marshal(header)
	p, _ := json.Marshal(payload)
	mac := hs256(key, segments(h, p))
	return segments(h, p) + "." + mac
}

func segments(parts ...[]byte) string {
	segs := make([]string, 0, len(parts))
	for _, p := range parts {
		segs = append(segs, base64.RawURLEncoding.EncodeToString(p))
	}
	return strings.Join(segs, ".")
}

func TestExpiredTokenRejected(t *testing.T) {
	withJWTEnv(t)
	key := "test-signing-key-0123456789abcdef"
	tok := sign(key,
		map[string]any{"alg": "HS256", "typ": "JWT"},
		map[string]any{"sub": "u1", "exp": time.Now().Add(-time.Minute).Unix(), "iss": "lab-management"},
	)
	if _, err := Verify(tok); err == nil {
		t.Fatal("过期 token 必须拒")
	}
}

func TestBadSignatureRejected(t *testing.T) {
	withJWTEnv(t)
	tok := sign("another-key-ffffffffffffffffffff",
		map[string]any{"alg": "HS256", "typ": "JWT"},
		map[string]any{"sub": "u1", "exp": time.Now().Add(time.Hour).Unix(), "iss": "lab-management"},
	)
	if _, err := Verify(tok); err == nil {
		t.Fatal("签名错必须拒")
	}
}

func TestAlgConfusionRejected(t *testing.T) {
	withJWTEnv(t)
	// header alg=none（签名段留空）——alg 混淆必须拒
	tok := sign("", map[string]any{"alg": "none", "typ": "JWT"},
		map[string]any{"sub": "u1", "exp": time.Now().Add(time.Hour).Unix()})
	if _, err := Verify(tok); err == nil {
		t.Fatal("alg=none 必须拒")
	}
	// alg=HS512 但用 HS256 密钥段——同样拒
	tok2 := sign("test-signing-key-0123456789abcdef",
		map[string]any{"alg": "HS512", "typ": "JWT"},
		map[string]any{"sub": "u1", "exp": time.Now().Add(time.Hour).Unix(), "iss": "lab-management"},
	)
	if _, err := Verify(tok2); err == nil {
		t.Fatal("alg≠HS256 必须拒")
	}
}

// TestVerifyIgnoresAudClaim lab 验签不读 aud——payload 里有没有 aud 都放行。
func TestVerifyIgnoresAudClaim(t *testing.T) {
	withJWTEnv(t)
	tok := sign("test-signing-key-0123456789abcdef",
		map[string]any{"alg": "HS256", "typ": "JWT"},
		map[string]any{"sub": "u1", "exp": time.Now().Add(time.Hour).Unix(), "iss": "lab-management", "aud": "whatever"},
	)
	if _, err := Verify(tok); err != nil {
		t.Fatalf("lab 验签不验 aud，不应拒: %v", err)
	}
}

func TestMissingEnvFailsFast(t *testing.T) {
	t.Setenv("JWT_SIGNING_KEY", "")
	if _, err := Encode(map[string]any{"sub": "u1"}); err == nil {
		t.Fatal("JWT_SIGNING_KEY 缺失必须报错（禁默认值兜底）")
	}
}

func TestWrongSegmentCountRejected(t *testing.T) {
	withJWTEnv(t)
	if _, err := Verify("a.b"); err == nil {
		t.Fatal("非三段 token 必须拒")
	}
}
