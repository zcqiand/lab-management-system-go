package service

// parameter_plane_service.go — 参数面三服务（REQ-2026-004 Task 8）：
// CalculationMethod（M06.F05）/ ParamInterface（M06.F08）/ TechnicalRequirement
// （M06.F06）—— lab-springboot + rails 同名 service 1:1 镜像。
//
// - 计算方法：复合双键，平台级；create 同 PK upsert merge（springboot save=merge
//   镜像）；缺省 algorithmType=manual / specimenCount=1 / sortOrder=0；list 裸数组
//   双过滤，排序 sort_order, object, parameter；PATCH truthy/strict
// - 参数界面：code/componentPath 必填；config jsonb DTO 端 null→{}；list 信封
//   （handler）；update truthy/sortOrder=0 不写（ruby truthy 镜像）；
//   link/unlink 幂等 204、list-links 短信封（handler）
// - 技术要求：业务三键 + tenant 收口；create 同 PK merge；缺省 numeric/≥/manual/
//   draft；list 裸数组 4 过滤；update nil-skip 全字段

import (
	"encoding/json"
	"strings"

	"gorm.io/gorm"

	familyerrors "lab-management-system-go/internal/impl/errors"
)

// ---- M06.F05 CalculationMethod ----

// CalculationMethodService 计算方法面。
type CalculationMethodService struct {
	DB *gorm.DB
}

// NewCalculationMethodService 构造。
func NewCalculationMethodService(db *gorm.DB) *CalculationMethodService {
	return &CalculationMethodService{DB: db}
}

const calcMethodsTable = "inspection_calculation_methods"

func dtoCalcMethod(row map[string]any) map[string]any {
	return map[string]any{
		"inspectionObjectCode":    row["inspection_object_code"],
		"inspectionParameterCode": row["inspection_parameter_code"],
		"testingStandardCode":     row["testing_standard_code"],
		"reportNameCode":          row["report_name_code"],
		"algorithmType":           row["algorithm_type"],
		"specimenCount":           row["specimen_count"],
		"formula":                 row["formula"],
		"conditions":              row["conditions"],
		"roundingRule":            row["rounding_rule"],
		"remark":                  row["remark"],
		"sortOrder":               row["sort_order"],
		"createdAt":               row["created_at"],
		"updatedAt":               row["updated_at"],
	}
}

// findCalcMethod 复合键寻址（未知 → 404）。
func (s *CalculationMethodService) findCalcMethod(objectCode, parameterCode string) (map[string]any, error) {
	var rows []map[string]any
	err := s.DB.Table(calcMethodsTable).
		Where("inspection_object_code = ? AND inspection_parameter_code = ?", objectCode, parameterCode).
		Find(&rows)
	if err.Error != nil {
		return nil, err.Error
	}
	if len(rows) == 0 {
		return nil, familyerrors.ErrNotFound("CalculationMethod not found")
	}
	return rows[0], nil
}

// ListCalcMethods 双过滤全量（裸数组，排序 sort_order, object, parameter）。
func (s *CalculationMethodService) ListCalcMethods(objectCode, parameterCode string) ([]map[string]any, error) {
	q := s.DB.Table(calcMethodsTable)
	if strings.TrimSpace(objectCode) != "" {
		q = q.Where("inspection_object_code = ?", objectCode)
	}
	if strings.TrimSpace(parameterCode) != "" {
		q = q.Where("inspection_parameter_code = ?", parameterCode)
	}
	var rows []map[string]any
	if err := q.Order("sort_order asc, inspection_object_code asc, inspection_parameter_code asc").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		out = append(out, dtoCalcMethod(r))
	}
	return out, nil
}

// GetCalcMethod 详情。
func (s *CalculationMethodService) GetCalcMethod(objectCode, parameterCode string) (map[string]any, error) {
	row, err := s.findCalcMethod(objectCode, parameterCode)
	if err != nil {
		return nil, err
	}
	return dtoCalcMethod(row), nil
}

// CreateCalcMethodInput create 载荷。
type CreateCalcMethodInput struct {
	InspectionObjectCode, InspectionParameterCode string
	TestingStandardCode, ReportNameCode           *string
	AlgorithmType                                 string
	SpecimenCount, SortOrder                      *int32
	Formula, Conditions, RoundingRule, Remark     *string
}

