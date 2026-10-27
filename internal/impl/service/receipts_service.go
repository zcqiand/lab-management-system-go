package service

// receipts_service.go — 接样单面（REQ-2026-004 Task 11）：Receipts
// （M03.F01.I01-I06 CRUD/history）+ AssignTask（M03.F02.I01）+ ReportFlow 七段
// act 状态机（M03.F01.I08 等七行）——rails Receipts::SampleReceiptService +
// ReportFlowService + ReceiptQueries 1:1 镜像。
//
// - tenant 收口；contractId FK（未知 404）；起步态 receiving/[]/""；
//   update strict 指针（contractId 不可变、3 个 jsonb 数组非 nil 即写——ruby
//   Array(v || []) 镜像）；list 排序 updated_at desc, commission_code asc；
//   5.57 三态 filter（not_yet/submitted jsonb 谓词 native SQL）
// - 状态机 7+1 态：submit 串行链 + receiving 自 withdraw；submit 写
//   last_submitted_by / withdraw 清空 / return 保留；archived 仅 submit
//   自转移写 audit；per-id err 结果不整批 5xx；operator 缺失整批 400

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	familyerrors "lab-management-system-go/internal/impl/errors"
)

// ---- M03.F01-F09 receipts ----

// FlowStatuses 状态机全集（7 段 + completed 终态；enum_param! 同集）。
var FlowStatuses = []string{"receiving", "task_assignment", "data_entry",
	"review", "approval", "issuance", "archived", "completed"}

// ValidFlowStatus flowStatus 过滤参数合法性。
func ValidFlowStatus(s string) bool {
	for _, v := range FlowStatuses {
		if v == s {
			return true
		}
	}
	return false
}

// SampleReceiptService 接样单面（tenant 收口）。
type SampleReceiptService struct {
	DB *gorm.DB
}

// NewSampleReceiptService 构造。
func NewSampleReceiptService(db *gorm.DB) *SampleReceiptService {
	return &SampleReceiptService{DB: db}
}

const receiptsTable = "sample_receipts"

// jsonArrField jsonb 数组回读（null → []）。
func jsonArrField(raw any) []any {
	arr := []any{}
	switch v := raw.(type) {
	case []byte:
		_ = json.Unmarshal(v, &arr)
	case string:
		_ = json.Unmarshal([]byte(v), &arr)
	}
	return arr
}

// dtoReceipt camel DTO（rails ReceiptFields 三段 merge 镜像）。
func dtoReceipt(row map[string]any) map[string]any {
	var issuedAt any
	if t, ok := row["issued_at"].(time.Time); ok {
		issuedAt = t.UTC().Format("2006-01-02T15:04:05+00:00")
	}
	return map[string]any{
		"id": row["id"], "contractId": row["contract_id"],
		"commissionCode": row["commission_code"], "commissionDate": row["commission_date"],
		"commissionRegisterCode": row["commission_register_code"],
		"commissionRegisterDate": row["commission_register_date"],
		"categoryCode":           row["category_code"], "projectName": row["project_name"],
		"clientUnit": row["client_unit"], "buildingUnit": row["building_unit"],
		"supervisorUnit": row["supervisor_unit"], "constructionUnit": row["construction_unit"],
		"witnessUnit": row["witness_unit"], "samplingLocation": row["sampling_location"],
		"witness": row["witness"], "witnessPhone": row["witness_phone"],
		"inspector": row["inspector"], "inspectorPhone": row["inspector_phone"],
		"receivedBy": row["received_by"], "sampleSource": row["sample_source"],
		"testCategory": row["test_category"], "testEnvironment": row["test_environment"],
		"mainEquipment": row["main_equipment"], "testOperator": row["test_operator"],
		"testStartDate": row["test_start_date"], "testEndDate": row["test_end_date"],
		"originalRecordNo": row["original_record_no"], "remark": row["remark"],
		"judgmentBasis":   jsonArrField(row["judgment_basis"]),
		"testingBasis":    jsonArrField(row["testing_basis"]),
		"testParameters":  jsonArrField(row["test_parameters"]),
		"flowStatus":      row["flow_status"],
		"flowHistory":     jsonArrField(row["flow_history"]),
		"lastSubmittedBy": row["last_submitted_by"],
		"assigneeId":      row["assignee_id"], "assigneeName": row["assignee_name"],
		"plannedTestDate": row["planned_test_date"],
		"reportCode":      row["report_code"], "reportDate": row["report_date"],
		"conclusion": row["conclusion"], "result": row["result"],
		"issuedAt": issuedAt, "tenantId": row["tenant_id"],
		"createdAt": row["created_at"], "updatedAt": row["updated_at"],
	}
}

