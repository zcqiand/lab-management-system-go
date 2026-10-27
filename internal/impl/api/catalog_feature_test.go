package api

// catalog_feature_test.go — 目录谱系 CRUD Feature 测试（REQ-2026-004 Task 6）。
//
// 镜像 rails Catalog::CatalogService 语义：4 表同构（brand/model/spec/grade），
// tenant 隔离（unique (tenant_id, code)），list 全量+Page 信封（page 缺省 1、
// pageSize 缺省=items.size，不切页），keyword 大小写不敏感 LIKE code/name，
// 排序 sort_order, code；更新 nil 字段跳过；寻址 404。
//
// 真库 lab_test，WithTx 整体回滚（testsupport.Tx）——不污染种子态。

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"lab-management-system-go/internal/impl/auth"
	"lab-management-system-go/internal/testsupport"
)

// catalogServer 每用例一条事务贯穿：Server.DB 与断言查询共用同一连接。
func catalogServer(t *testing.T) *Server {
	t.Helper()
	setHandlerEnv(t)
	return &Server{DB: testsupport.Tx(t)}
}

// catalogToken 用测签发 TENANT-001 会话 token（catalog 端点全部 Bearer 保护）。
func catalogToken(t *testing.T, tenantID string) string {
	t.Helper()
	tok, err := auth.TokenService{}.IssueAccess("tester", tenantID)
	if err != nil {
		t.Fatalf("签发测试 token 失败: %v", err)
	}
	return tok
}

// catalogSeed 造码表行（tx 内，回滚即净）。inspection_object_code 有 FK 指向
// inspection_objects（其上还有 specialty 前置）——父行 OnConflict DoNothing 先补。
func catalogSeed(t *testing.T, db *gorm.DB, table, code, objectCode, name string, sortOrder int32) {
	t.Helper()
	now := time.Now().UTC().Format("2006-01-02T15:04:05+00:00")
	if err := db.Table("inspection_specialties").Clauses(clause.OnConflict{DoNothing: true}).Create(map[string]any{
		"code": "sp-t6", "official_no": "T6", "name": "T6 专项",
		"is_official": true, "enabled": true, "sort_order": 1,
		"created_at": now, "updated_at": now,
	}).Error; err != nil {
		t.Fatalf("seed specialty: %v", err)
	}
	if err := db.Table("inspection_objects").Clauses(clause.OnConflict{DoNothing: true}).Create(map[string]any{
		"code": objectCode, "inspection_specialty_code": "sp-t6",
		"source_project_no": "T6-001", "source_project_name": "T6 项目", "name": "T6 对象",
		"is_optional_for_qualification": false, "is_official": true, "enabled": true,
		"sort_order": 1, "created_at": now, "updated_at": now,
	}).Error; err != nil {
		t.Fatalf("seed object: %v", err)
	}
	err := db.Table(table).Create(map[string]any{
		"code": code, "inspection_object_code": objectCode, "name": name,
		"remark": nil, "sort_order": sortOrder,
		"created_at": now, "updated_at": now,
		"tenant_id": "TENANT-001",
	}).Error
	if err != nil {
		t.Fatalf("seed %s/%s: %v", table, code, err)
	}
}