// CreateCalcMethod 新建（同 PK upsert merge——双键必填，缺省 manual/1/0）。
func (s *CalculationMethodService) CreateCalcMethod(in CreateCalcMethodInput) (map[string]any, error) {
	if in.InspectionObjectCode == "" || in.InspectionParameterCode == "" {
		return nil, familyerrors.ErrBadRequest(
			"inspectionObjectCode and inspectionParameterCode are required")
	}
	algorithmType := in.AlgorithmType
	if algorithmType == "" {
		algorithmType = "manual"
	}
	now := NowISO()
	row := map[string]any{
		"inspection_object_code":    in.InspectionObjectCode,
		"inspection_parameter_code": in.InspectionParameterCode,
		"testing_standard_code":     in.TestingStandardCode,
		"report_name_code":          in.ReportNameCode,
		"algorithm_type":            algorithmType,
		"specimen_count":            derefOr(in.SpecimenCount, 1),
		"formula":                   in.Formula,
		"conditions":                in.Conditions,
		"rounding_rule":             in.RoundingRule,
		"remark":                    in.Remark,
		"sort_order":                derefOr(in.SortOrder, 0),
		"created_at":                now, "updated_at": now,
	}
	if _, err := s.findCalcMethod(in.InspectionObjectCode, in.InspectionParameterCode); err == nil {
		// 同 PK merge：整行覆盖（含 created_at——springboot save=merge 镜像）
		if err := s.DB.Table(calcMethodsTable).
			Where("inspection_object_code = ? AND inspection_parameter_code = ?",
				in.InspectionObjectCode, in.InspectionParameterCode).
			Updates(row).Error; err != nil {
			return nil, err
		}
	} else if err := s.DB.Table(calcMethodsTable).Create(row).Error; err != nil {
		return nil, err
	}
	return dtoCalcMethod(row), nil
}

// UpdateCalcMethod PATCH（truthy 字符串 / strict 数值；updated_at 恒刷新）。
func (s *CalculationMethodService) UpdateCalcMethod(objectCode, parameterCode string,
	testingStandardCode, reportNameCode string, algorithmType string,
	specimenCount, sortOrder *int32, formula, conditions, roundingRule, remark *string) (map[string]any, error) {
	if _, err := s.findCalcMethod(objectCode, parameterCode); err != nil {
		return nil, err
	}
	updates := map[string]any{"updated_at": NowISO()}
	// truthy：非空才写（handler 已把空串折成 ""）
	for col, v := range map[string]string{
		"testing_standard_code": testingStandardCode,
		"report_name_code":      reportNameCode,
		"algorithm_type":        algorithmType,
		"formula":               derefStrVal(formula),
		"conditions":            derefStrVal(conditions),
		"rounding_rule":         derefStrVal(roundingRule),
		"remark":                derefStrVal(remark),
	} {
		if v != "" {
			updates[col] = v
		}
	}
	if specimenCount != nil {
		updates["specimen_count"] = *specimenCount
	}
	if sortOrder != nil {
		updates["sort_order"] = *sortOrder
	}
	if err := s.DB.Table(calcMethodsTable).
		Where("inspection_object_code = ? AND inspection_parameter_code = ?", objectCode, parameterCode).
		Updates(updates).Error; err != nil {
		return nil, err
	}
	return s.GetCalcMethod(objectCode, parameterCode)
}

// DeleteCalcMethod 删除（未知 → 404）。
func (s *CalculationMethodService) DeleteCalcMethod(objectCode, parameterCode string) error {
	if _, err := s.findCalcMethod(objectCode, parameterCode); err != nil {
		return err
	}
	return s.DB.Table(calcMethodsTable).
		Where("inspection_object_code = ? AND inspection_parameter_code = ?", objectCode, parameterCode).
		Delete(nil).Error
}

// ---- M06.F08 ParamInterface ----

// ParamInterfaceService 参数界面面。
type ParamInterfaceService struct {
	DB *gorm.DB
}

// NewParamInterfaceService 构造。
func NewParamInterfaceService(db *gorm.DB) *ParamInterfaceService {
	return &ParamInterfaceService{DB: db}
}

const paramInterfacesTable = "inspection_param_interfaces"

