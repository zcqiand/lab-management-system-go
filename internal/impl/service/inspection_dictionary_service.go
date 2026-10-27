package service

// inspection_dictionary_service.go — 检测能力字典服务（镜像 rails
// InspectionDictionary::Specialties/Objects/Parameters/Standards +
// InspectionJunctionService，REQ-2026-004 Task 7）。
//
// 四实体 CRUD（业务 code 主键，平台级无 tenant 过滤——M06 功能树口径）：
// - list：keyword（code/name 大小写不敏感 LIKE）+ 各自专有过滤，排序 sort_order, code；
//   信封（page 缺省 1、pageSize 缺省 = items.size）在 handler
// - create：必填键缺失 → 400；缺省值镜像 rails attrs（specialty isOfficial/enabled
//   true；object isOptionalForQualification false；parameter rawName/canonicalName
//   缺省 = name、aliases []、sourceType official；standard status active）
// - update：PATCH 三模式（truthy 字符串非空才写 / strict 非 nil 才写 / key 非 nil 即写），
//   updated_at 恒刷新；未知 code → 404
// - delete：204 由 handler 回；未知 → 404
//
// junction link/unlink/list（4 组 12 端点）：
// - link 关键键 requireNonBlank（空/缺 → 400）；upsert 同 PK 整行覆盖（含 created_at
//   —— springboot merge 语义镜像）；重复 link 幂等 204
// - unlink 幂等：命中才删，缺失静默 no-op 恒 204（REQ-2026-001 四方一致）
// - list 按 code/role 过滤（blank = 不过滤），DTO 不含时间戳

import (
	"encoding/json"
	"strings"

	"gorm.io/gorm"

	familyerrors "lab-management-system-go/internal/impl/errors"
)

// InspectionDictionaryService 字典面。
type InspectionDictionaryService struct {
	DB *gorm.DB
}

// NewInspectionDictionaryService 构造。
func NewInspectionDictionaryService(db *gorm.DB) *InspectionDictionaryService {
	return &InspectionDictionaryService{DB: db}
}

// ---- 共享小件 ----

// keywordLike keyword 过滤（blank = 不过滤）。
func keywordLike(q *gorm.DB, keyword string) *gorm.DB {
	if strings.TrimSpace(keyword) == "" {
		return q
	}
	kw := "%" + strings.ToLower(keyword) + "%"
	return q.Where("LOWER(code) LIKE ? OR LOWER(name) LIKE ?", kw, kw)
}

// requireNonBlank 镜像 junction requireNonBlank（空/缺 → 400）。
func requireNonBlank(values ...string) error {
	for _, v := range values {
		if strings.TrimSpace(v) == "" {
			return familyerrors.ErrBadRequest("junction link fields must be non-blank")
		}
	}
	return nil
}

// findRow 按 code 寻址（唯一行；未知 → 404）。
func (s *InspectionDictionaryService) findRow(table, code string) (map[string]any, error) {
	var rows []map[string]any
	if err := s.DB.Table(table).Where("code = ?", code).Find(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, familyerrors.ErrNotFound("record not found")
	}
	return rows[0], nil
}

// ---- M06.F01 InspectionSpecialty ----

func dtoSpecialty(row map[string]any) map[string]any {
	return map[string]any{
		"code": row["code"], "officialNo": row["official_no"], "name": row["name"],
		"isOfficial": row["is_official"], "enabled": row["enabled"],
		"sortOrder": row["sort_order"], "createdAt": row["created_at"],
		"updatedAt": row["updated_at"],
	}
}

// ListSpecialties keyword 过滤全量。
func (s *InspectionDictionaryService) ListSpecialties(keyword string) ([]map[string]any, error) {
	var rows []map[string]any
	q := keywordLike(s.DB.Table("inspection_specialties"), keyword).
		Order("sort_order asc, code asc")
	if err := q.Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		out = append(out, dtoSpecialty(r))
	}
	return out, nil
}

// CreateSpecialtyInput create 载荷（必填 code/officialNo/name）。
type CreateSpecialtyInput struct {
	Code, OfficialNo, Name string
	IsOfficial, Enabled    *bool
	SortOrder              *int32
}