// TestCatalogBrandCycle brand 四件套：list 信封+过滤+排序 → create → update → delete。
// fn: M04.F09.I01
// fn: M04.F09.I02
// fn: M04.F09.I03
// fn: M04.F09.I04
func TestCatalogBrandCycle(t *testing.T) {
	s := catalogServer(t)
	tok := catalogToken(t, "TENANT-001")
	hdr := map[string]string{"Authorization": "Bearer " + tok}
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())

	objA := "obj-t6a-" + suffix
	objB := "obj-t6b-" + suffix
	catalogSeed(t, s.DB, "inspection_brands", "br-b-"+suffix, objA, "品牌乙", 2)
	catalogSeed(t, s.DB, "inspection_brands", "br-a-"+suffix, objB, "品牌甲", 1)
	catalogSeed(t, s.DB, "inspection_brands", "br-c-"+suffix, objA, "品牌丙", 0)

	// list：keyword=suffix 圈定本用例数据；信封（page 缺省 1、pageSize 缺省=items.size）
	// + sort_order 排序。lab_test 有预置种子码表行，绝对总数不可断言。
	rec := doReq(t, s, "GET", "/api/catalog/brands?keyword="+suffix, "", hdr)
	if rec.Code != 200 {
		t.Fatalf("list 期望 200，得 %d: %s", rec.Code, rec.Body.String())
	}
	m := jsonBody(t, rec)
	items := m["items"].([]any)
	if m["page"].(float64) != 1 || m["total"].(float64) != 3 {
		t.Fatalf("信封不符: %v", m)
	}
	first := items[0].(map[string]any)
	if first["code"] != "br-c-"+suffix || first["sortOrder"].(float64) != 0 {
		t.Fatalf("sort_order 升序不符: %v", first)
	}

	// 过滤：inspectionObjectCode + keyword（大小写不敏感）
	rec = doReq(t, s, "GET", "/api/catalog/brands?inspectionObjectCode="+objA, "", hdr)
	if got := len(jsonBody(t, rec)["items"].([]any)); got != 2 {
		t.Fatalf("objectCode 过滤应 2 条，得 %d", got)
	}
	rec = doReq(t, s, "GET", "/api/catalog/brands?keyword=BR-A-"+suffix, "", hdr)
	if got := len(jsonBody(t, rec)["items"].([]any)); got != 1 {
		t.Fatalf("keyword 大小写不敏感应 1 条，得 %d", got)
	}

	// create：DTO camel 直出 + 缺 name 400
	rec = doJSON(t, s, "POST", "/api/catalog/brands", map[string]any{"code": "br-new-" + suffix, "name": "新品牌", "inspectionObjectCode": objA,
		"remark": "备注", "sortOrder": 5}, hdr)
	if rec.Code != 200 {
		t.Fatalf("create 期望 200，得 %d: %s", rec.Code, rec.Body.String())
	}
	created := jsonBody(t, rec)
	if created["code"] != "br-new-"+suffix || created["tenantId"] != "TENANT-001" || created["sortOrder"].(float64) != 5 {
		t.Fatalf("create DTO 不符: %v", created)
	}
	if created["createdAt"] == "" || created["updatedAt"] == "" {
		t.Fatalf("时间戳缺: %v", created)
	}
	rec = doJSON(t, s, "POST", "/api/catalog/brands", map[string]any{"code": "x"}, hdr)
	if rec.Code != 400 {
		t.Fatalf("缺 name 应 400，得 %d", rec.Code)
	}

	// update：nil 字段跳过（部分更新）+ 未知 404
	rec = doJSON(t, s, "PUT", "/api/catalog/brands/br-b-"+suffix,
		map[string]any{"name": "品牌乙改"}, hdr)
	if rec.Code != 200 {
		t.Fatalf("update 期望 200，得 %d: %s", rec.Code, rec.Body.String())
	}
	updated := jsonBody(t, rec)
	if updated["name"] != "品牌乙改" || updated["inspectionObjectCode"] != objA {
		t.Fatalf("部分更新不符（未提字段应保留）: %v", updated)
	}
	rec = doJSON(t, s, "PUT", "/api/catalog/brands/nope-"+suffix, map[string]any{"name": "x"}, hdr)
	if rec.Code != 404 {
		t.Fatalf("未知 code update 应 404，得 %d", rec.Code)
	}

	// delete：204 + 复删 404
	rec = doReq(t, s, "DELETE", "/api/catalog/brands/br-c-"+suffix, "", hdr)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete 期望 204，得 %d", rec.Code)
	}
	rec = doReq(t, s, "DELETE", "/api/catalog/brands/br-c-"+suffix, "", hdr)
	if rec.Code != 404 {
		t.Fatalf("复删应 404，得 %d", rec.Code)
	}
}

// TestCatalogModelCycle model 四件套（M04.F06）。
// fn: M04.F06.I01
// fn: M04.F06.I02
// fn: M04.F06.I03
// fn: M04.F06.I04
func TestCatalogModelCycle(t *testing.T) {
	catalogKindCycle(t, "models", "inspection_models", "md")
}

// TestCatalogSpecCycle spec 四件套（M04.F07）。
// fn: M04.F07.I01
// fn: M04.F07.I02
// fn: M04.F07.I03
// fn: M04.F07.I04
func TestCatalogSpecCycle(t *testing.T) {
	catalogKindCycle(t, "specs", "inspection_specs", "sp")
}

// TestCatalogGradeCycle grade 四件套（M04.F08）。
// fn: M04.F08.I01
// fn: M04.F08.I02
// fn: M04.F08.I03
// fn: M04.F08.I04
func TestCatalogGradeCycle(t *testing.T) {
	catalogKindCycle(t, "grades", "inspection_grades", "gd")
}