// dtoParamInterface config jsonb 回读（null → {}——deserialize 镜像）。
func dtoParamInterface(row map[string]any) map[string]any {
	return map[string]any{
		"code": row["code"], "name": row["name"], "componentPath": row["component_path"],
		"description": row["description"], "isOfficial": row["is_official"],
		"sortOrder": row["sort_order"], "config": jsonbMap(row["config"]),
		"createdAt": row["created_at"], "updatedAt": row["updated_at"],
	}
}

// jsonbMap jsonb 列回读为对象（nil/空 → {}）。
func jsonbMap(v any) map[string]any {
	out := map[string]any{}
	switch raw := v.(type) {
	case []byte:
		_ = json.Unmarshal(raw, &out)
	case string:
		_ = json.Unmarshal([]byte(raw), &out)
	}
	return out
}

// findParamInterface 按 code 寻址（未知 → 404）。
func (s *ParamInterfaceService) findParamInterface(code string) (map[string]any, error) {
	var rows []map[string]any
	if err := s.DB.Table(paramInterfacesTable).Where("code = ?", code).Find(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, familyerrors.ErrNotFound("ParamInterface not found")
	}
	return rows[0], nil
}

// ListParamInterfaces keyword 过滤（信封在 handler）。
func (s *ParamInterfaceService) ListParamInterfaces(keyword string) ([]map[string]any, error) {
	var rows []map[string]any
	q := keywordLike(s.DB.Table(paramInterfacesTable), keyword).Order("sort_order asc, code asc")
	if err := q.Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		out = append(out, dtoParamInterface(r))
	}
	return out, nil
}

// GetParamInterface 详情。
func (s *ParamInterfaceService) GetParamInterface(code string) (map[string]any, error) {
	row, err := s.findParamInterface(code)
	if err != nil {
		return nil, err
	}
	return dtoParamInterface(row), nil
}

// CreateParamInterfaceInput create 载荷。
type CreateParamInterfaceInput struct {
	Code, Name, ComponentPath, Description string
	IsOfficial                             *bool
	SortOrder                              *int32
	Config                                 map[string]any
}

// CreateParamInterface 新建（code/componentPath 必填；config 缺省 {}）。
func (s *ParamInterfaceService) CreateParamInterface(in CreateParamInterfaceInput) (map[string]any, error) {
	if in.Code == "" || in.ComponentPath == "" {
		return nil, familyerrors.ErrBadRequest("code and componentPath are required")
	}
	config := "{}"
	if in.Config != nil {
		b, err := json.Marshal(in.Config)
		if err != nil {
			return nil, err
		}
		config = string(b)
	}
	now := NowISO()
	row := map[string]any{
		"code": in.Code, "component_path": in.ComponentPath,
		"is_official": in.IsOfficial, "sort_order": derefOr(in.SortOrder, 0),
		"config": config, "created_at": now, "updated_at": now,
	}
	if in.Name != "" {
		row["name"] = in.Name
	}
	if in.Description != "" {
		row["description"] = in.Description
	}
	if err := s.DB.Table(paramInterfacesTable).Create(row).Error; err != nil {
		return nil, err
	}
	return dtoParamInterface(row), nil
}

// UpdateParamInterface PATCH（truthy 字符串 / strict isOfficial / sortOrder 非 0 才写
// ——ruby `if body['sortOrder']` 镜像；config 非 nil 才写）。
func (s *ParamInterfaceService) UpdateParamInterface(code, name, componentPath, description string,
	isOfficial *bool, sortOrder *int32, config map[string]any) (map[string]any, error) {
	if _, err := s.findParamInterface(code); err != nil {
		return nil, err
	}
	updates := map[string]any{"updated_at": NowISO()}
	if name != "" {
		updates["name"] = name
	}
	if componentPath != "" {
		updates["component_path"] = componentPath
	}
	if description != "" {
		updates["description"] = description
	}
	if isOfficial != nil {
		updates["is_official"] = *isOfficial
	}
	if sortOrder != nil && *sortOrder != 0 {
		updates["sort_order"] = *sortOrder
	}
	if config != nil {
		b, err := json.Marshal(config)
		if err != nil {
			return nil, err
		}
		updates["config"] = string(b)
	}
	if err := s.DB.Table(paramInterfacesTable).Where("code = ?", code).
		Updates(updates).Error; err != nil {
		return nil, err
	}
	return s.GetParamInterface(code)
}

