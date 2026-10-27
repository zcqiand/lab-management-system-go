package api

// receipts_impl.go — 接样单/流程/汇总 handler（REQ-2026-004 Task 11）。
//
// 17 端点：receipts CRUD+history 6（M03.F01.I01-I06）+ assignTask（M03.F02.I01）
// + act×7（M03.F01.I08/F02.I05/F03.I12/F05.I07/F06.I05/F07.I05/F08.I05）+
// summary 2（M05.F01.I01/I06）+ frontend-bind snapshot 占位 404。
// 薄层装配：守卫 → service → 信封/状态码；act per-id err 结果仍 200。

import (
	"net/http"

	genapi "lab-management-system-go/internal/gen/api"
	familyerrors "lab-management-system-go/internal/impl/errors"
	"lab-management-system-go/internal/impl/render"
	"lab-management-system-go/internal/impl/service"
)

// derefArr 可选字符串数组 → 裸切片（nil 保持 nil）。
func derefArr(v *[]string) []string {
	if v == nil {
		return nil
	}
	return *v
}

// ---- M03.F01 receipts ----

func (s *Server) rcSvc() *service.SampleReceiptService {
	return service.NewSampleReceiptService(s.DB)
}

// rcTenant Bearer claim 的 tenant_id。
func (s *Server) rcTenant(w http.ResponseWriter, r *http.Request) (string, bool) {
	claims, ok := s.authenticated(w, r)
	if !ok {
		return "", false
	}
	return claims["tenant_id"].(string), true
}

func (s *Server) ReceiptsListReceipts(w http.ResponseWriter, r *http.Request, p genapi.ReceiptsListReceiptsParams) {
	tenantID, ok := s.rcTenant(w, r)
	if !ok {
		return
	}
	flowStatus := enumStr(p.FlowStatus)
	if flowStatus != "" && !service.ValidFlowStatus(flowStatus) {
		familyerrors.Write(w, familyerrors.ErrBadRequest("unexpected flowStatus '"+flowStatus+"'"))
		return
	}
	items, err := s.rcSvc().ListReceipts(tenantID, derefStr(p.ContractId),
		flowStatus, derefStr(p.Keyword), derefStr(p.Filter))
	if err != nil {
		familyerrors.Write(w, err)
		return
	}
	page, pageSize := 1, 20
	if p.Page != nil {
		page = int(*p.Page)
	}
	if p.PageSize != nil {
		pageSize = int(*p.PageSize)
	}
	render.WriteJSON(w, http.StatusOK, map[string]any{
		"items": items, "page": page, "pageSize": pageSize, "total": len(items)})
}

func (s *Server) ReceiptsGetReceipt(w http.ResponseWriter, r *http.Request, id string) {
	tenantID, ok := s.rcTenant(w, r)
	if !ok {
		return
	}
	dto, err := s.rcSvc().GetReceipt(tenantID, id)
	if err != nil {
		familyerrors.Write(w, err)
		return
	}
	render.WriteJSON(w, http.StatusOK, dto)
}

func (s *Server) ReceiptsCreateReceipt(w http.ResponseWriter, r *http.Request) {
	var body genapi.CreateSampleReceiptRequest
	decodeBodyInto(r, &body)
	tenantID, ok := s.rcTenant(w, r)
	if !ok {
		return
	}
	in := service.CreateReceiptInput{
		TenantID: tenantID, ContractID: body.ContractId,
		CommissionCode: body.CommissionCode, CommissionDate: body.CommissionDate,
		CommissionRegisterCode: body.CommissionRegisterCode,
		CommissionRegisterDate: body.CommissionRegisterDate,
		CategoryCode:           body.CategoryCode, ProjectName: body.ProjectName,
		ClientUnit: body.ClientUnit, BuildingUnit: body.BuildingUnit,
		SupervisorUnit: body.SupervisorUnit, ConstructionUnit: body.ConstructionUnit,
		WitnessUnit: body.WitnessUnit, SamplingLocation: body.SamplingLocation,
		Witness: body.Witness, WitnessPhone: body.WitnessPhone,
		Inspector: body.Inspector, InspectorPhone: body.InspectorPhone,
		ReceivedBy: body.ReceivedBy, SampleSource: body.SampleSource,
		TestCategory: body.TestCategory, TestEnvironment: body.TestEnvironment,
		MainEquipment: body.MainEquipment, TestOperator: body.TestOperator,
		TestStartDate: body.TestStartDate, TestEndDate: body.TestEndDate,
		OriginalRecordNo: body.OriginalRecordNo, Remark: body.Remark,
	}
	in.JudgmentBasis = derefArr(body.JudgmentBasis)
	in.TestingBasis = derefArr(body.TestingBasis)
	in.TestParameters = derefArr(body.TestParameters)
	dto, err := s.rcSvc().CreateReceipt(in)
	if err != nil {
		familyerrors.Write(w, err)
		return
	}
	render.WriteJSON(w, http.StatusOK, dto)
}