// CreateSpecialty 新建（必填缺失 → 400）。
func (s *InspectionDictionaryService) CreateSpecialty(in CreateSpecialtyInput) (map[string]any, error) {
	if in.Code == "" || in.OfficialNo == "" || in.Name == "" {
		return nil, familyerrors.ErrBadRequest("code/officialNo/name are required")
	}
	now := NowISO()
	row := map[string]any{
		"code": in.Code, "official_no": in.OfficialNo, "name": in.Name,
		"is_official": derefBool(in.IsOfficial, true), "enabled": derefBool(in.Enabled, true),
		"sort_order": derefOr(in.SortOrder, 0), "created_at": now, "updated_at": now,
	}
	if err := s.DB.Table("inspection_specialties").Create(row).Error; err != nil {
		return nil, err
	}
	return dtoSpecialty(row), nil
}

// UpdateSpecialty PATCH（truthy: officialNo/name；strict: isOfficial/enabled/sortOrder）。
func (s *InspectionDictionaryService) UpdateSpecialty(code string, officialNo, name *string,
	isOfficial, enabled *bool, sortOrder *int32) (map[string]any, error) {
	if _, err := s.findRow("inspection_specialties", code); err != nil {
		return nil, err
	}
	updates := map[string]any{"updated_at": NowISO()}
	if officialNo != nil && *officialNo != "" {
		updates["official_no"] = *officialNo
	}
	if name != nil && *name != "" {
		updates["name"] = *name
	}
	if isOfficial != nil {
		updates["is_official"] = *isOfficial
	}
	if enabled != nil {
		updates["enabled"] = *enabled
	}
	if sortOrder != nil {
		updates["sort_order"] = *sortOrder
	}
	if err := s.DB.Table("inspection_specialties").Where("code = ?", code).
		Updates(updates).Error; err != nil {
		return nil, err
	}
	row, err := s.findRow("inspection_specialties", code)
	if err != nil {
		return nil, err
	}
	return dtoSpecialty(row), nil
}

// DeleteSpecialty 删除（未知 → 404）。
func (s *InspectionDictionaryService) DeleteSpecialty(code string) error {
	if _, err := s.findRow("inspection_specialties", code); err != nil {
		return err
	}
	return s.DB.Table("inspection_specialties").Where("code = ?", code).Delete(nil).Error
}

// ---- M06.F02 InspectionObject ----

func dtoObject(row map[string]any) map[string]any {
	return map[string]any{
		"code": row["code"], "inspectionSpecialtyCode": row["inspection_specialty_code"],
		"sourceProjectNo": row["source_project_no"], "sourceProjectName": row["source_project_name"],
		"name": row["name"], "isOptionalForQualification": row["is_optional_for_qualification"],
		"isOfficial": row["is_official"], "enabled": row["enabled"],
		"sortOrder": row["sort_order"], "createdAt": row["created_at"], "updatedAt": row["updated_at"],
	}
}

// ListObjects keyword + 专项过滤。
func (s *InspectionDictionaryService) ListObjects(keyword, specialtyCode string) ([]map[string]any, error) {
	q := keywordLike(s.DB.Table("inspection_objects"), keyword)
	if strings.TrimSpace(specialtyCode) != "" {
		q = q.Where("inspection_specialty_code = ?", specialtyCode)
	}
	var rows []map[string]any
	if err := q.Order("sort_order asc, code asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		out = append(out, dtoObject(r))
	}
	return out, nil
}

// CreateObjectInput create 载荷（必填 5 键）。
type CreateObjectInput struct {
	Code, InspectionSpecialtyCode, SourceProjectNo, SourceProjectName, Name string
	IsOptionalForQualification, IsOfficial, Enabled                         *bool
	SortOrder                                                               *int32
}

// CreateObject 新建。
func (s *InspectionDictionaryService) CreateObject(in CreateObjectInput) (map[string]any, error) {
	if in.Code == "" || in.InspectionSpecialtyCode == "" || in.SourceProjectNo == "" ||
		in.SourceProjectName == "" || in.Name == "" {
		return nil, familyerrors.ErrBadRequest(
			"code/inspectionSpecialtyCode/sourceProjectNo/sourceProjectName/name are required")
	}
	now := NowISO()
	row := map[string]any{
		"code": in.Code, "inspection_specialty_code": in.InspectionSpecialtyCode,
		"source_project_no": in.SourceProjectNo, "source_project_name": in.SourceProjectName,
		"name": in.Name, "is_optional_for_qualification": derefBool(in.IsOptionalForQualification, false),
		"is_official": derefBool(in.IsOfficial, true), "enabled": derefBool(in.Enabled, true),
		"sort_order": derefOr(in.SortOrder, 0), "created_at": now, "updated_at": now,
	}
	if err := s.DB.Table("inspection_objects").Create(row).Error; err != nil {
		return nil, err
	}
	return dtoObject(row), nil
}