// DeleteParamInterface 删除（未知 → 404）。
func (s *ParamInterfaceService) DeleteParamInterface(code string) error {
	if _, err := s.findParamInterface(code); err != nil {
		return err
	}
	return s.DB.Table(paramInterfacesTable).Where("code = ?", code).Delete(nil).Error
}

// ---- M06.F08 parameter ↔ param-interface junction ----

// LinkParamInterface upsert（缺键 400；config jsonb 载荷；重复 link 整行覆盖）。
func (s *ParamInterfaceService) LinkParamInterface(parameterCode, interfaceCode string,
	reportNameCode *string, config map[string]any) error {
	if err := requireNonBlank(parameterCode, interfaceCode); err != nil {
		return err
	}
	var configJSON any
	if config != nil {
		b, err := json.Marshal(config)
		if err != nil {
			return err
		}
		configJSON = string(b)
	}
	return upsertJunctionTable(s.DB, "inspection_param_interface_links",
		map[string]any{"inspection_parameter_code": parameterCode,
			"param_interface_code": interfaceCode},
		map[string]any{"report_name_code": reportNameCode, "config": configJSON})
}

// UnlinkParamInterface 幂等删除。
func (s *ParamInterfaceService) UnlinkParamInterface(parameterCode, interfaceCode string) error {
	return s.DB.Table("inspection_param_interface_links").
		Where("inspection_parameter_code = ? AND param_interface_code = ?", parameterCode, interfaceCode).
		Delete(nil).Error
}

// ListParamInterfaceLinks 双 code 过滤（信封在 handler）。
func (s *ParamInterfaceService) ListParamInterfaceLinks(parameterCode, interfaceCode string) ([]map[string]any, error) {
	q := s.DB.Table("inspection_param_interface_links")
	if strings.TrimSpace(parameterCode) != "" {
		q = q.Where("inspection_parameter_code = ?", parameterCode)
	}
	if strings.TrimSpace(interfaceCode) != "" {
		q = q.Where("param_interface_code = ?", interfaceCode)
	}
	var rows []map[string]any
	if err := q.Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		out = append(out, map[string]any{
			"inspectionParameterCode": r["inspection_parameter_code"],
			"paramInterfaceCode":      r["param_interface_code"],
			"reportNameCode":          r["report_name_code"],
			"config":                  jsonbMap(r["config"]),
		})
	}
	return out, nil
}

// ---- M06.F06 TechnicalRequirement ----

// TechnicalRequirementService 技术要求面（tenant 收口）。
type TechnicalRequirementService struct {
	DB *gorm.DB
}

// NewTechnicalRequirementService 构造。
func NewTechnicalRequirementService(db *gorm.DB) *TechnicalRequirementService {
	return &TechnicalRequirementService{DB: db}
}

const techReqTable = "inspection_technical_requirements"

func dtoTechReq(row map[string]any) map[string]any {
	return map[string]any{
		"tenantId":                row["tenant_id"],
		"inspectionObjectCode":    row["inspection_object_code"],
		"inspectionParameterCode": row["inspection_parameter_code"],
		"judgmentStandardCode":    row["judgment_standard_code"],
		"conditions":              row["conditions"],
		"valueType":               row["value_type"],
		"minValue":                row["min_value"],
		"maxValue":                row["max_value"],
		"targetValue":             row["target_value"],
		"expression":              row["expression"],
		"unit":                    row["unit"],
		"comparison":              row["comparison"],
		"judgmentMode":            row["judgment_mode"],
		"verificationStatus":      row["verification_status"],
		"clause":                  row["clause"],
		"sourcePage":              row["source_page"],
		"sourceHash":              row["source_hash"],
		"brand":                   row["brand"],
		"model":                   row["model"],
		"grade":                   row["grade"],
		"spec":                    row["spec"],
		"sieve":                   row["sieve"],
		"remark":                  row["remark"],
		"sortOrder":               row["sort_order"],
		"createdAt":               row["created_at"],
		"updatedAt":               row["updated_at"],
	}
}