// catalogKindCycle 4 字典同构路径共用（brand 已展开独立用例）。
func catalogKindCycle(t *testing.T, endpoint, table, prefix string) {
	t.Helper()
	s := catalogServer(t)
	tok := catalogToken(t, "TENANT-001")
	hdr := map[string]string{"Authorization": "Bearer " + tok}
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	code := prefix + "-1-" + suffix

	// create
	rec := doJSON(t, s, "POST", "/api/catalog/"+endpoint, map[string]any{"code": code, "name": "字典项"}, hdr)
	if rec.Code != 200 {
		t.Fatalf("%s create 期望 200，得 %d: %s", endpoint, rec.Code, rec.Body.String())
	}
	// list 命中
	rec = doReq(t, s, "GET", "/api/catalog/"+endpoint+"?keyword="+code, "", hdr)
	items := jsonBody(t, rec)["items"].([]any)
	if len(items) != 1 || items[0].(map[string]any)["code"] != code {
		t.Fatalf("%s list 应命中: %v", endpoint, items)
	}
	// update
	rec = doJSON(t, s, "PUT", "/api/catalog/"+endpoint+"/"+code, map[string]any{"name": "改"}, hdr)
	if rec.Code != 200 || jsonBody(t, rec)["name"] != "改" {
		t.Fatalf("%s update 不符: %d %s", endpoint, rec.Code, rec.Body.String())
	}
	// delete + 404
	rec = doReq(t, s, "DELETE", "/api/catalog/"+endpoint+"/"+code, "", hdr)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("%s delete 期望 204，得 %d", endpoint, rec.Code)
	}
	rec = doReq(t, s, "GET", "/api/catalog/"+endpoint+"?keyword="+code, "", hdr)
	if got := len(jsonBody(t, rec)["items"].([]any)); got != 0 {
		t.Fatalf("%s delete 后 list 应空，得 %d", endpoint, got)
	}
}

// TestCatalogTenantIsolation tenant 隔离：TENANT-001 的行对 TENANT-002 不可见/不可寻址。
// fn: M04.F09.I01
func TestCatalogTenantIsolation(t *testing.T) {
	s := catalogServer(t)
	hdr1 := map[string]string{"Authorization": "Bearer " + catalogToken(t, "TENANT-001")}
	hdr2 := map[string]string{"Authorization": "Bearer " + catalogToken(t, "TENANT-002")}
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())

	catalogSeed(t, s.DB, "inspection_brands", "br-iso-"+suffix, "obj-1", "隔离品牌", 1)

	// TENANT-002 list 看不到
	rec := doReq(t, s, "GET", "/api/catalog/brands?keyword=br-iso-"+suffix, "", hdr2)
	if got := len(jsonBody(t, rec)["items"].([]any)); got != 0 {
		t.Fatalf("跨租户 list 应不可见，得 %d", got)
	}
	// TENANT-002 寻址 404（update/delete）
	rec = doJSON(t, s, "PUT", "/api/catalog/brands/br-iso-"+suffix, map[string]any{"name": "x"}, hdr2)
	if rec.Code != 404 {
		t.Fatalf("跨租户 update 应 404，得 %d", rec.Code)
	}
	// TENANT-001 正常可见
	rec = doReq(t, s, "GET", "/api/catalog/brands?keyword=br-iso-"+suffix, "", hdr1)
	if got := len(jsonBody(t, rec)["items"].([]any)); got != 1 {
		t.Fatalf("本租户应可见，得 %d", got)
	}
}

// TestCatalogPageEnvelopeOverride page/pageSize 显式回显不切页（T11 live 语义）。
// fn: M04.F09.I01
func TestCatalogPageEnvelopeOverride(t *testing.T) {
	s := catalogServer(t)
	hdr := map[string]string{"Authorization": "Bearer " + catalogToken(t, "TENANT-001")}
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	objP := "obj-p-" + suffix
	catalogSeed(t, s.DB, "inspection_brands", "br-p1-"+suffix, objP, "P1", 1)
	catalogSeed(t, s.DB, "inspection_brands", "br-p2-"+suffix, objP, "P2", 2)

	rec := doReq(t, s, "GET", "/api/catalog/brands?inspectionObjectCode="+objP+"&page=3&pageSize=99", "", hdr)
	m := jsonBody(t, rec)
	if m["page"].(float64) != 3 || m["pageSize"].(float64) != 99 {
		t.Fatalf("page/pageSize 只回显不切页: %v", m)
	}
	if got := len(m["items"].([]any)); got != 2 {
		t.Fatalf("全量 items 不切页，得 %d", got)
	}
}
