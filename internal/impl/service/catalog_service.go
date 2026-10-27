package service

// catalog_service.go — 目录谱系码表服务（镜像 rails Catalog::CatalogService，
// lab-springboot CatalogService 1:1，REQ-2026-004 Task 6）。
//
// 4 表同构（brand/model/spec/grade）：tenant 隔离（unique (tenant_id, code)，
// code 是全局 PK 列但按 tenant 过滤后寻址）；list 按 inspectionObjectCode +
// keyword（code/name 大小写不敏感 LIKE）过滤，排序 sort_order, code；
// 更新 nil 字段跳过（部分更新），updated_at 恒刷新。
//
// 实现注：4 表列集全同，单实现走 Table(name) 动态表（rails KINDS meta 的等价物），
// DTO 蛇形 map，camelCase 由渲染层处理。

import (
	"strings"
	"time"

	"gorm.io/gorm"

	familyerrors "lab-management-system-go/internal/impl/errors"
)

// catalogTables kind → 表名（rails CatalogService::KINDS 等价）。
var catalogTables = map[string]string{
	"brand": "inspection_brands",
	"model": "inspection_models",
	"spec":  "inspection_specs",
	"grade": "inspection_grades",
}

// NowISO 应用层时间戳（lab 家族 created_at/updated_at 是 TEXT 列，ISO-8601 UTC
// 串由应用层维护——rails ApplicationRecord.now_iso / springboot nowIso() 镜像）。
func NowISO() string {
	return time.Now().UTC().Format("2006-01-02T15:04:05+00:00")
}

// CatalogService 码表面。
type CatalogService struct {
	DB *gorm.DB
}

// NewCatalogService 构造。
func NewCatalogService(db *gorm.DB) *CatalogService { return &CatalogService{DB: db} }

// List 全量列表（不切页，信封在 handler）：objectCode/keyword 可空。
func (s *CatalogService) List(kind, tenantID, objectCode, keyword string) ([]map[string]any, error) {
	table, err := catalogTable(kind)
	if err != nil {
		return nil, err
	}
	q := s.DB.Table(table).Where("tenant_id = ?", tenantID)
	if objectCode != "" {
		q = q.Where("inspection_object_code = ?", objectCode)
	}
	if keyword != "" {
		kw := "%" + strings.ToLower(keyword) + "%"
		q = q.Where("LOWER(code) LIKE ? OR LOWER(name) LIKE ?", kw, kw)
	}
	var rows []map[string]any
	if err := q.Order("sort_order asc, code asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// Create 新建（code+name 必填 → 400；唯一冲突 → 500 上抛）。
func (s *CatalogService) Create(kind, tenantID, code, name string, objectCode, remark *string, sortOrder *int32) (map[string]any, error) {
	table, err := catalogTable(kind)
	if err != nil {
		return nil, err
	}
	if code == "" || name == "" {
		return nil, familyerrors.ErrBadRequest("code and name are required")
	}
	now := NowISO()
	entry := map[string]any{
		"code": code, "name": name, "tenant_id": tenantID,
		"created_at": now, "updated_at": now,
		"inspection_object_code": objectCode, "remark": remark,
		"sort_order": derefOr(sortOrder, 0),
	}
	if err := s.DB.Table(table).Create(entry).Error; err != nil {
		return nil, err
	}
	return s.find(table, tenantID, code)
}

// Update 部分更新（nil 字段跳过，updated_at 恒刷新；未知 → 404）。
func (s *CatalogService) Update(kind, tenantID, code string, objectCode, name, remark *string, sortOrder *int32) (map[string]any, error) {
	table, err := catalogTable(kind)
	if err != nil {
		return nil, err
	}
	if _, err := s.find(table, tenantID, code); err != nil {
		return nil, err
	}
	updates := map[string]any{"updated_at": NowISO()}
	if objectCode != nil {
		updates["inspection_object_code"] = *objectCode
	}
	if name != nil {
		updates["name"] = *name
	}
	if remark != nil {
		updates["remark"] = *remark
	}
	if sortOrder != nil {
		updates["sort_order"] = *sortOrder
	}
	if err := s.DB.Table(table).Where("tenant_id = ? AND code = ?", tenantID, code).
		Updates(updates).Error; err != nil {
		return nil, err
	}
	return s.find(table, tenantID, code)
}

// Delete 删除（未知 → 404）。
func (s *CatalogService) Delete(kind, tenantID, code string) error {
	table, err := catalogTable(kind)
	if err != nil {
		return err
	}
	if _, err := s.find(table, tenantID, code); err != nil {
		return err
	}
	return s.DB.Table(table).Where("tenant_id = ? AND code = ?", tenantID, code).Delete(nil).Error
}

// find 按 tenant+code 寻址（唯一 DTO 出口；未知 → 404）。
func (s *CatalogService) find(table, tenantID, code string) (map[string]any, error) {
	var rows []map[string]any
	err := s.DB.Table(table).Where("tenant_id = ? AND code = ?", tenantID, code).Find(&rows).Error
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, familyerrors.ErrNotFound("record not found")
	}
	return rows[0], nil
}

func catalogTable(kind string) (string, error) {
	table, ok := catalogTables[kind]
	if !ok {
		return "", familyerrors.ErrBadRequest("unknown catalog kind: " + kind)
	}
	return table, nil
}

func derefOr(v *int32, def int32) int32 {
	if v == nil {
		return def
	}
	return *v
}