// UpdateObject PATCH（truthy: specialty/sourceProjectNo/sourceProjectName/name；
// strict: isOptionalForQualification/isOfficial/enabled/sortOrder）。
func (s *InspectionDictionaryService) UpdateObject(code string, specialtyCode, sourceProjectNo,
	sourceProjectName, name *string, isOptionalForQualification, isOfficial, enabled *bool,
	sortOrder *int32) (map[string]any, error) {
	if _, err := s.findRow("inspection_objects", code); err != nil {
		return nil, err
	}
	updates := map[string]any{"updated_at": NowISO()}
	for k, v := range map[string]*string{
		"inspection_specialty_code": specialtyCode,
		"source_project_no":         sourceProjectNo,
		"source_project_name":       sourceProjectName,
		"name":                      name,
	} {
		if v != nil && *v != "" {
			updates[k] = *v
		}
	}
	if isOptionalForQualification != nil {
		updates["is_optional_for_qualification"] = *isOptionalForQualification
	}
	if isOfficial != nil {
		updates["is_official"] = *isOfficial
	}
	if enabled != nil {
		updates["enabled"] = *enabled
	}
	if sortOrder != nil {
		updates["sort_order"] = *sortOrder
	}
	if err := s.DB.Table("inspection_objects").Where("code = ?", code).
		Updates(updates).Error; err != nil {
		return nil, err
	}
	row, err := s.findRow("inspection_objects", code)
	if err != nil {
		return nil, err
	}
	return dtoObject(row), nil
}

// DeleteObject 删除（未知 → 404）。
func (s *InspectionDictionaryService) DeleteObject(code string) error {
	if _, err := s.findRow("inspection_objects", code); err != nil {
		return err
	}
	return s.DB.Table("inspection_objects").Where("code = ?", code).Delete(nil).Error
}

// ---- M06.F03 InspectionParameter ----

func dtoParameter(row map[string]any) map[string]any {
	var aliases []string
	switch raw := row["aliases"].(type) {
	case []byte:
		_ = json.Unmarshal(raw, &aliases)
	case string:
		_ = json.Unmarshal([]byte(raw), &aliases)
	}
	if aliases == nil {
		aliases = []string{}
	}
	return map[string]any{
		"code": row["code"], "name": row["name"], "rawName": row["raw_name"],
		"canonicalName": row["canonical_name"], "methodText": row["method_text"],
		"aliases": aliases, "unit": row["unit"], "sourceType": row["source_type"],
		"sortOrder": row["sort_order"], "createdAt": row["created_at"], "updatedAt": row["updated_at"],
	}
}

// ListParameters keyword + sourceType 过滤。
func (s *InspectionDictionaryService) ListParameters(keyword, sourceType string) ([]map[string]any, error) {
	q := keywordLike(s.DB.Table("inspection_parameters"), keyword)
	if strings.TrimSpace(sourceType) != "" {
		q = q.Where("source_type = ?", sourceType)
	}
	var rows []map[string]any
	if err := q.Order("sort_order asc, code asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		out = append(out, dtoParameter(r))
	}
	return out, nil
}

// CreateParameterInput create 载荷（必填 code/name；rawName/canonicalName 缺省 = name）。
type CreateParameterInput struct {
	Code, Name, RawName, CanonicalName string
	MethodText, Unit                   *string
	Aliases                            []string
	SourceType                         string
	SortOrder                          *int32
}

// CreateParameter 新建。
func (s *InspectionDictionaryService) CreateParameter(in CreateParameterInput) (map[string]any, error) {
	if in.Code == "" || in.Name == "" {
		return nil, familyerrors.ErrBadRequest("code/name are required")
	}
	rawName, canonicalName := in.RawName, in.CanonicalName
	if rawName == "" {
		rawName = in.Name
	}
	if canonicalName == "" {
		canonicalName = in.Name
	}
	sourceType := in.SourceType
	if sourceType == "" {
		sourceType = "official"
	}
	if in.Aliases == nil {
		in.Aliases = []string{}
	}
	aliases, err := json.Marshal(in.Aliases)
	if err != nil {
		return nil, err
	}
	now := NowISO()
	row := map[string]any{
		"code": in.Code, "name": in.Name, "raw_name": rawName,
		"canonical_name": canonicalName, "method_text": in.MethodText,
		"aliases": string(aliases), "unit": in.Unit, "source_type": sourceType,
		"sort_order": derefOr(in.SortOrder, 0), "created_at": now, "updated_at": now,
	}
	if err := s.DB.Table("inspection_parameters").Create(row).Error; err != nil {
		return nil, err
	}
	return dtoParameter(row), nil
}

