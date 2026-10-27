package auth

// token_service.go — lab 自签 JWT 双 token（镜像 rails lab_auth/token_service，
// lab-springboot LabJwtSigner 谱系）：
//   - access:  {sub, tenant_id?, typ=access, iss, iat, exp}（JWT_TTL_SECONDS；无 aud）
//   - refresh: {sub, saas_refresh_token, typ=refresh, iss, iat, exp}（JWT_REFRESH_TTL_SECONDS）
//
// fail-fast：JWT_SIGNING_KEY 缺/<32B 直接报错阻断（suite-hard-rules §1）。

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"
)

func jsonMarshal(v any) ([]byte, error) { return json.Marshal(v) }

const (
	// TypAccess access token 类型 claim。
	TypAccess = "access"
	// TypRefresh refresh token 类型 claim。
	TypRefresh = "refresh"
	// MinSigningKeyLength 签名钥最小长度（rails 同款 ≥32B 硬约束）。
	MinSigningKeyLength = 32
)

// TokenService 无状态签发器（env 即配置；构造零参，测试 t.Setenv 即可）。
type TokenService struct{}

type tokenEnv struct {
	key, issuer string
	accessTTL   int64
	refreshTTL  int64
}

func tokenEnvRead() (tokenEnv, error) {
	e := tokenEnv{}
	e.key = os.Getenv("JWT_SIGNING_KEY")
	if len(e.key) < MinSigningKeyLength {
		return e, fmt.Errorf("JWT_SIGNING_KEY 缺失或弱于 %d 字节（禁默认值兜底）", MinSigningKeyLength)
	}
	e.issuer = os.Getenv("JWT_ISSUER")
	if e.issuer == "" {
		return e, errors.New("缺少环境变量 JWT_ISSUER（禁默认值兜底）")
	}
	accessTTL, err := strconv.Atoi(os.Getenv("JWT_TTL_SECONDS"))
	if err != nil || accessTTL <= 0 {
		return e, fmt.Errorf("JWT_TTL_SECONDS 非法 %q", os.Getenv("JWT_TTL_SECONDS"))
	}
	refreshTTL, err := strconv.Atoi(os.Getenv("JWT_REFRESH_TTL_SECONDS"))
	if err != nil || refreshTTL <= 0 {
		return e, fmt.Errorf("JWT_REFRESH_TTL_SECONDS 非法 %q", os.Getenv("JWT_REFRESH_TTL_SECONDS"))
	}
	e.accessTTL = int64(accessTTL)
	e.refreshTTL = int64(refreshTTL)
	return e, nil
}

// AccessTTLSeconds 取 access TTL（login payload expires_in 用）。
func AccessTTLSeconds() (int64, error) {
	e, err := tokenEnvRead()
	return e.accessTTL, err
}

// IssueAccess 签发 access token（tenant_id 空 → 不出 claim）。
func (TokenService) IssueAccess(userID, tenantID string) (string, error) {
	e, err := tokenEnvRead()
	if err != nil {
		return "", err
	}
	now := time.Now().Unix()
	claims := map[string]any{
		"sub": userID, "iat": now, "exp": now + e.accessTTL, "typ": TypAccess, "iss": e.issuer,
	}
	if tenantID != "" {
		claims["tenant_id"] = tenantID
	}
	return signClaims(e.key, claims)
}

// IssueRefresh 签发 refresh token（内嵌 saas_refresh_token，rotate-once 载体）。
func (TokenService) IssueRefresh(userID, saasRefreshToken string) (string, error) {
	e, err := tokenEnvRead()
	if err != nil {
		return "", err
	}
	now := time.Now().Unix()
	return signClaims(e.key, map[string]any{
		"sub": userID, "saas_refresh_token": saasRefreshToken,
		"iat": now, "exp": now + e.refreshTTL, "typ": TypRefresh, "iss": e.issuer,
	})
}

// VerifyTyp 验签 + exp/iss/typ 校验（无效返回错误，上层统一转 401）。
func (TokenService) VerifyTyp(token, wantTyp string) (map[string]any, error) {
	e, err := tokenEnvRead()
	if err != nil {
		return nil, err
	}
	claims, err := Verify(token)
	if err != nil {
		return nil, err
	}
	if claims["typ"] != wantTyp {
		return nil, fmt.Errorf("unexpected typ: %v", claims["typ"])
	}
	_ = e
	return claims, nil
}

func signClaims(key string, claims map[string]any) (string, error) {
	header, err := jsonMarshal(map[string]string{"alg": "HS256", "typ": "JWT"})
	if err != nil {
		return "", err
	}
	body, err := jsonMarshal(claims)
	if err != nil {
		return "", err
	}
	signingInput := b64(header) + "." + b64(body)
	return signingInput + "." + hs256(key, signingInput), nil
}
