package service

// sample_plane_service.go — 样品面两服务（REQ-2026-004 Task 10）：
// Samples（M03.F03.I01-I05）+ TestRecords（M03.F03.I06-I11）——rails 同名
// service 1:1 镜像。
//
// - samples：tenant 收口；receiptId FK 校验（不存在 404——rails RecordNotFound
//   镜像）；ext jsonb DTO 端 null→{}；update strict 指针语义（rails .compact——
//   非 nil 即写，含 receiptId 除外：UPDATE_FIELDS 不含 receiptId 不可变）；
//   updateExt 独立端点 ext 必填整体替换；list 排序 created_at desc, sample_code
// - test-records：4 键必填 400；tenant 收口；sampleId 过滤（rails 仅此一过滤）；
//   update strict 全列（含 verdict）；setVerdict 独立改判；list 排序
//   updated_at desc, id asc

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"strings"

	"gorm.io/gorm"

	familyerrors "lab-management-system-go/internal/impl/errors"
)

// newPrefixedID <prefix>-<uuid v4>（rails SecureRandom.uuid 镜像）。
func newPrefixedID(prefix string) (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%s-%x-%x-%x-%x-%x", prefix,
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

// ---- M03.F03 samples ----

// SampleService 样品面（tenant 收口）。
type SampleService struct {
	DB *gorm.DB
}

// NewSampleService 构造。
func NewSampleService(db *gorm.DB) *SampleService {
	return &SampleService{DB: db}
}

const samplesTable = "samples"

// dtoSample ext jsonb 回读（null → {}）。
func dtoSample(row map[string]any) map[string]any {
	ext := map[string]any{}
	switch raw := row["ext"].(type) {
	case []byte:
		_ = json.Unmarshal(raw, &ext)
	case string:
		_ = json.Unmarshal([]byte(raw), &ext)
	}
	return map[string]any{
		"id": row["id"], "receiptId": row["receipt_id"],
		"sampleCode": row["sample_code"], "sampleName": row["sample_name"],
		"model": row["model"], "specification": row["specification"],
		"grade": row["grade"], "brand": row["brand"],
		"manufacturer": row["manufacturer"], "structuralPart": row["structural_part"],
		"representQuantity": row["represent_quantity"], "sampleQuantity": row["sample_quantity"],
		"batchNumber": row["batch_number"], "supplyUnit": row["supply_unit"],
		"arrivalDate": row["arrival_date"], "samplingDate": row["sampling_date"],
		"curingCondition": row["curing_condition"], "age": row["age"],
		"ext": ext, "remark": row["remark"], "tenantId": row["tenant_id"],
		"createdAt": row["created_at"], "updatedAt": row["updated_at"],
	}
}

// findSample tenant+id 寻址（未知 → 404）。
func (s *SampleService) findSample(tenantID, id string) (map[string]any, error) {
	var rows []map[string]any
	if err := s.DB.Table(samplesTable).
		Where("tenant_id = ? AND id = ?", tenantID, id).Find(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, familyerrors.ErrNotFound("Sample not found: " + id)
	}
	return rows[0], nil
}

// ListSamples receiptId + keyword 过滤（信封在 handler——pageSize 缺省 20）。
func (s *SampleService) ListSamples(tenantID, receiptID, keyword string) ([]map[string]any, error) {
	q := s.DB.Table(samplesTable).Where("tenant_id = ?", tenantID)
	if receiptID != "" {
		q = q.Where("receipt_id = ?", receiptID)
	}
	if keyword != "" {
		kw := "%" + strings.ToLower(keyword) + "%"
		q = q.Where("LOWER(sample_code) LIKE ? OR LOWER(sample_name) LIKE ?", kw, kw)
	}
	var rows []map[string]any
	if err := q.Order("created_at desc, sample_code asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		out = append(out, dtoSample(r))
	}
	return out, nil
}

// GetSample 详情。
func (s *SampleService) GetSample(tenantID, id string) (map[string]any, error) {
	row, err := s.findSample(tenantID, id)
	if err != nil {
		return nil, err
	}
	return dtoSample(row), nil
}

// CreateSampleInput create 载荷。
type CreateSampleInput struct {
	TenantID, ReceiptID, SampleCode string
	SampleName                      *string
	Model, Specification            *string
	Grade, Brand, Manufacturer      *string
	StructuralPart                  *string
	RepresentQuantity               *string
	SampleQuantity, BatchNumber     *string
	SupplyUnit                      *string
	ArrivalDate, SamplingDate       *string
	CuringCondition, Age            *string
	Ext                             *map[string]string
	Remark                          *string
}

// CreateSample 新建（receipt 须存在于本租户，否则 404；ext 缺省 {}）。
func (s *SampleService) CreateSample(in CreateSampleInput) (map[string]any, error) {
	var n int64
	if err := s.DB.Table("sample_receipts").
		Where("tenant_id = ? AND id = ?", in.TenantID, in.ReceiptID).
		Count(&n).Error; err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, familyerrors.ErrNotFound("Receipt not found: " + in.ReceiptID)
	}
	id, err := newPrefixedID("S")
	if err != nil {
		return nil, err
	}
	now := NowISO()
	ext := "{}"
	if in.Ext != nil {
		b, mErr := json.Marshal(*in.Ext)
		if mErr != nil {
			return nil, mErr
		}
		ext = string(b)
	}
	row := map[string]any{
		"id": id, "receipt_id": in.ReceiptID, "sample_code": in.SampleCode,
		"sample_name": in.SampleName, "model": in.Model, "specification": in.Specification,
		"grade": in.Grade, "brand": in.Brand, "manufacturer": in.Manufacturer,
		"structural_part": in.StructuralPart, "represent_quantity": in.RepresentQuantity,
		"sample_quantity": in.SampleQuantity, "batch_number": in.BatchNumber,
		"supply_unit": in.SupplyUnit, "arrival_date": in.ArrivalDate,
		"sampling_date": in.SamplingDate, "curing_condition": in.CuringCondition,
		"age": in.Age, "ext": ext, "remark": in.Remark,
		"tenant_id": in.TenantID, "created_at": now, "updated_at": now,
	}
	if err := s.DB.Table(samplesTable).Create(row).Error; err != nil {
		return nil, err
	}
	return dtoSample(row), nil
}

