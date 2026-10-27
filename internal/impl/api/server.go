// Package api — ServerInterface 实现壳（REQ-2026-004 Task 3 路由壳）。
//
// API 面只认生成物（suite-hard-rules §4）：路由注册由 internal/gen/api 的
// HandlerFromMux 完成，本结构只实现 120 个方法签名；每域实现落地时把对应方法
// 从 notImplemented 壳替换为 service 委派（ktor delegate 同构——handler 只做
// 装配与状态码映射，业务进 service，CLAUDE.md §2）。本文件由接口签名机械生成
// （awk 提取 gen_api.go ServerInterface + Params 类型加 genapi. 限定），
// 域接管后签名不手改。
package api

import (
	"encoding/json"
	"net/http"

	"gorm.io/gorm"

	genapi "lab-management-system-go/internal/gen/api"
	"lab-management-system-go/internal/impl/service"
)

// Server 实现 genapi.ServerInterface。DB 为装配期注入的连接（*gorm.DB；
// 各域实现接管后经 service 使用，路由壳阶段可为 nil 的域不触库）。
// Auth 认证域服务（Task 5 起落地；nil 时 auth handler 不可用——壳阶段语义）。
type Server struct {
	DB   *gorm.DB
	Auth *service.AuthService
}

// 编译期保证 Server 满足生成接口——签名漂移在 build 门即红。
var _ genapi.ServerInterface = (*Server)(nil)

// NewServer 构造路由壳（db 注入给已落地的域；Auth 未装配——壳阶段/纯路由面测试）。
func NewServer(db *gorm.DB) *Server { return &Server{DB: db} }

// NewServerWithAuth 构造带认证域服务的 Server（装配层 main 用）。
func NewServerWithAuth(db *gorm.DB, authSvc *service.AuthService) *Server {
	return &Server{DB: db, Auth: authSvc}
}

func notImplemented(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	_ = json.NewEncoder(w).Encode(map[string]string{"code": "NOT_IMPLEMENTED"})
}

func (s *Server) FrontendBindMetaGetFrontendBindSnapshot(w http.ResponseWriter, r *http.Request) {
	notImplemented(w)
}

func (s *Server) ReceiptsListReceipts(w http.ResponseWriter, r *http.Request, params genapi.ReceiptsListReceiptsParams) {
	notImplemented(w)
}

func (s *Server) ReceiptsCreateReceipt(w http.ResponseWriter, r *http.Request) {
	notImplemented(w)
}

func (s *Server) ReceiptsActFlowApprove(w http.ResponseWriter, r *http.Request) {
	notImplemented(w)
}

func (s *Server) ReceiptsActFlowArchived(w http.ResponseWriter, r *http.Request) {
	notImplemented(w)
}

func (s *Server) ReceiptsActFlowAssigning(w http.ResponseWriter, r *http.Request) {
	notImplemented(w)
}

func (s *Server) ReceiptsActFlowDataEntry(w http.ResponseWriter, r *http.Request) {
	notImplemented(w)
}

func (s *Server) ReceiptsActFlowIssuance(w http.ResponseWriter, r *http.Request) {
	notImplemented(w)
}

func (s *Server) ReceiptsActFlowReceiving(w http.ResponseWriter, r *http.Request) {
	notImplemented(w)
}

func (s *Server) ReceiptsActFlowReview(w http.ResponseWriter, r *http.Request) {
	notImplemented(w)
}

func (s *Server) ReceiptsDeleteReceipt(w http.ResponseWriter, r *http.Request, id string) {
	notImplemented(w)
}

func (s *Server) ReceiptsGetReceipt(w http.ResponseWriter, r *http.Request, id string) {
	notImplemented(w)
}

func (s *Server) ReceiptsUpdateReceipt(w http.ResponseWriter, r *http.Request, id string) {
	notImplemented(w)
}

func (s *Server) ReceiptsGetReceiptHistory(w http.ResponseWriter, r *http.Request, id string) {
	notImplemented(w)
}

func (s *Server) ReceiptsAssignTask(w http.ResponseWriter, r *http.Request, id string) {
	notImplemented(w)
}

func (s *Server) SummaryGetReportSummary(w http.ResponseWriter, r *http.Request, params genapi.SummaryGetReportSummaryParams) {
	notImplemented(w)
}

func (s *Server) SummaryGetDashboardStats(w http.ResponseWriter, r *http.Request) {
	notImplemented(w)
}
