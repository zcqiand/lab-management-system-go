package service

// business_plane_service.go — 商务面两服务（REQ-2026-004 Task 9）：
// Contracts（M02.F01）+ ReportNames（M06.F07 + 三组 junction）——rails 同名
// service 1:1 镜像。
//
// - contracts：tenant 收口；keyword LIKE contract_code/project_name；status 枚举
//   active/archived（非法 400）；id 前缀 C-（uuid）；create 必填 6 键；
//   update nil-skip（contractCode 不可变）；list 排序 updated_at desc, contract_code
// - report-names：code 主键平台级；create 同 code upsert merge；extFields jsonb
//   DTO 端 null→[]；update truthy（extFields 非 nil 即写——ruby [] truthy 镜像）/
//   strict sortOrder；三组 junction link upsert 覆盖/unlink 幂等/list 无时间戳

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"strings"

	"gorm.io/gorm"

	familyerrors "lab-management-system-go/internal/impl/errors"
)

// ---- M02.F01 Contracts ----

// ContractStatuses 枚举（business 信封 enum_param! 同集）。
var ContractStatuses = []string{"active", "archived"}

// ContractService 合同面（tenant 收口）。
type ContractService struct {
	DB *gorm.DB
}

// NewContractService 构造。
func NewContractService(db *gorm.DB) *ContractService {
	return &ContractService{DB: db}
}

const contractsTable = "contracts"

// ValidContractStatus status 合法性（active/archived）。
func ValidContractStatus(status string) bool {
	for _, s := range ContractStatuses {
		if s == status {
			return true
		}
	}
	return false
}

func dtoContract(row map[string]any) map[string]any {
	return map[string]any{
		"id": row["id"], "contractCode": row["contract_code"],
		"clientUnit": row["client_unit"], "projectName": row["project_name"],
		"projectLocation": row["project_location"], "constructionUnit": row["construction_unit"],
		"inspectionSpecialtyCode": row["inspection_specialty_code"],
		"buildingUnit":            row["building_unit"], "supervisorUnit": row["supervisor_unit"],
		"inspectionPerson": row["inspection_person"], "inspectionPhone": row["inspection_phone"],
		"witnessUnit": row["witness_unit"], "witness": row["witness"],
		"witnessPhone": row["witness_phone"], "contactPerson": row["contact_person"],
		"contactPhone": row["contact_phone"], "entrustedDate": row["entrusted_date"],
		"status": row["status"], "tenantId": row["tenant_id"],
		"createdAt": row["created_at"], "updatedAt": row["updated_at"],
	}
}

// findContract tenant+id 寻址（未知 → 404）。
func (s *ContractService) findContract(tenantID, id string) (map[string]any, error) {
	var rows []map[string]any
	if err := s.DB.Table(contractsTable).
		Where("tenant_id = ? AND id = ?", tenantID, id).Find(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, familyerrors.ErrNotFound("Contract not found")
	}
	return rows[0], nil
}

// ListContracts keyword + status 过滤（信封在 handler——pageSize 缺省 20）。
func (s *ContractService) ListContracts(tenantID, keyword, status string) ([]map[string]any, error) {
	q := s.DB.Table(contractsTable).Where("tenant_id = ?", tenantID)
	if strings.TrimSpace(keyword) != "" {
		kw := "%" + strings.ToLower(keyword) + "%"
		q = q.Where("LOWER(contract_code) LIKE ? OR LOWER(project_name) LIKE ?", kw, kw)
	}
	if strings.TrimSpace(status) != "" {
		q = q.Where("status = ?", status)
	}
	var rows []map[string]any
	if err := q.Order("updated_at desc, contract_code asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		out = append(out, dtoContract(r))
	}
	return out, nil
}

// GetContract 详情。
func (s *ContractService) GetContract(tenantID, id string) (map[string]any, error) {
	row, err := s.findContract(tenantID, id)
	if err != nil {
		return nil, err
	}
	return dtoContract(row), nil
}

// CreateContractInput create 载荷（tenant 由 token claim 注入）。
type CreateContractInput struct {
	TenantID                string
	ContractCode            string
	ClientUnit, ProjectName string
	ProjectLocation         *string
	ConstructionUnit        string
	InspectionSpecialtyCode *string
	BuildingUnit            *string
	SupervisorUnit          *string
	InspectionPerson        *string
	InspectionPhone         *string
	WitnessUnit, Witness    string
	WitnessPhone            *string
	ContactPerson           *string
	ContactPhone            *string
	EntrustedDate           *string
	Status                  string
}

