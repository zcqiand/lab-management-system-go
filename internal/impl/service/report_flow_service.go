package service

// report_flow_service.go — 报告流程状态机 M03.F01-F08，7 阶段全 act 模式
// （REQ-2026-004 Task 11；rails Receipts::ReportFlowService 镜像，2026-09-17
// 收敛版：POST /receipts/<stage>/act，body.action={submit|return|withdraw}）。
//
// action 范围因 stage 而异：
// - receiving/assigning/data-entry：{submit, return, withdraw}
// - review/approve/issuance：{submit, return}
// - archived：仅 submit（终态自转移写 audit，语义按 submit 走 5.69）
// per-id try/catch：单条失败出 err 结果不整批 5xx。

import (
	familyerrors "lab-management-system-go/internal/impl/errors"
)

// SubmitNext submit 串行链。
var SubmitNext = map[string]string{
	"receiving": "task_assignment", "task_assignment": "data_entry",
	"data_entry": "review", "review": "approval", "approval": "issuance",
	"issuance": "archived",
}

// ReturnPrev return 回退链。
var ReturnPrev = map[string]string{
	"task_assignment": "receiving", "data_entry": "task_assignment",
	"review": "data_entry", "approval": "review", "issuance": "approval",
	"archived": "issuance",
}

// ReportFlowService act 批量入口。
type ReportFlowService struct {
	Receipts *SampleReceiptService
	TenantID string
}

// NewReportFlowService 构造。
func NewReportFlowService(receipts *SampleReceiptService, tenantID string) *ReportFlowService {
	return &ReportFlowService{Receipts: receipts, TenantID: tenantID}
}

// ActResult 单条结果（FlowActionResult camel 形状）。
type ActResult struct {
	ID         string
	OK         bool
	FlowStatus string
	Message    string
}

// Act 批量执行（operator 契约必填：缺失与空串都 400，先于 per-id 循环整批拒——
// 5.75 SSOT = lab-nextjs act-route.ts）。
func (s *ReportFlowService) Act(stage, action string, ids []string,
	operator string, reason *string) ([]ActResult, error) {
	if operator == "" {
		return nil, familyerrors.ErrBadRequest("operator is required")
	}
	out := make([]ActResult, 0, len(ids))
	for _, id := range ids {
		if stage == "archived" {
			out = append(out, s.actArchivedOne(id, action, operator, reason))
		} else {
			out = append(out, s.actStageOne(stage, id, action, operator, reason))
		}
	}
	return out, nil
}

// targetOf 动作目标态（withdraw 仅 receiving 自转移）。
func targetOf(current, action string) string {
	switch action {
	case "submit":
		return SubmitNext[current]
	case "return":
		return ReturnPrev[current]
	case "withdraw":
		if current == "receiving" {
			return "receiving"
		}
		return ""
	}
	return ""
}

func (s *ReportFlowService) actStageOne(stage, id, action, operator string,
	reason *string) ActResult {
	dto, err := s.Receipts.GetReceipt(s.TenantID, id)
	if err != nil {
		return ActResult{ID: id, Message: err.Error()}
	}
	current, _ := dto["flowStatus"].(string)
	if current != stage {
		return ActResult{ID: id, Message: "Stage mismatch: requires " + stage +
			" but is " + current}
	}
	target := targetOf(current, action)
	if target == "" {
		return ActResult{ID: id, Message: "Invalid transition from " + current +
			" with " + action}
	}
	if _, err := s.Receipts.TransitionTo(s.TenantID, id, FlowMove{
		From: current, To: target, Action: action, Operator: operator, Reason: reason}); err != nil {
		return ActResult{ID: id, Message: err.Error()}
	}
	return ActResult{ID: id, OK: true, FlowStatus: target}
}

// actArchivedOne archived 终态：仅 submit；自转移写 audit（reason 缺省
// "archived: post-archive audit"，语义按 submit——lastSubmittedBy 写操作人）。
func (s *ReportFlowService) actArchivedOne(id, action, operator string, reason *string) ActResult {
	dto, err := s.Receipts.GetReceipt(s.TenantID, id)
	if err != nil {
		return ActResult{ID: id, Message: err.Error()}
	}
	if current, _ := dto["flowStatus"].(string); current != "archived" {
		return ActResult{ID: id, Message: "Stage mismatch: requires archived but is " + current}
	}
	if action != "submit" {
		return ActResult{ID: id, Message: "Action not allowed: archived accepts only submit but got " + action}
	}
	auditReason := reason
	if auditReason == nil {
		r := "archived: post-archive audit"
		auditReason = &r
	}
	if _, err := s.Receipts.TransitionTo(s.TenantID, id, FlowMove{
		From: "archived", To: "archived", Action: "submit", Operator: operator,
		Reason: auditReason}); err != nil {
		return ActResult{ID: id, Message: err.Error()}
	}
	return ActResult{ID: id, OK: true, FlowStatus: "archived"}
}
