package auth

// guard.go — Bearer 守卫（镜像 rails lab jwt_guard concern，REQ-2026-004 lab 分叉）：
// 无/坏 token → 401 + WWW-Authenticate；tenant_id claim 缺省回退 directory 默认
// 租户 TENANT-001（代码常量；禁 LAB_SAAS_DEFAULT_TENANT_ID env 兜底——rails
// jwt_guard L38 明令）。lab 契约无 {tenantId} 路径段（单租户），无路径租户校验。

import (
	"context"
	"net/http"
	"strings"

	familyerrors "lab-management-system-go/internal/impl/errors"
)

// DirectoryDefaultTenant directory 默认租户（种子字面量，单租户家族口径）。
const DirectoryDefaultTenant = "TENANT-001"

type claimsKeyType struct{}

// Authenticate Bearer 守卫核心：无/坏 token → 401 + WWW-Authenticate 并返回 false；
// 合法 → claims（缺 tenant_id 回退 TENANT-001）。生成 handler 入口直接调用——
// oapi-codegen 无中间件挂点，rails include JwtGuard 的等价物是每个 protected
// handler 首行 s.auth(w, r)。
func Authenticate(w http.ResponseWriter, r *http.Request) (map[string]any, bool) {
	raw := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if !strings.HasPrefix(raw, prefix) {
		unauthorized(w)
		return nil, false
	}
	claims, err := Verify(strings.TrimPrefix(raw, prefix))
	if err != nil {
		unauthorized(w)
		return nil, false
	}
	// typ 必须是 access（rails token_service verify! 默认 TYP_ACCESS 镜像）——
	// 不验则 7 天 refresh token（内嵌 saas 凭据）可打通全部受保护 op。
	if claims["typ"] != TypAccess {
		unauthorized(w)
		return nil, false
	}
	if _, ok := claims["tenant_id"].(string); !ok || claims["tenant_id"] == "" {
		claims["tenant_id"] = DirectoryDefaultTenant
	}
	return claims, true
}

// Guard Bearer 守卫中间件（Authenticate + claims 进 context）。
func Guard(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := Authenticate(w, r)
		if !ok {
			return
		}
		next(w, r.WithContext(WithClaims(r.Context(), claims)))
	}
}

// WithClaims 把 claims 挂进 context（ClaimsFrom 的写入侧）。
func WithClaims(ctx context.Context, claims map[string]any) context.Context {
	return context.WithValue(ctx, claimsKeyType{}, claims)
}

func unauthorized(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", `Bearer realm="api"`)
	familyerrors.Write(w, familyerrors.ErrInvalidCredentials())
}

// ClaimsFrom 取守卫注入的 claims（未过守卫时 ok=false）。
func ClaimsFrom(ctx context.Context) (map[string]any, bool) {
	claims, ok := ctx.Value(claimsKeyType{}).(map[string]any)
	return claims, ok
}

// WriteError 错误落响应薄包装（测试桩与本包自用；handler 侧直接用 errors.Write）。
func WriteError(w http.ResponseWriter, err error) {
	familyerrors.Write(w, err)
}