func (s *Server) ReceiptsUpdateReceipt(w http.ResponseWriter, r *http.Request, id string) {
	var body genapi.UpdateSampleReceiptRequest
	decodeBodyInto(r, &body)
	tenantID, ok := s.rcTenant(w, r)
	if !ok {
		return
	}
	// contractId 不可变（rails LIST_FIELDS 不含 contractId 镜像）
	fields := map[string]*string{
		"commission_code": body.CommissionCode, "commission_date": body.CommissionDate,
		"commission_register_code": body.CommissionRegisterCode,
		"commission_register_date": body.CommissionRegisterDate,
		"category_code":            body.CategoryCode, "project_name": body.ProjectName,
		"client_unit": body.ClientUnit, "building_unit": body.BuildingUnit,
		"supervisor_unit": body.SupervisorUnit, "construction_unit": body.ConstructionUnit,
		"witness_unit": body.WitnessUnit, "sampling_location": body.SamplingLocation,
		"witness": body.Witness, "witness_phone": body.WitnessPhone,
		"inspector": body.Inspector, "inspector_phone": body.InspectorPhone,
		"received_by": body.ReceivedBy, "sample_source": body.SampleSource,
		"test_category": body.TestCategory, "test_environment": body.TestEnvironment,
		"main_equipment": body.MainEquipment, "test_operator": body.TestOperator,
		"test_start_date": body.TestStartDate, "test_end_date": body.TestEndDate,
		"original_record_no": body.OriginalRecordNo, "remark": body.Remark,
	}
	dto, err := s.rcSvc().UpdateReceipt(tenantID, id, fields,
		derefArr(body.JudgmentBasis), derefArr(body.TestingBasis), derefArr(body.TestParameters))
	if err != nil {
		familyerrors.Write(w, err)
		return
	}
	render.WriteJSON(w, http.StatusOK, dto)
}

func (s *Server) ReceiptsDeleteReceipt(w http.ResponseWriter, r *http.Request, id string) {
	tenantID, ok := s.rcTenant(w, r)
	if !ok {
		return
	}
	if err := s.rcSvc().DeleteReceipt(tenantID, id); err != nil {
		familyerrors.Write(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) ReceiptsGetReceiptHistory(w http.ResponseWriter, r *http.Request, id string) {
	tenantID, ok := s.rcTenant(w, r)
	if !ok {
		return
	}
	hist, err := s.rcSvc().ReceiptHistory(tenantID, id)
	if err != nil {
		familyerrors.Write(w, err)
		return
	}
	render.WriteJSON(w, http.StatusOK, hist)
}

func (s *Server) ReceiptsAssignTask(w http.ResponseWriter, r *http.Request, id string) {
	var body genapi.AssignTaskRequest
	decodeBodyInto(r, &body)
	tenantID, ok := s.rcTenant(w, r)
	if !ok {
		return
	}
	dto, err := s.rcSvc().AssignTask(tenantID, id,
		body.AssigneeId, body.AssigneeName, body.PlannedTestDate)
	if err != nil {
		familyerrors.Write(w, err)
		return
	}
	render.WriteJSON(w, http.StatusOK, dto)
}

// ---- 七段 act（stage URL 词 → 状态机内部名） ----

func (s *Server) actFlow(w http.ResponseWriter, r *http.Request, stage string) {
	var body genapi.FlowActionRequest
	decodeBodyInto(r, &body)
	tenantID, ok := s.rcTenant(w, r)
	if !ok {
		return
	}
	flow := service.NewReportFlowService(s.rcSvc(), tenantID)
	results, err := flow.Act(stage, string(body.Action), body.Ids,
		body.Operator, body.Reason)
	if err != nil {
		familyerrors.Write(w, err)
		return
	}
	out := make([]map[string]any, 0, len(results))
	for _, res := range results {
		item := map[string]any{"id": res.ID, "ok": res.OK}
		if res.FlowStatus != "" {
			item["flowStatus"] = res.FlowStatus
		}
		if res.Message != "" {
			item["message"] = res.Message
		}
		out = append(out, item)
	}
	render.WriteJSON(w, http.StatusOK, out)
}

func (s *Server) ReceiptsActFlowReceiving(w http.ResponseWriter, r *http.Request) {
	s.actFlow(w, r, "receiving")
}

func (s *Server) ReceiptsActFlowAssigning(w http.ResponseWriter, r *http.Request) {
	s.actFlow(w, r, "task_assignment")
}

func (s *Server) ReceiptsActFlowDataEntry(w http.ResponseWriter, r *http.Request) {
	s.actFlow(w, r, "data_entry")
}

func (s *Server) ReceiptsActFlowReview(w http.ResponseWriter, r *http.Request) {
	s.actFlow(w, r, "review")
}

func (s *Server) ReceiptsActFlowApprove(w http.ResponseWriter, r *http.Request) {
	s.actFlow(w, r, "approval")
}

func (s *Server) ReceiptsActFlowIssuance(w http.ResponseWriter, r *http.Request) {
	s.actFlow(w, r, "issuance")
}

func (s *Server) ReceiptsActFlowArchived(w http.ResponseWriter, r *http.Request) {
	s.actFlow(w, r, "archived")
}

// ---- M05 summary ----

func (s *Server) SummaryGetReportSummary(w http.ResponseWriter, r *http.Request, p genapi.SummaryGetReportSummaryParams) {
	tenantID, ok := s.rcTenant(w, r)
	if !ok {
		return
	}
	dto, err := service.NewSummaryService(s.DB).ReportSummary(tenantID,
		derefStr(p.CategoryCode), derefStr(p.DateFrom), derefStr(p.DateTo))
	if err != nil {
		familyerrors.Write(w, err)
		return
	}
	render.WriteJSON(w, http.StatusOK, dto)
}

func (s *Server) SummaryGetDashboardStats(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.rcTenant(w, r)
	if !ok {
		return
	}
	dto, err := service.NewSummaryService(s.DB).DashboardStats(tenantID)
	if err != nil {
		familyerrors.Write(w, err)
		return
	}
	// rails 仪表盘是 render json:（键原状，funnel snake_case 不 camel）
	render.WriteRawJSON(w, http.StatusOK, dto)
}

// ---- frontend-bind snapshot（rails 1:1：挂认证，恒 404） ----

func (s *Server) FrontendBindMetaGetFrontendBindSnapshot(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.authenticated(w, r); !ok {
		return
	}
	familyerrors.Write(w, familyerrors.ErrNotFound("resource not found"))
}