// findReceipt tenant+id 寻址（未知 → 404）。
func (s *SampleReceiptService) findReceipt(tenantID, id string) (map[string]any, error) {
	var rows []map[string]any
	if err := s.DB.Table(receiptsTable).
		Where("tenant_id = ? AND id = ?", tenantID, id).Find(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, familyerrors.ErrNotFound("Receipt not found: " + id)
	}
	return rows[0], nil
}

// ListReceipts contractId/flowStatus/keyword/filter 过滤（信封在 handler——
// pageSize 缺省 20）。
func (s *SampleReceiptService) ListReceipts(tenantID, contractID, flowStatus,
	keyword, filter string) ([]map[string]any, error) {
	if filter == "not_yet" || filter == "submitted" {
		return s.filterThreeState(tenantID, contractID, flowStatus, keyword, filter)
	}
	q := s.DB.Table(receiptsTable).Where("tenant_id = ?", tenantID)
	if contractID != "" {
		q = q.Where("contract_id = ?", contractID)
	}
	if flowStatus != "" {
		q = q.Where("flow_status = ?", flowStatus)
	}
	if keyword != "" {
		kw := "%" + strings.ToLower(keyword) + "%"
		q = q.Where("LOWER(commission_code) LIKE ? OR LOWER(project_name) LIKE ?", kw, kw)
	}
	var rows []map[string]any
	if err := q.Order("updated_at desc, commission_code asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		out = append(out, dtoReceipt(r))
	}
	return out, nil
}

// filterThreeState 三态 filter（5.57；rails ReceiptQueries native SQL 谓词镜像）：
// not_yet = 停在本环节（无 stage 时 = 无流转记录新单）；submitted = 已 submit 至
// 下一环节（无 stage 时 = 有流转记录且 last_submitted_by 非空）。
func (s *SampleReceiptService) filterThreeState(tenantID, contractID, stage,
	keyword, filter string) ([]map[string]any, error) {
	conds := `(@tenant_id = '' OR tenant_id = @tenant_id)
		AND (@contract_id = '' OR contract_id = @contract_id)
		AND (@keyword = '' OR LOWER(commission_code) LIKE '%' || LOWER(@keyword) || '%'
		     OR LOWER(project_name) LIKE '%' || LOWER(@keyword) || '%')
		AND (
		  (@filter = 'not_yet' AND (
		     (@stage <> '' AND flow_status = @stage)
		     OR (@stage = '' AND jsonb_array_length(flow_history) = 0)))
		  OR (@filter = 'submitted' AND (
		     (@stage <> '' AND flow_status <> @stage
		        AND EXISTS (SELECT 1 FROM jsonb_array_elements(flow_history) h
		                    WHERE h ->> 'action' = 'submit' AND h ->> 'from' = @stage))
		     OR (@stage = '' AND jsonb_array_length(flow_history) > 0
		           AND last_submitted_by IS NOT NULL))))`
	var rows []map[string]any
	if err := s.DB.Table(receiptsTable).Where(conds, map[string]any{
		"tenant_id": tenantID, "contract_id": contractID,
		"keyword": keyword, "stage": stage, "filter": filter,
	}).Order("updated_at desc, commission_code asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		out = append(out, dtoReceipt(r))
	}
	return out, nil
}

// GetReceipt 详情。
func (s *SampleReceiptService) GetReceipt(tenantID, id string) (map[string]any, error) {
	row, err := s.findReceipt(tenantID, id)
	if err != nil {
		return nil, err
	}
	return dtoReceipt(row), nil
}

// ReceiptHistory flow_history 展开（未知 → 404；null → []）。
func (s *SampleReceiptService) ReceiptHistory(tenantID, id string) ([]any, error) {
	row, err := s.findReceipt(tenantID, id)
	if err != nil {
		return nil, err
	}
	return jsonArrField(row["flow_history"]), nil
}