// UpdateParameter PATCH（truthy: name/rawName/canonicalName/sourceType；
// key: methodText/aliases/unit；strict: sortOrder）。
func (s *InspectionDictionaryService) UpdateParameter(code string, name, rawName,
	canonicalName, methodText *string, aliases []string, unit *string, sourceType *string,
	sortOrder *int32) (map[string]any, error) {
	if _, err := s.findRow("inspection_parameters", code); err != nil {
		return nil, err
	}
	updates := map[string]any{"updated_at": NowISO()}
	if name != nil && *name != "" {
		updates["name"] = *name
	}
	if rawName != nil && *rawName != "" {
		updates["raw_name"] = *rawName
	}
	if canonicalName != nil && *canonicalName != "" {
		updates["canonical_name"] = *canonicalName
	}
	if sourceType != nil && *sourceType != "" {
		updates["source_type"] = *sourceType
	}
	if methodText != nil {
		updates["method_text"] = *methodText
	}
	if unit != nil {
		updates["unit"] = *unit
	}
	if aliases != nil {
		b, err := json.Marshal(aliases)
		if err != nil {
			return nil, err
		}
		updates["aliases"] = string(b)
	}
	if sortOrder != nil {
		updates["sort_order"] = *sortOrder
	}
	if err := s.DB.Table("inspection_parameters").Where("code = ?", code).
		Updates(updates).Error; err != nil {
		return nil, err
	}
	row, err := s.findRow("inspection_parameters", code)
	if err != nil {
		return nil, err
	}
	return dtoParameter(row), nil
}

// DeleteParameter 删除（未知 → 404）。
func (s *InspectionDictionaryService) DeleteParameter(code string) error {
	if _, err := s.findRow("inspection_parameters", code); err != nil {
		return err
	}
	return s.DB.Table("inspection_parameters").Where("code = ?", code).Delete(nil).Error
}

// ---- M06.F04 InspectionStandard ----

func dtoStandard(row map[string]any) map[string]any {
	return map[string]any{
		"code": row["code"], "name": row["name"], "version": row["version"],
		"status": row["status"], "sourceDocumentId": row["source_document_id"],
		"sourceHash": row["source_hash"], "sortOrder": row["sort_order"],
		"createdAt": row["created_at"], "updatedAt": row["updated_at"],
	}
}

// ListStandards keyword + status 过滤。
func (s *InspectionDictionaryService) ListStandards(keyword, status string) ([]map[string]any, error) {
	q := keywordLike(s.DB.Table("inspection_standards"), keyword)
	if strings.TrimSpace(status) != "" {
		q = q.Where("status = ?", status)
	}
	var rows []map[string]any
	if err := q.Order("sort_order asc, code asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		out = append(out, dtoStandard(r))
	}
	return out, nil
}

// CreateStandardInput create 载荷（必填 code/name；status 缺省 active）。
type CreateStandardInput struct {
	Code, Name       string
	Version, Status  string
	SourceDocumentId *string
	SourceHash       *string
	SortOrder        *int32
}

// CreateStandard 新建。
func (s *InspectionDictionaryService) CreateStandard(in CreateStandardInput) (map[string]any, error) {
	if in.Code == "" || in.Name == "" {
		return nil, familyerrors.ErrBadRequest("code/name are required")
	}
	status := in.Status
	if status == "" {
		status = "active"
	}
	now := NowISO()
	row := map[string]any{
		"code": in.Code, "name": in.Name, "version": strPtrOrNil(in.Version),
		"status": status, "source_document_id": in.SourceDocumentId,
		"source_hash": in.SourceHash, "sort_order": derefOr(in.SortOrder, 0),
		"created_at": now, "updated_at": now,
	}
	if err := s.DB.Table("inspection_standards").Create(row).Error; err != nil {
		return nil, err
	}
	return dtoStandard(row), nil
}

