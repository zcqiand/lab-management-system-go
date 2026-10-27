package api

// auth_impl.go — 认证域 handler（REQ-2026-004 Task 5）。
//
// 薄层铁律（CLAUDE.md §2）：handler 只做 body 装配与状态码映射，业务全在
// service.AuthService（rails AuthController 镜像）。匿名放行面：login /
// native-login / refresh / sso/**；logout 与 me/menus/permissions/switch-tenant
// 首行 auth.Authenticate（oapi-codegen 无中间件挂点，rails JwtGuard include 的
// 等价物）。错误形状统一 familyerrors.Write。
//
// 本文件域接管自 server.go 机械壳：签名不手改（编译期 ServerInterface 断言守着）。

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	genapi "lab-management-system-go/internal/gen/api"
	"lab-management-system-go/internal/impl/auth"
	familyerrors "lab-management-system-go/internal/impl/errors"
	"lab-management-system-go/internal/impl/render"
)

// decodeBody 请求体 → 零值兜底（rails body rescue {} 同语义：解析失败按空体走，
// 字段校验在 service 层出 400/401）。
func decodeBody(r *http.Request, dst any) {
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		return
	}
	_ = json.Unmarshal(raw, dst)
}

// authenticated Bearer 守卫薄包装：失败已写 401 响应并返 false。
func (s *Server) authenticated(w http.ResponseWriter, r *http.Request) (map[string]any, bool) {
	return auth.Authenticate(w, r)
}

// requireAuth 服务未装配时防御（装配层保证 auth 域必有 Auth；nil 即编程错误 → 500）。
func (s *Server) requireAuth(w http.ResponseWriter) bool {
	if s.Auth == nil {
		familyerrors.Write(w, fmt.Errorf("auth service not assembled"))
		return false
	}
	return true
}

func (s *Server) AuthLogin(w http.ResponseWriter, r *http.Request) {
	s.loginCommon(w, r)
}

// M01.F05.I06 原生登录：非浏览器密码通道，校验与签发口径与 login 同源。
func (s *Server) AuthNativeLogin(w http.ResponseWriter, r *http.Request) {
	s.loginCommon(w, r)
}

func (s *Server) loginCommon(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w) {
		return
	}
	var body genapi.AuthLoginJSONRequestBody
	decodeBody(r, &body)
	sess, err := s.Auth.Login(r.Context(), body.Username, body.Password)
	if err != nil {
		familyerrors.Write(w, err)
		return
	}
	render.WriteJSON(w, http.StatusOK, sess)
}

// M01.F05.I05 — 无状态 JWT，服务端无 session store，204 无体。
func (s *Server) AuthLogout(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.authenticated(w, r); !ok {
		return
	}
	s.Auth.Logout()
	w.WriteHeader(http.StatusNoContent)
}

// M00.F01.I01 — 当前会话。
func (s *Server) AuthGetCurrentUser(w http.ResponseWriter, r *http.Request) {
	claims, ok := s.authenticated(w, r)
	if !ok {
		return
	}
	out, err := s.Auth.Me(claims)
	if err != nil {
		familyerrors.Write(w, err)
		return
	}
	render.WriteJSON(w, http.StatusOK, out)
}

// M01.F04.I01 — 动态菜单（快照已在 service 侧映射成 camel 形状）。
func (s *Server) AuthGetMenus(w http.ResponseWriter, r *http.Request) {
	claims, ok := s.authenticated(w, r)
	if !ok {
		return
	}
	out, err := s.Auth.Menus(claims)
	if err != nil {
		familyerrors.Write(w, err)
		return
	}
	render.WriteJSON(w, http.StatusOK, out)
}

// M01.F04.I02 — 权限集。
func (s *Server) AuthGetPermissions(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.authenticated(w, r); !ok {
		return
	}
	render.WriteJSON(w, http.StatusOK, s.Auth.Permissions())
}

// M01.F05.I04 — 刷新 token。
func (s *Server) AuthRefresh(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w) {
		return
	}
	var body genapi.AuthRefreshJSONRequestBody
	decodeBody(r, &body)
	sess, err := s.Auth.Refresh(r.Context(), body.RefreshToken)
	if err != nil {
		familyerrors.Write(w, err)
		return
	}
	render.WriteJSON(w, http.StatusOK, sess)
}

// M01.F05.I02 — 跳板语义：直接回 saas 登录页 URL（redirect_uri 缺失 fail-fast 在 service）。
func (s *Server) AuthSsoAuthorize(w http.ResponseWriter, r *http.Request, params genapi.AuthSsoAuthorizeParams) {
	if !s.requireAuth(w) {
		return
	}
	out, err := s.Auth.SsoAuthorize(params.RedirectUri, params.State)
	if err != nil {
		familyerrors.Write(w, err)
		return
	}
	render.WriteJSON(w, http.StatusOK, out)
}

// M01.F05.I03 — code 换 token。
func (s *Server) AuthSsoCallback(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w) {
		return
	}
	var body genapi.AuthSsoCallbackJSONRequestBody
	decodeBody(r, &body)
	sess, err := s.Auth.SsoCallback(r.Context(), body.Code, body.RedirectUri)
	if err != nil {
		familyerrors.Write(w, err)
		return
	}
	render.WriteJSON(w, http.StatusOK, sess)
}

// M00.F02.I01 — 选租户换发。
func (s *Server) AuthSwitchTenant(w http.ResponseWriter, r *http.Request) {
	claims, ok := s.authenticated(w, r)
	if !ok {
		return
	}
	var body genapi.AuthSwitchTenantJSONRequestBody
	decodeBody(r, &body)
	sess, err := s.Auth.SwitchTenant(r.Context(), claims, body.TenantId)
	if err != nil {
		familyerrors.Write(w, err)
		return
	}
	render.WriteJSON(w, http.StatusOK, sess)
}