// UpdateSample PATCH（strict 指针语义——rails .compact 非 nil 即写；
// receiptId 不可变；ext 非 nil 即写）。
func (s *SampleService) UpdateSample(tenantID, id string,
	sampleName, model, specification, grade, brand, manufacturer,
	structuralPart, representQuantity, sampleQuantity, batchNumber,
	supplyUnit, arrivalDate, samplingDate, curingCondition, age *string,
	ext *map[string]string, remark *string) (map[string]any, error) {
	if _, err := s.findSample(tenantID, id); err != nil {
		return nil, err
	}
	updates := map[string]any{"updated_at": NowISO()}
	for col, v := range map[string]*string{
		"sample_name": sampleName, "model": model, "specification": specification,
		"grade": grade, "brand": brand, "manufacturer": manufacturer,
		"structural_part": structuralPart, "represent_quantity": representQuantity,
		"sample_quantity": sampleQuantity, "batch_number": batchNumber,
		"supply_unit": supplyUnit, "arrival_date": arrivalDate,
		"sampling_date": samplingDate, "curing_condition": curingCondition,
		"age": age, "remark": remark,
	} {
		if v != nil {
			updates[col] = *v
		}
	}
	if ext != nil {
		b, err := json.Marshal(*ext)
		if err != nil {
			return nil, err
		}
		updates["ext"] = string(b)
	}
	if err := s.DB.Table(samplesTable).
		Where("tenant_id = ? AND id = ?", tenantID, id).Updates(updates).Error; err != nil {
		return nil, err
	}
	return s.GetSample(tenantID, id)
}

// UpdateSampleExt ext 整体替换（rails：req 或 req['ext'] 缺失 → ArgumentError 400）。
func (s *SampleService) UpdateSampleExt(tenantID, id string, ext *map[string]string) (map[string]any, error) {
	if ext == nil {
		return nil, familyerrors.ErrBadRequest("ext is required")
	}
	if _, err := s.findSample(tenantID, id); err != nil {
		return nil, err
	}
	b, err := json.Marshal(*ext)
	if err != nil {
		return nil, err
	}
	if err := s.DB.Table(samplesTable).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		Updates(map[string]any{"ext": string(b), "updated_at": NowISO()}).Error; err != nil {
		return nil, err
	}
	return s.GetSample(tenantID, id)
}

// DeleteSample 删除（未知 → 404；检测记录 FK RESTRICT 由 DB 拒）。
func (s *SampleService) DeleteSample(tenantID, id string) error {
	if _, err := s.findSample(tenantID, id); err != nil {
		return err
	}
	return s.DB.Table(samplesTable).
		Where("tenant_id = ? AND id = ?", tenantID, id).Delete(nil).Error
}

// ---- M03.F03 test-records ----

// TestRecordService 检测记录面（tenant 收口）。
type TestRecordService struct {
	DB *gorm.DB
}

// NewTestRecordService 构造。
func NewTestRecordService(db *gorm.DB) *TestRecordService {
	return &TestRecordService{DB: db}
}

const testRecordsTable = "test_records"