// findTechReq tenant + 三键寻址（未知 → 404）。
func (s *TechnicalRequirementService) findTechReq(tenantID, objectCode, parameterCode, standardCode string) (map[string]any, error) {
	var rows []map[string]any
	err := s.DB.Table(techReqTable).
		Where("tenant_id = ? AND inspection_object_code = ? AND inspection_parameter_code = ? AND judgment_standard_code = ?",
			tenantID, objectCode, parameterCode, standardCode).
		Find(&rows)
	if err.Error != nil {
		return nil, err.Error
	}
	if len(rows) == 0 {
		return nil, familyerrors.ErrNotFound("TechnicalRequirement not found")
	}
	return rows[0], nil
}

// ListTechReqs 4 过滤 + tenant 收口（裸数组，排序 sort_order, object, parameter, standard）。
func (s *TechnicalRequirementService) ListTechReqs(tenantID, objectCode, parameterCode, standardCode, status string) ([]map[string]any, error) {
	q := s.DB.Table(techReqTable).Where("tenant_id = ?", tenantID)
	if strings.TrimSpace(objectCode) != "" {
		q = q.Where("inspection_object_code = ?", objectCode)
	}
	if strings.TrimSpace(parameterCode) != "" {
		q = q.Where("inspection_parameter_code = ?", parameterCode)
	}
	if strings.TrimSpace(standardCode) != "" {
		q = q.Where("judgment_standard_code = ?", standardCode)
	}
	if strings.TrimSpace(status) != "" {
		q = q.Where("verification_status = ?", status)
	}
	var rows []map[string]any
	if err := q.Order("sort_order asc, inspection_object_code asc, inspection_parameter_code asc, judgment_standard_code asc").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		out = append(out, dtoTechReq(r))
	}
	return out, nil
}

// GetTechReq 详情。
func (s *TechnicalRequirementService) GetTechReq(tenantID, objectCode, parameterCode, standardCode string) (map[string]any, error) {
	row, err := s.findTechReq(tenantID, objectCode, parameterCode, standardCode)
	if err != nil {
		return nil, err
	}
	return dtoTechReq(row), nil
}

// CreateTechReqInput create 载荷（tenant 由 token claim 注入）。
type CreateTechReqInput struct {
	TenantID                                                string
	InspectionObjectCode, InspectionParameterCode           string
	JudgmentStandardCode                                    string
	Conditions, TargetValue, Expression, Unit               *string
	ValueType, Comparison, JudgmentMode, VerificationStatus string
	MinValue, MaxValue, SourcePage                          *int32
	Clause, SourceHash, Brand, Model, Grade                 *string
	Spec, Sieve, Remark                                     *string
	SortOrder                                               *int32
}

// CreateTechReq 新建（三键必填；同 PK merge；缺省 numeric/≥/manual/draft）。
func (s *TechnicalRequirementService) CreateTechReq(in CreateTechReqInput) (map[string]any, error) {
	if in.InspectionObjectCode == "" || in.InspectionParameterCode == "" || in.JudgmentStandardCode == "" {
		return nil, familyerrors.ErrBadRequest(
			"inspectionObjectCode/inspectionParameterCode/judgmentStandardCode are required")
	}
	valueType := enumDefault(in.ValueType, "numeric")
	comparison := enumDefault(in.Comparison, "≥")
	judgmentMode := enumDefault(in.JudgmentMode, "manual")
	verificationStatus := enumDefault(in.VerificationStatus, "draft")
	now := NowISO()
	row := map[string]any{
		"tenant_id":                 in.TenantID,
		"inspection_object_code":    in.InspectionObjectCode,
		"inspection_parameter_code": in.InspectionParameterCode,
		"judgment_standard_code":    in.JudgmentStandardCode,
		"conditions":                in.Conditions,
		"value_type":                valueType,
		"min_value":                 in.MinValue,
		"max_value":                 in.MaxValue,
		"target_value":              in.TargetValue,
		"expression":                in.Expression,
		"unit":                      in.Unit,
		"comparison":                comparison,
		"judgment_mode":             judgmentMode,
		"verification_status":       verificationStatus,
		"clause":                    in.Clause,
		"source_page":               in.SourcePage,
		"source_hash":               in.SourceHash,
		"brand":                     in.Brand,
		"model":                     in.Model,
		"grade":                     in.Grade,
		"spec":                      in.Spec,
		"sieve":                     in.Sieve,
		"remark":                    in.Remark,
		"sort_order":                derefOr(in.SortOrder, 0),
		"created_at":                now, "updated_at": now,
	}
	if _, err := s.findTechReq(in.TenantID, in.InspectionObjectCode,
		in.InspectionParameterCode, in.JudgmentStandardCode); err == nil {
		// 同 PK merge（tenant 收口内）
		if err := s.DB.Table(techReqTable).
			Where("tenant_id = ? AND inspection_object_code = ? AND inspection_parameter_code = ? AND judgment_standard_code = ?",
				in.TenantID, in.InspectionObjectCode, in.InspectionParameterCode, in.JudgmentStandardCode).
			Updates(row).Error; err != nil {
			return nil, err
		}
	} else if err := s.DB.Table(techReqTable).Create(row).Error; err != nil {
		return nil, err
	}
	return dtoTechReq(row), nil
}