// UpdateStandard PATCH（truthy: name/status；key: version/sourceDocumentId/sourceHash；
// strict: sortOrder）。
func (s *InspectionDictionaryService) UpdateStandard(code string, name, status, version *string,
	sourceDocumentId, sourceHash *string, sortOrder *int32) (map[string]any, error) {
	if _, err := s.findRow("inspection_standards", code); err != nil {
		return nil, err
	}
	updates := map[string]any{"updated_at": NowISO()}
	if name != nil && *name != "" {
		updates["name"] = *name
	}
	if status != nil && *status != "" {
		updates["status"] = *status
	}
	if version != nil {
		updates["version"] = *version
	}
	if sourceDocumentId != nil {
		updates["source_document_id"] = *sourceDocumentId
	}
	if sourceHash != nil {
		updates["source_hash"] = *sourceHash
	}
	if sortOrder != nil {
		updates["sort_order"] = *sortOrder
	}
	if err := s.DB.Table("inspection_standards").Where("code = ?", code).
		Updates(updates).Error; err != nil {
		return nil, err
	}
	row, err := s.findRow("inspection_standards", code)
	if err != nil {
		return nil, err
	}
	return dtoStandard(row), nil
}

// DeleteStandard 删除（未知 → 404）。
func (s *InspectionDictionaryService) DeleteStandard(code string) error {
	if _, err := s.findRow("inspection_standards", code); err != nil {
		return err
	}
	return s.DB.Table("inspection_standards").Where("code = ?", code).Delete(nil).Error
}

// ---- junction：specialty ↔ object ----

// LinkSpecialtyObject upsert（重复 link 整行覆盖，含 created_at）。
func (s *InspectionDictionaryService) LinkSpecialtyObject(specialtyCode, objectCode string, remark *string) error {
	if err := requireNonBlank(specialtyCode, objectCode); err != nil {
		return err
	}
	return s.upsertJunction("inspection_specialty_objects",
		map[string]any{"inspection_specialty_code": specialtyCode,
			"inspection_object_code": objectCode},
		map[string]any{"remark": remark})
}

// UnlinkSpecialtyObject 幂等删除。
func (s *InspectionDictionaryService) UnlinkSpecialtyObject(specialtyCode, objectCode string) error {
	return s.DB.Table("inspection_specialty_objects").
		Where("inspection_specialty_code = ? AND inspection_object_code = ?", specialtyCode, objectCode).
		Delete(nil).Error
}

// ListSpecialtyObjectLinks 按专项过滤（blank = 不过滤）。
func (s *InspectionDictionaryService) ListSpecialtyObjectLinks(specialtyCode string) ([]map[string]any, error) {
	q := s.DB.Table("inspection_specialty_objects")
	if strings.TrimSpace(specialtyCode) != "" {
		q = q.Where("inspection_specialty_code = ?", specialtyCode)
	}
	var rows []map[string]any
	if err := q.Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		out = append(out, map[string]any{
			"inspectionSpecialtyCode": r["inspection_specialty_code"],
			"inspectionObjectCode":    r["inspection_object_code"],
			"remark":                  r["remark"],
		})
	}
	return out, nil
}

// ---- junction：object ↔ parameter ----

// LinkObjectParameter upsert（qualificationLevel 缺省 QUALIFIED）。
func (s *InspectionDictionaryService) LinkObjectParameter(objectCode, parameterCode string,
	qualificationLevel string, sourcePage *int32, remark *string) error {
	if err := requireNonBlank(objectCode, parameterCode); err != nil {
		return err
	}
	if qualificationLevel == "" {
		qualificationLevel = "QUALIFIED"
	}
	return s.upsertJunction("inspection_object_parameters",
		map[string]any{"inspection_object_code": objectCode,
			"inspection_parameter_code": parameterCode},
		map[string]any{"qualification_level": qualificationLevel,
			"source_page": sourcePage, "remark": remark})
}

// UnlinkObjectParameter 幂等删除。
func (s *InspectionDictionaryService) UnlinkObjectParameter(objectCode, parameterCode string) error {
	return s.DB.Table("inspection_object_parameters").
		Where("inspection_object_code = ? AND inspection_parameter_code = ?", objectCode, parameterCode).
		Delete(nil).Error
}

// ListObjectParameterLinks 双 code 过滤。
func (s *InspectionDictionaryService) ListObjectParameterLinks(objectCode, parameterCode string) ([]map[string]any, error) {
	q := s.DB.Table("inspection_object_parameters")
	if strings.TrimSpace(objectCode) != "" {
		q = q.Where("inspection_object_code = ?", objectCode)
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
			"inspectionObjectCode":    r["inspection_object_code"],
			"inspectionParameterCode": r["inspection_parameter_code"],
			"qualificationLevel":      r["qualification_level"],
			"sourcePage":              r["source_page"],
			"remark":                  r["remark"],
		})
	}
	return out, nil
}