// CreateReceiptInput create 载荷（jsonb 三数组 + 标量；tenant 由 token 注入）。
type CreateReceiptInput struct {
	TenantID, ContractID      string
	CommissionCode            string
	CommissionDate            string
	CommissionRegisterCode    *string
	CommissionRegisterDate    *string
	CategoryCode              string
	ProjectName               *string
	ClientUnit                *string
	BuildingUnit              *string
	SupervisorUnit            *string
	ConstructionUnit          *string
	WitnessUnit               *string
	SamplingLocation          *string
	Witness, WitnessPhone     *string
	Inspector, InspectorPhone *string
	ReceivedBy                string
	SampleSource              string
	TestCategory              string
	TestEnvironment           *string
	MainEquipment             *string
	TestOperator              *string
	TestStartDate             *string
	TestEndDate               *string
	OriginalRecordNo          *string
	JudgmentBasis             []string
	TestingBasis              []string
	TestParameters            []string
	Remark                    *string
}

// marshalJSONArr jsonb 数组列写值（nil → "[]"——rails Array(v || []) 起步镜像）。
func marshalJSONArr(arr []string) (string, error) {
	if arr == nil {
		return "[]", nil
	}
	b, err := json.Marshal(arr)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// CreateReceipt 新建（contract 须存在于本租户，否则 404；起步 receiving/[]/""）。
func (s *SampleReceiptService) CreateReceipt(in CreateReceiptInput) (map[string]any, error) {
	var n int64
	if err := s.DB.Table("contracts").
		Where("tenant_id = ? AND id = ?", in.TenantID, in.ContractID).
		Count(&n).Error; err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, familyerrors.ErrNotFound("Contract not found: " + in.ContractID)
	}
	id, err := newPrefixedID("R")
	if err != nil {
		return nil, err
	}
	now := NowISO()
	jb, err := marshalJSONArr(in.JudgmentBasis)
	if err != nil {
		return nil, err
	}
	tb, err := marshalJSONArr(in.TestingBasis)
	if err != nil {
		return nil, err
	}
	tp, err := marshalJSONArr(in.TestParameters)
	if err != nil {
		return nil, err
	}
	row := map[string]any{
		"id": id, "contract_id": in.ContractID,
		"commission_code": in.CommissionCode, "commission_date": in.CommissionDate,
		"commission_register_code": in.CommissionRegisterCode,
		"commission_register_date": in.CommissionRegisterDate,
		"category_code":            in.CategoryCode, "project_name": in.ProjectName,
		"client_unit": in.ClientUnit, "building_unit": in.BuildingUnit,
		"supervisor_unit": in.SupervisorUnit, "construction_unit": in.ConstructionUnit,
		"witness_unit": in.WitnessUnit, "sampling_location": in.SamplingLocation,
		"witness": in.Witness, "witness_phone": in.WitnessPhone,
		"inspector": in.Inspector, "inspector_phone": in.InspectorPhone,
		"received_by": in.ReceivedBy, "sample_source": in.SampleSource,
		"test_category": in.TestCategory, "test_environment": in.TestEnvironment,
		"main_equipment": in.MainEquipment, "test_operator": in.TestOperator,
		"test_start_date": in.TestStartDate, "test_end_date": in.TestEndDate,
		"original_record_no": in.OriginalRecordNo,
		"judgment_basis":     jb, "testing_basis": tb, "test_parameters": tp,
		"remark": in.Remark, "ext": "{}",
		"flow_status": "receiving", "flow_history": "[]", "result": "",
		"tenant_id": in.TenantID, "created_at": now, "updated_at": now,
	}
	if err := s.DB.Table(receiptsTable).Create(row).Error; err != nil {
		return nil, err
	}
	return dtoReceipt(row), nil
}

