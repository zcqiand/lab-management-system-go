// Package auth — 认证基础件：手写 HS256 JWT（禁三方 jwt 库，version-lock 裁定）、
// Bearer 守卫（镜像 rails lab_auth/token_service + jwt_guard concern）。
//
// lab 家族有意差异（REQ-2026-004，镜像 rails token_service L6-8）：lab 契约
// token **不带 audience claim**，验签也不读 aud——与 saas 家族 verifier 分叉。
package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"
)

// env 必需三件（fail-fast，无兜底——suite-hard-rules §1）。无 JWT_AUDIENCE：
// lab token 不带 aud（家族有意差异，见包注释）。
func jwtEnv() (key, issuer string, ttl int, err error) {
	key = os.Getenv("JWT_SIGNING_KEY")
	if key == "" {
		return "", "", 0, errors.New("缺少环境变量 JWT_SIGNING_KEY（禁默认值兜底）")
	}
	issuer = os.Getenv("JWT_ISSUER")
	if issuer == "" {
		return "", "", 0, errors.New("缺少环境变量 JWT_ISSUER（禁默认值兜底）")
	}
	raw := os.Getenv("JWT_TTL_SECONDS")
	ttl, err = strconv.Atoi(raw)
	if err != nil || ttl <= 0 {
		return "", "", 0, fmt.Errorf("JWT_TTL_SECONDS 非法 %q: %w", raw, err)
	}
	return key, issuer, ttl, nil
}

func b64(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

// TTLSeconds 取 JWT_TTL_SECONDS（login payload expires_in 用；fail-fast 同 encode）。
func TTLSeconds() (int, error) {
	_, _, ttl, err := jwtEnv()
	return ttl, err
}

func hs256(key string, signingInput string) string {
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte(signingInput))
	return b64(mac.Sum(nil))
}

// Encode 签发 HS256：payload 注入 iat/exp/iss（**无 aud**，rails token_service 同形：
// access {sub, tenant_id?, typ=access, iss, iat, exp}）。
func Encode(payload map[string]any) (string, error) {
	key, issuer, ttl, err := jwtEnv()
	if err != nil {
		return "", err
	}
	now := time.Now().Unix()
	full := make(map[string]any, len(payload)+3)
	for k, v := range payload {
		full[k] = v
	}
	full["iat"] = now
	full["exp"] = now + int64(ttl)
	full["iss"] = issuer

	header, err := json.Marshal(map[string]string{"alg": "HS256", "typ": "JWT"})
	if err != nil {
		return "", err
	}
	body, err := json.Marshal(full)
	if err != nil {
		return "", err
	}
	signingInput := b64(header) + "." + b64(body)
	return signingInput + "." + hs256(key, signingInput), nil
}

// Verify 验签 HS256：结构三段、header alg 恒等 HS256（alg 混淆拒）、签名 HMAC 比对、
// exp/iss 全验；**不读 aud**（lab 分叉）。任一不过返回带语义的错误（上层统一转 401）。
func Verify(token string) (map[string]any, error) {
	key, issuer, _, err := jwtEnv()
	if err != nil {
		return nil, err
	}
	parts := splitSegments(token)
	if len(parts) != 3 {
		return nil, errors.New("token 段数非 3")
	}
	headerRaw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, fmt.Errorf("header 解码失败: %w", err)
	}
	var header struct {
		Alg string `json:"alg"`
	}
	if err := json.Unmarshal(headerRaw, &header); err != nil {
		return nil, fmt.Errorf("header 解析失败: %w", err)
	}
	if header.Alg != "HS256" {
		return nil, fmt.Errorf("alg %q 非法（仅接受 HS256）", header.Alg)
	}
	signingInput := parts[0] + "." + parts[1]
	if !hmac.Equal([]byte(hs256(key, signingInput)), []byte(parts[2])) {
		return nil, errors.New("签名校验失败")
	}
	payloadRaw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("payload 解码失败: %w", err)
	}
	var claims map[string]any
	if err := json.Unmarshal(payloadRaw, &claims); err != nil {
		return nil, fmt.Errorf("payload 解析失败: %w", err)
	}
	if exp, ok := numField(claims, "exp"); ok && time.Now().Unix() >= int64(exp) {
		return nil, errors.New("token 已过期")
	}
	if iss, _ := claims["iss"].(string); iss != issuer {
		return nil, fmt.Errorf("iss %q 不符", iss)
	}
	return claims, nil
}

func splitSegments(tok string) []string {
	segs := []string{}
	cur := []rune{}
	for _, r := range tok {
		if r == '.' {
			segs = append(segs, string(cur))
			cur = []rune{}
			continue
		}
		cur = append(cur, r)
	}
	return append(segs, string(cur))
}

// numField 取数值 claim（json 反序列化数字落 float64）。
func numField(claims map[string]any, key string) (float64, bool) {
	v, ok := claims[key].(float64)
	return v, ok
}