// CreateContract 新建（必填 6 键；status 非法 400；id C-<uuid>）。
func (s *ContractService) CreateContract(in CreateContractInput) (map[string]any, error) {
	if in.ContractCode == "" || in.ClientUnit == "" || in.ProjectName == "" ||
		in.ConstructionUnit == "" || in.WitnessUnit == "" || in.Witness == "" {
		return nil, familyerrors.ErrBadRequest(
			"contractCode/clientUnit/projectName/constructionUnit/witnessUnit/witness are required")
	}
	status := in.Status
	if status == "" {
		status = "active"
	}
	if !ValidContractStatus(status) {
		return nil, familyerrors.ErrBadRequest("unexpected status '" + status + "'")
	}
	id, err := newContractID()
	if err != nil {
		return nil, err
	}
	now := NowISO()
	row := map[string]any{
		"id": id, "contract_code": in.ContractCode,
		"client_unit": in.ClientUnit, "project_name": in.ProjectName,
		"project_location": in.ProjectLocation, "construction_unit": in.ConstructionUnit,
		"inspection_specialty_code": in.InspectionSpecialtyCode,
		"building_unit":             in.BuildingUnit, "supervisor_unit": in.SupervisorUnit,
		"inspection_person": in.InspectionPerson, "inspection_phone": in.InspectionPhone,
		"witness_unit": in.WitnessUnit, "witness": in.Witness,
		"witness_phone": in.WitnessPhone, "contact_person": in.ContactPerson,
		"contact_phone": in.ContactPhone, "entrusted_date": in.EntrustedDate,
		"status": status, "tenant_id": in.TenantID,
		"created_at": now, "updated_at": now,
	}
	if err := s.DB.Table(contractsTable).Create(row).Error; err != nil {
		return nil, err
	}
	return dtoContract(row), nil
}

// UpdateContract PATCH（nil 跳过；status 非法 400；contractCode 不可变）。
func (s *ContractService) UpdateContract(tenantID, id string,
	clientUnit, projectName string, projectLocation *string,
	constructionUnit string, inspectionSpecialtyCode *string,
	buildingUnit, supervisorUnit *string, inspectionPerson, inspectionPhone *string,
	witnessUnit, witness string, witnessPhone *string,
	contactPerson, contactPhone *string, entrustedDate *string, status string) (map[string]any, error) {
	if _, err := s.findContract(tenantID, id); err != nil {
		return nil, err
	}
	updates := map[string]any{"updated_at": NowISO()}
	for col, v := range map[string]string{
		"client_unit": clientUnit, "project_name": projectName,
		"construction_unit": constructionUnit, "witness_unit": witnessUnit,
		"witness": witness,
	} {
		if v != "" {
			updates[col] = v
		}
	}
	for col, v := range map[string]*string{
		"project_location": projectLocation, "inspection_specialty_code": inspectionSpecialtyCode,
		"building_unit": buildingUnit, "supervisor_unit": supervisorUnit,
		"inspection_person": inspectionPerson, "inspection_phone": inspectionPhone,
		"witness_phone": witnessPhone, "contact_person": contactPerson,
		"contact_phone": contactPhone, "entrusted_date": entrustedDate,
	} {
		if v != nil {
			updates[col] = *v
		}
	}
	if status != "" {
		if !ValidContractStatus(status) {
			return nil, familyerrors.ErrBadRequest("unexpected status '" + status + "'")
		}
		updates["status"] = status
	}
	if err := s.DB.Table(contractsTable).
		Where("tenant_id = ? AND id = ?", tenantID, id).Updates(updates).Error; err != nil {
		return nil, err
	}
	return s.GetContract(tenantID, id)
}

// DeleteContract 删除（未知 → 404；接样 FK RESTRICT 由 DB 拒）。
func (s *ContractService) DeleteContract(tenantID, id string) error {
	if _, err := s.findContract(tenantID, id); err != nil {
		return err
	}
	return s.DB.Table(contractsTable).
		Where("tenant_id = ? AND id = ?", tenantID, id).Delete(nil).Error
}