// dtoTestRecord camel 映射。
func dtoTestRecord(row map[string]any) map[string]any {
	return map[string]any{
		"id": row["id"], "tenantId": row["tenant_id"], "sampleId": row["sample_id"],
		"parameterCode": row["parameter_code"], "standardCode": row["standard_code"],
		"requirementCode": row["requirement_code"], "requirement": row["requirement"],
		"result": row["result"], "verdict": row["verdict"],
		"createdAt": row["created_at"], "updatedAt": row["updated_at"],
	}
}

// findRecord tenant+id 寻址（未知 → 404）。
func (s *TestRecordService) findRecord(tenantID, id string) (map[string]any, error) {
	var rows []map[string]any
	if err := s.DB.Table(testRecordsTable).
		Where("tenant_id = ? AND id = ?", tenantID, id).Find(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, familyerrors.ErrNotFound("TestRecord not found: " + id)
	}
	return rows[0], nil
}

// ListTestRecords sampleId 过滤（信封在 handler；rails 仅 sampleId 一过滤）。
func (s *TestRecordService) ListTestRecords(tenantID, sampleID string) ([]map[string]any, error) {
	q := s.DB.Table(testRecordsTable).Where("tenant_id = ?", tenantID)
	if sampleID != "" {
		q = q.Where("sample_id = ?", sampleID)
	}
	var rows []map[string]any
	if err := q.Order("updated_at desc, id asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		out = append(out, dtoTestRecord(r))
	}
	return out, nil
}

// GetTestRecord 详情。
func (s *TestRecordService) GetTestRecord(tenantID, id string) (map[string]any, error) {
	row, err := s.findRecord(tenantID, id)
	if err != nil {
		return nil, err
	}
	return dtoTestRecord(row), nil
}

// CreateTestRecordInput create 载荷。
type CreateTestRecordInput struct {
	TenantID, SampleID, ParameterCode string
	StandardCode, RequirementCode     *string
	Requirement, Result               string
	Verdict                           *string
}

// CreateTestRecord 新建（sampleId/parameterCode/requirement/result 必填 400）。
func (s *TestRecordService) CreateTestRecord(in CreateTestRecordInput) (map[string]any, error) {
	if in.SampleID == "" || in.ParameterCode == "" || in.Requirement == "" || in.Result == "" {
		return nil, familyerrors.ErrBadRequest(
			"sampleId, parameterCode, requirement and result are required")
	}
	id, err := newPrefixedID("TR")
	if err != nil {
		return nil, err
	}
	now := NowISO()
	row := map[string]any{
		"id": id, "tenant_id": in.TenantID, "sample_id": in.SampleID,
		"parameter_code": in.ParameterCode, "standard_code": in.StandardCode,
		"requirement_code": in.RequirementCode, "requirement": in.Requirement,
		"result": in.Result, "verdict": in.Verdict,
		"created_at": now, "updated_at": now,
	}
	if err := s.DB.Table(testRecordsTable).Create(row).Error; err != nil {
		return nil, err
	}
	return dtoTestRecord(row), nil
}

// UpdateTestRecord PATCH（strict 指针语义——rails .compact；含 verdict）。
func (s *TestRecordService) UpdateTestRecord(tenantID, id string,
	parameterCode, standardCode, requirementCode, requirement,
	result, verdict *string) (map[string]any, error) {
	if _, err := s.findRecord(tenantID, id); err != nil {
		return nil, err
	}
	updates := map[string]any{"updated_at": NowISO()}
	for col, v := range map[string]*string{
		"parameter_code": parameterCode, "standard_code": standardCode,
		"requirement_code": requirementCode, "requirement": requirement,
		"result": result, "verdict": verdict,
	} {
		if v != nil {
			updates[col] = *v
		}
	}
	if err := s.DB.Table(testRecordsTable).
		Where("tenant_id = ? AND id = ?", tenantID, id).Updates(updates).Error; err != nil {
		return nil, err
	}
	return s.GetTestRecord(tenantID, id)
}

// SetVerdict 人工改判（独立动词 PATCH /{id}/verdict）。
func (s *TestRecordService) SetVerdict(tenantID, id, verdict string) (map[string]any, error) {
	if _, err := s.findRecord(tenantID, id); err != nil {
		return nil, err
	}
	if err := s.DB.Table(testRecordsTable).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		Updates(map[string]any{"verdict": verdict, "updated_at": NowISO()}).Error; err != nil {
		return nil, err
	}
	return s.GetTestRecord(tenantID, id)
}

// DeleteTestRecord 删除（未知 → 404）。
func (s *TestRecordService) DeleteTestRecord(tenantID, id string) error {
	if _, err := s.findRecord(tenantID, id); err != nil {
		return err
	}
	return s.DB.Table(testRecordsTable).
		Where("tenant_id = ? AND id = ?", tenantID, id).Delete(nil).Error
}