// UpdateReceipt PATCH（strict 指针；contractId 不可变；jsonb 数组非 nil 即写）。
func (s *SampleReceiptService) UpdateReceipt(tenantID, id string, fields map[string]*string,
	judgmentBasis, testingBasis, testParameters []string) (map[string]any, error) {
	if _, err := s.findReceipt(tenantID, id); err != nil {
		return nil, err
	}
	updates := map[string]any{"updated_at": NowISO()}
	for col, v := range fields {
		if v != nil {
			updates[col] = *v
		}
	}
	for col, arr := range map[string][]string{
		"judgment_basis": judgmentBasis, "testing_basis": testingBasis,
		"test_parameters": testParameters,
	} {
		if arr != nil {
			b, err := json.Marshal(arr)
			if err != nil {
				return nil, err
			}
			updates[col] = string(b)
		}
	}
	if err := s.DB.Table(receiptsTable).
		Where("tenant_id = ? AND id = ?", tenantID, id).Updates(updates).Error; err != nil {
		return nil, err
	}
	return s.GetReceipt(tenantID, id)
}

// DeleteReceipt 删除（未知 → 404；samples FK RESTRICT 由 DB 拒）。
func (s *SampleReceiptService) DeleteReceipt(tenantID, id string) error {
	if _, err := s.findReceipt(tenantID, id); err != nil {
		return err
	}
	return s.DB.Table(receiptsTable).
		Where("tenant_id = ? AND id = ?", tenantID, id).Delete(nil).Error
}

// AssignTask M03.F02 任务分配：字段按需更新；仅 receiving 推进 task_assignment
// 并写 history（action 字面量 submit，operator 记 assigneeName）。
func (s *SampleReceiptService) AssignTask(tenantID, id string,
	assigneeID, assigneeName, plannedTestDate *string) (map[string]any, error) {
	row, err := s.findReceipt(tenantID, id)
	if err != nil {
		return nil, err
	}
	updates := map[string]any{"updated_at": NowISO()}
	for col, v := range map[string]*string{
		"assignee_id": assigneeID, "assignee_name": assigneeName,
		"planned_test_date": plannedTestDate,
	} {
		if v != nil {
			updates[col] = *v
		}
	}
	if row["flow_status"] == "receiving" {
		updates["flow_status"] = "task_assignment"
		hist := jsonArrField(row["flow_history"])
		hist = append(hist, map[string]any{
			"action": "submit", "operator": assigneeName,
			"from": "receiving", "to": "task_assignment",
			"reason": "M03.F02 任务分配", "at": NowISO()})
		b, mErr := json.Marshal(hist)
		if mErr != nil {
			return nil, mErr
		}
		updates["flow_history"] = string(b)
	}
	if err := s.DB.Table(receiptsTable).
		Where("tenant_id = ? AND id = ?", tenantID, id).Updates(updates).Error; err != nil {
		return nil, err
	}
	return s.GetReceipt(tenantID, id)
}

// FlowMove 一次状态迁移的语义（rails transition_to **move 镜像）。
type FlowMove struct {
	From, To, Action string
	Operator         string
	Reason           *string
}

// TransitionTo 状态迁移 + last_submitted_by 语义（5.69：submit 写操作人 /
// withdraw 清空 / return 保留）+ history 记 action 真值（5.75）。
func (s *SampleReceiptService) TransitionTo(tenantID, id string, move FlowMove) (map[string]any, error) {
	row, err := s.findReceipt(tenantID, id)
	if err != nil {
		return nil, err
	}
	if row["flow_status"] != move.From {
		return nil, familyerrors.ErrBadRequest(fmt.Sprintf(
			"Receipt %s not in expected stage %s but %s", id, move.From, row["flow_status"]))
	}
	hist := jsonArrField(row["flow_history"])
	entry := map[string]any{
		"action": move.Action, "from": move.From, "to": move.To,
		"operator": nilIfEmpty(move.Operator), "reason": move.Reason, "at": NowISO()}
	hist = append(hist, entry)
	b, mErr := json.Marshal(hist)
	if mErr != nil {
		return nil, mErr
	}
	updates := map[string]any{
		"flow_status": move.To, "flow_history": string(b), "updated_at": NowISO()}
	switch move.Action {
	case "submit":
		updates["last_submitted_by"] = move.Operator
	case "withdraw":
		updates["last_submitted_by"] = nil
	}
	if err := s.DB.Table(receiptsTable).
		Where("tenant_id = ? AND id = ?", tenantID, id).Updates(updates).Error; err != nil {
		return nil, err
	}
	return s.GetReceipt(tenantID, id)
}

// nilIfEmpty 空串归 null（operator 可缺省语义镜像）。
func nilIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}