// newContractID C-<uuid v4>（rails SecureRandom.uuid 镜像）。
func newContractID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("C-%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

// ---- M06.F07 ReportNames ----

// ReportNameService 报告名称面（平台级）。
type ReportNameService struct {
	DB *gorm.DB
}

// NewReportNameService 构造。
func NewReportNameService(db *gorm.DB) *ReportNameService {
	return &ReportNameService{DB: db}
}

const reportNamesTable = "inspection_report_names"

// dtoReportName extFields jsonb 回读（null → []）。
func dtoReportName(row map[string]any) map[string]any {
	ext := []any{}
	switch raw := row["ext_fields"].(type) {
	case []byte:
		_ = json.Unmarshal(raw, &ext)
	case string:
		_ = json.Unmarshal([]byte(raw), &ext)
	}
	return map[string]any{
		"code": row["code"], "name": row["name"], "fullName": row["full_name"],
		"templatePath": row["template_path"], "summaryName": row["summary_name"],
		"extFields": ext, "description": row["description"],
		"sortOrder": row["sort_order"], "createdAt": row["created_at"], "updatedAt": row["updated_at"],
	}
}

// findReportName 按 code 寻址（未知 → 404）。
func (s *ReportNameService) findReportName(code string) (map[string]any, error) {
	var rows []map[string]any
	if err := s.DB.Table(reportNamesTable).Where("code = ?", code).Find(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, familyerrors.ErrNotFound("ReportName not found")
	}
	return rows[0], nil
}

// ListReportNames keyword 过滤（信封在 handler）。
func (s *ReportNameService) ListReportNames(keyword string) ([]map[string]any, error) {
	var rows []map[string]any
	q := keywordLike(s.DB.Table(reportNamesTable), keyword).Order("sort_order asc, code asc")
	if err := q.Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		out = append(out, dtoReportName(r))
	}
	return out, nil
}

// GetReportName 详情。
func (s *ReportNameService) GetReportName(code string) (map[string]any, error) {
	row, err := s.findReportName(code)
	if err != nil {
		return nil, err
	}
	return dtoReportName(row), nil
}

// CreateReportNameInput create 载荷。
type CreateReportNameInput struct {
	Code, Name, FullName, TemplatePath, SummaryName string
	ExtFields                                       []any
	Description                                     string
	SortOrder                                       *int32
}

// CreateReportName 新建（code/name 必填；同 code upsert merge；extFields 缺省 []）。
func (s *ReportNameService) CreateReportName(in CreateReportNameInput) (map[string]any, error) {
	if in.Code == "" || in.Name == "" {
		return nil, familyerrors.ErrBadRequest("code and name are required")
	}
	ext := "[]"
	if in.ExtFields != nil {
		b, err := json.Marshal(in.ExtFields)
		if err != nil {
			return nil, err
		}
		ext = string(b)
	}
	now := NowISO()
	row := map[string]any{
		"code": in.Code, "name": in.Name,
		"template_path": strPtrOrNil(in.TemplatePath), "summary_name": strPtrOrNil(in.SummaryName),
		"ext_fields": ext, "description": strPtrOrNil(in.Description),
		"sort_order": derefOr(in.SortOrder, 0), "created_at": now, "updated_at": now,
	}
	if in.FullName != "" {
		row["full_name"] = in.FullName
	}
	if _, err := s.findReportName(in.Code); err == nil {
		// 同 code merge：整行覆盖（含 created_at——springboot save=merge 镜像）
		if err := s.DB.Table(reportNamesTable).Where("code = ?", in.Code).
			Updates(row).Error; err != nil {
			return nil, err
		}
	} else if err := s.DB.Table(reportNamesTable).Create(row).Error; err != nil {
		return nil, err
	}
	return dtoReportName(row), nil
}

// UpdateReportName PATCH（truthy 字符串；extFields 非 nil 即写；strict sortOrder）。
func (s *ReportNameService) UpdateReportName(code, name, fullName, templatePath,
	summaryName string, extFields []any, description string, sortOrder *int32) (map[string]any, error) {
	if _, err := s.findReportName(code); err != nil {
		return nil, err
	}
	updates := map[string]any{"updated_at": NowISO()}
	for col, v := range map[string]string{
		"name": name, "full_name": fullName, "template_path": templatePath,
		"summary_name": summaryName, "description": description,
	} {
		if v != "" {
			updates[col] = v
		}
	}
	if extFields != nil {
		b, err := json.Marshal(extFields)
		if err != nil {
			return nil, err
		}
		updates["ext_fields"] = string(b)
	}
	if sortOrder != nil {
		updates["sort_order"] = *sortOrder
	}
	if err := s.DB.Table(reportNamesTable).Where("code = ?", code).
		Updates(updates).Error; err != nil {
		return nil, err
	}
	return s.GetReportName(code)
}

// DeleteReportName 删除（未知 → 404）。
func (s *ReportNameService) DeleteReportName(code string) error {
	if _, err := s.findReportName(code); err != nil {
		return err
	}
	return s.DB.Table(reportNamesTable).Where("code = ?", code).Delete(nil).Error
}

// ---- M06.F07/M06.F04 report-name junctions ----