// UpdateTechReq PATCH（nil 字段跳过全列；updated_at 恒刷新）。
func (s *TechnicalRequirementService) UpdateTechReq(tenantID, objectCode, parameterCode, standardCode string,
	in UpdateTechReqInput) (map[string]any, error) {
	if _, err := s.findTechReq(tenantID, objectCode, parameterCode, standardCode); err != nil {
		return nil, err
	}
	updates := map[string]any{"updated_at": NowISO()}
	for col, v := range map[string]*string{
		"conditions": in.Conditions, "target_value": in.TargetValue,
		"expression": in.Expression, "unit": in.Unit,
		"clause": in.Clause, "source_hash": in.SourceHash,
		"brand": in.Brand, "model": in.Model, "grade": in.Grade,
		"spec": in.Spec, "sieve": in.Sieve, "remark": in.Remark,
	} {
		if v != nil {
			updates[col] = *v
		}
	}
	for col, v := range map[string]*string{
		"value_type": in.ValueType, "comparison": in.Comparison,
		"judgment_mode": in.JudgmentMode, "verification_status": in.VerificationStatus,
	} {
		if v != nil {
			updates[col] = *v
		}
	}
	if in.MinValue != nil {
		updates["min_value"] = *in.MinValue
	}
	if in.MaxValue != nil {
		updates["max_value"] = *in.MaxValue
	}
	if in.SourcePage != nil {
		updates["source_page"] = *in.SourcePage
	}
	if in.SortOrder != nil {
		updates["sort_order"] = *in.SortOrder
	}
	if err := s.DB.Table(techReqTable).
		Where("tenant_id = ? AND inspection_object_code = ? AND inspection_parameter_code = ? AND judgment_standard_code = ?",
			tenantID, objectCode, parameterCode, standardCode).
		Updates(updates).Error; err != nil {
		return nil, err
	}
	return s.GetTechReq(tenantID, objectCode, parameterCode, standardCode)
}

// UpdateTechReqInput update 载荷（全可空，nil 跳过）。
type UpdateTechReqInput struct {
	Conditions, TargetValue, Expression, Unit               *string
	ValueType, Comparison, JudgmentMode, VerificationStatus *string
	MinValue, MaxValue, SourcePage                          *int32
	Clause, SourceHash, Brand, Model, Grade                 *string
	Spec, Sieve, Remark                                     *string
	SortOrder                                               *int32
}

// DeleteTechReq 删除（未知 → 404）。
func (s *TechnicalRequirementService) DeleteTechReq(tenantID, objectCode, parameterCode, standardCode string) error {
	if _, err := s.findTechReq(tenantID, objectCode, parameterCode, standardCode); err != nil {
		return err
	}
	return s.DB.Table(techReqTable).
		Where("tenant_id = ? AND inspection_object_code = ? AND inspection_parameter_code = ? AND judgment_standard_code = ?",
			tenantID, objectCode, parameterCode, standardCode).
		Delete(nil).Error
}

// ---- 小件 ----

// enumDefault 枚举缺省（空串 → default；值类型 string）。
func enumDefault(v string, def string) string {
	if v == "" {
		return def
	}
	return v
}

// derefStrVal 指针解引（nil → 空串，供 truthy 归一）。
func derefStrVal(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}