// ---- junction：object ↔ standard (role) ----

// LinkObjectStandard upsert（role 必填，复合 PK 三列）。
func (s *InspectionDictionaryService) LinkObjectStandard(objectCode, standardCode, role string, remark *string) error {
	if err := requireNonBlank(objectCode, standardCode, role); err != nil {
		return err
	}
	return s.upsertJunction("inspection_object_standards",
		map[string]any{"inspection_object_code": objectCode,
			"inspection_standard_code": standardCode, "role": role},
		map[string]any{"remark": remark})
}

// UnlinkObjectStandard 幂等删除。
func (s *InspectionDictionaryService) UnlinkObjectStandard(objectCode, standardCode, role string) error {
	return s.DB.Table("inspection_object_standards").
		Where("inspection_object_code = ? AND inspection_standard_code = ? AND role = ?",
			objectCode, standardCode, role).
		Delete(nil).Error
}

// ListObjectStandardLinks objectCode 过滤 + role 过滤（blank = 不过滤）。
func (s *InspectionDictionaryService) ListObjectStandardLinks(objectCode, role string) ([]map[string]any, error) {
	q := s.DB.Table("inspection_object_standards")
	if strings.TrimSpace(objectCode) != "" {
		q = q.Where("inspection_object_code = ?", objectCode)
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
			"inspectionObjectCode":   r["inspection_object_code"],
			"inspectionStandardCode": r["inspection_standard_code"],
			"role":                   r["role"],
			"remark":                 r["remark"],
		})
	}
	return out, nil
}

// ---- junction：standard ↔ parameter ----

// LinkStandardParameter upsert（无载荷双键）。
func (s *InspectionDictionaryService) LinkStandardParameter(standardCode, parameterCode string) error {
	if err := requireNonBlank(standardCode, parameterCode); err != nil {
		return err
	}
	return s.upsertJunction("inspection_standard_parameters",
		map[string]any{"inspection_standard_code": standardCode,
			"inspection_parameter_code": parameterCode},
		map[string]any{})
}

// UnlinkStandardParameter 幂等删除。
func (s *InspectionDictionaryService) UnlinkStandardParameter(standardCode, parameterCode string) error {
	return s.DB.Table("inspection_standard_parameters").
		Where("inspection_standard_code = ? AND inspection_parameter_code = ?", standardCode, parameterCode).
		Delete(nil).Error
}

// ListStandardParameterLinks 双 code 过滤。
func (s *InspectionDictionaryService) ListStandardParameterLinks(standardCode, parameterCode string) ([]map[string]any, error) {
	q := s.DB.Table("inspection_standard_parameters")
	if strings.TrimSpace(standardCode) != "" {
		q = q.Where("inspection_standard_code = ?", standardCode)
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
			"inspectionStandardCode":  r["inspection_standard_code"],
			"inspectionParameterCode": r["inspection_parameter_code"],
		})
	}
	return out, nil
}

// ---- junction 共享 ----

// upsertJunction 同 PK 已存在则整行覆盖（含 created_at —— springboot merge 语义镜像），
// 否则新建。
func (s *InspectionDictionaryService) upsertJunction(table string, keys, payload map[string]any) error {
	now := NowISO()
	var rows []map[string]any
	q := s.DB.Table(table)
	for col, val := range keys {
		q = q.Where(col+" = ?", val)
	}
	if err := q.Find(&rows).Error; err != nil {
		return err
	}
	if len(rows) > 0 {
		updates := map[string]any{"created_at": now, "updated_at": now}
		for col, val := range payload {
			updates[col] = val
		}
		q2 := s.DB.Table(table)
		for col, val := range keys {
			q2 = q2.Where(col+" = ?", val)
		}
		return q2.Updates(updates).Error
	}
	row := map[string]any{}
	for col, val := range keys {
		row[col] = val
	}
	for col, val := range payload {
		row[col] = val
	}
	row["created_at"] = now
	row["updated_at"] = now
	return s.DB.Table(table).Create(row).Error
}

// ---- 小件 ----

func derefBool(v *bool, def bool) bool {
	if v == nil {
		return def
	}
	return *v
}

func strPtrOrNil(s string) any {
	if s == "" {
		return nil
	}
	return s
}