// LinkObjectReportName upsert（缺键 400；重复 link 覆盖 remark）。
func (s *ReportNameService) LinkObjectReportName(objectCode, reportNameCode string, remark *string) error {
	if err := requireNonBlank(objectCode, reportNameCode); err != nil {
		return err
	}
	return upsertJunctionTable(s.DB, "inspection_object_report_names",
		map[string]any{"inspection_object_code": objectCode, "report_name_code": reportNameCode},
		map[string]any{"remark": remark})
}

// UnlinkObjectReportName 幂等删除。
func (s *ReportNameService) UnlinkObjectReportName(objectCode, reportNameCode string) error {
	return s.DB.Table("inspection_object_report_names").
		Where("inspection_object_code = ? AND report_name_code = ?", objectCode, reportNameCode).
		Delete(nil).Error
}

// ListObjectReportNameLinks 双 code 过滤。
func (s *ReportNameService) ListObjectReportNameLinks(objectCode, reportNameCode string) ([]map[string]any, error) {
	q := s.DB.Table("inspection_object_report_names")
	if strings.TrimSpace(objectCode) != "" {
		q = q.Where("inspection_object_code = ?", objectCode)
	}
	if strings.TrimSpace(reportNameCode) != "" {
		q = q.Where("report_name_code = ?", reportNameCode)
	}
	var rows []map[string]any
	if err := q.Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		out = append(out, map[string]any{
			"inspectionObjectCode": r["inspection_object_code"],
			"reportNameCode":       r["report_name_code"], "remark": r["remark"],
		})
	}
	return out, nil
}

// LinkReportNameStandard upsert（role 必填，复合 PK 三列）。
func (s *ReportNameService) LinkReportNameStandard(reportNameCode, standardCode, role string, remark *string) error {
	if err := requireNonBlank(reportNameCode, standardCode, role); err != nil {
		return err
	}
	return upsertJunctionTable(s.DB, "inspection_report_name_standards",
		map[string]any{"report_name_code": reportNameCode,
			"inspection_standard_code": standardCode, "role": role},
		map[string]any{"remark": remark})
}

// UnlinkReportNameStandard 幂等删除。
func (s *ReportNameService) UnlinkReportNameStandard(reportNameCode, standardCode, role string) error {
	return s.DB.Table("inspection_report_name_standards").
		Where("report_name_code = ? AND inspection_standard_code = ? AND role = ?",
			reportNameCode, standardCode, role).
		Delete(nil).Error
}

// ListReportNameStandardLinks reportNameCode + role 过滤。
func (s *ReportNameService) ListReportNameStandardLinks(reportNameCode, role string) ([]map[string]any, error) {
	q := s.DB.Table("inspection_report_name_standards")
	if strings.TrimSpace(reportNameCode) != "" {
		q = q.Where("report_name_code = ?", reportNameCode)
	}
	if strings.TrimSpace(role) != "" {
		q = q.Where("role = ?", role)
	}
	var rows []map[string]any
	if err := q.Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		out = append(out, map[string]any{
			"reportNameCode":         r["report_name_code"],
			"inspectionStandardCode": r["inspection_standard_code"],
			"role":                   r["role"], "remark": r["remark"],
		})
	}
	return out, nil
}

// LinkReportNameParameter upsert。
func (s *ReportNameService) LinkReportNameParameter(reportNameCode, parameterCode string, remark *string) error {
	if err := requireNonBlank(reportNameCode, parameterCode); err != nil {
		return err
	}
	return upsertJunctionTable(s.DB, "inspection_report_name_parameters",
		map[string]any{"report_name_code": reportNameCode,
			"inspection_parameter_code": parameterCode},
		map[string]any{"remark": remark})
}

// UnlinkReportNameParameter 幂等删除。
func (s *ReportNameService) UnlinkReportNameParameter(reportNameCode, parameterCode string) error {
	return s.DB.Table("inspection_report_name_parameters").
		Where("report_name_code = ? AND inspection_parameter_code = ?", reportNameCode, parameterCode).
		Delete(nil).Error
}

// ListReportNameParameterLinks 双 code 过滤。
func (s *ReportNameService) ListReportNameParameterLinks(reportNameCode, parameterCode string) ([]map[string]any, error) {
	q := s.DB.Table("inspection_report_name_parameters")
	if strings.TrimSpace(reportNameCode) != "" {
		q = q.Where("report_name_code = ?", reportNameCode)
	}
	if strings.TrimSpace(parameterCode) != "" {
		q = q.Where("inspection_parameter_code = ?", parameterCode)
	}
	var rows []map[string]any
	if err := q.Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		out = append(out, map[string]any{
			"reportNameCode":          r["report_name_code"],
			"inspectionParameterCode": r["inspection_parameter_code"], "remark": r["remark"],
		})
	}
	return out, nil
}
