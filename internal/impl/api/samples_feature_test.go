package api

// samples_feature_test.go — 样品/检测记录 Feature 测试（REQ-2026-004 Task 10）。
//
// 镜像 rails Samples::SampleService + TestRecords::TestRecordService：
// - samples（M03.F03.I01-I05）：tenant 收口；receiptId FK（未知 404）；ext jsonb
//   缺省 {}；update PATCH 语义（receiptId 不可变、ext 非 nil 即写）；
//   updateSampleExt 独立端点 ext 必填整体替换；business 信封（pageSize 缺省 20）
// - test-records（M06.F03.I06-I11）：4 键必填 400；tenant 收口；sampleId 过滤；
//   setVerdict 独立动词 PATCH /{id}/verdict；id 前缀 TR-
//
// 真库 lab_test，WithTx 整体回滚（testsupport.Tx）——不污染种子态。

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"lab-management-system-go/internal/impl/service"
)

// seedReceipt 直插 contract + sample_receipts 父链（receipts CRUD 属 Task 11，
// 此处只作 FK 母体；categoryCode FK → inspection_report_names 走 API 建）。
func seedReceipt(t *testing.T, s *Server, hdr map[string]string, suffix string) string {
	t.Helper()
	rnCode := "rc-rn-" + suffix
	rec := doJSON(t, s, "POST", "/api/report-names",
		map[string]any{"code": rnCode, "name": "收样报告" + suffix}, hdr)
	if rec.Code != 200 {
		t.Fatalf("report-name create 期望 200，得 %d: %s", rec.Code, rec.Body.String())
	}
	now := service.NowISO()

	contract := map[string]any{
		"id": "C-TEST-" + suffix, "contract_code": "CC-" + suffix,
		"client_unit": "甲方", "project_name": "项目", "construction_unit": "施工",
		"witness_unit": "监理", "witness": "张工", "status": "active",
		"tenant_id": "TENANT-001", "created_at": now, "updated_at": now}
	if err := s.DB.Table("contracts").Create(contract).Error; err != nil {
		t.Fatalf("seed contract: %v", err)
	}
	receipt := map[string]any{
		"id": "R-TEST-" + suffix, "contract_id": contract["id"],
		"commission_code": "WT-" + suffix, "commission_date": "2026-10-27",
		"category_code": rnCode, "received_by": "收样员", "sample_source": "现场",
		"test_category": "常规", "flow_status": "receiving",
		"flow_history": "[]", "ext": "{}",
		"tenant_id": "TENANT-001", "created_at": now, "updated_at": now}
	if err := s.DB.Table("sample_receipts").Create(receipt).Error; err != nil {
		t.Fatalf("seed receipt: %v", err)
	}
	return receipt["id"].(string)
}

// TestSampleCycle 样品五件套：未知 receipt 404 → create 缺省 ext → 信封 list +
// receiptId/keyword 过滤 → PATCH（receiptId 不可变、ext 写入）→
// updateSampleExt 必填/整体替换 → delete 204/复删 404。
// fn: M03.F03.I01
// fn: M03.F03.I02
// fn: M03.F03.I03
// fn: M03.F03.I04
// fn: M03.F03.I05
func TestSampleCycle(t *testing.T) {
	s := dictServer(t)
	hdr := dictHdr(t)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	receiptID := seedReceipt(t, s, hdr, suffix)

	// create：未知 receiptId → 404（rails RecordNotFound 镜像）
	rec := doJSON(t, s, "POST", "/api/samples",
		map[string]any{"receiptId": "R-NOPE-" + suffix, "sampleCode": "SP-" + suffix}, hdr)
	if rec.Code != 404 {
		t.Fatalf("未知 receiptId 应 404，得 %d", rec.Code)
	}
	// 正常 create：ext 缺省 {}
	rec = doJSON(t, s, "POST", "/api/samples",
		map[string]any{"receiptId": receiptID, "sampleCode": "sp-" + suffix,
			"sampleName": "样品" + suffix}, hdr)
	if rec.Code != 200 {
		t.Fatalf("create 期望 200，得 %d: %s", rec.Code, rec.Body.String())
	}
	created := jsonBody(t, rec)
	if !strings.HasPrefix(created["id"].(string), "S-") {
		t.Fatalf("id 应 S- 前缀: %v", created["id"])
	}
	if created["tenantId"] != "TENANT-001" {
		t.Fatalf("tenantId 应来自 token: %v", created)
	}
	if ext, ok := created["ext"].(map[string]any); !ok || len(ext) != 0 {
		t.Fatalf("ext 缺省应 {}: %v", created["ext"])
	}
	id := created["id"].(string)

	// list：business 信封（pageSize 缺省 20）+ receiptId/keyword 过滤
	rec = doReq(t, s, "GET", "/api/samples?receiptId="+receiptID, "", hdr)
	m := jsonBody(t, rec)
	if m["page"].(float64) != 1 || m["pageSize"].(float64) != 20 || m["total"].(float64) != 1 {
		t.Fatalf("list 信封不符: %v", m)
	}
	rec = doReq(t, s, "GET", "/api/samples?receiptId=R-other", "", hdr)
	if got := len(jsonBody(t, rec)["items"].([]any)); got != 0 {
		t.Fatalf("receiptId 过滤不应命中他单样品，得 %d", got)
	}
	rec = doReq(t, s, "GET", "/api/samples?keyword=SP-"+suffix, "", hdr)
	if got := len(jsonBody(t, rec)["items"].([]any)); got != 1 {
		t.Fatalf("keyword 大小写不敏感应命中，得 %d", got)
	}

	// get 详情
	rec = doReq(t, s, "GET", "/api/samples/"+id, "", hdr)
	if jsonBody(t, rec)["sampleCode"] != "sp-"+suffix {
		t.Fatalf("get 不符: %s", rec.Body.String())
	}
	rec = doReq(t, s, "GET", "/api/samples/"+id, "",
		map[string]string{"Authorization": "Bearer " + catalogToken(t, "TENANT-002")})
	if rec.Code != 404 {
		t.Fatalf("跨租户 get 应 404，得 %d", rec.Code)
	}

	// update PATCH：sampleName/ext 写入；receiptId 不可变（rails UPDATE_FIELDS 无 receiptId）
	rec = doJSON(t, s, "PUT", "/api/samples/"+id, map[string]any{
		"sampleName": "样品改", "ext": map[string]string{"strength": "C30"},
		"receiptId": "R-EVIL"}, hdr)
	if rec.Code != 200 {
		t.Fatalf("update 期望 200，得 %d: %s", rec.Code, rec.Body.String())
	}
	updated := jsonBody(t, rec)
	if updated["sampleName"] != "样品改" || updated["receiptId"] != receiptID {
		t.Fatalf("PATCH 不符（receiptId 应不可变）: %v", updated)
	}
	if ext, ok := updated["ext"].(map[string]any); !ok || ext["strength"] != "C30" {
		t.Fatalf("update ext jsonb 回读不符: %v", updated["ext"])
	}

	// updateSampleExt：ext 必填 400；整体替换
	rec = doJSON(t, s, "PUT", "/api/samples/"+id+"/ext", map[string]any{}, hdr)
	if rec.Code != 400 {
		t.Fatalf("updateSampleExt 缺 ext 应 400，得 %d", rec.Code)
	}
	rec = doJSON(t, s, "PUT", "/api/samples/"+id+"/ext",
		map[string]any{"ext": map[string]string{"slump": "180"}}, hdr)
	if rec.Code != 200 {
		t.Fatalf("updateSampleExt 期望 200，得 %d: %s", rec.Code, rec.Body.String())
	}
	if ext, ok := jsonBody(t, rec)["ext"].(map[string]any); !ok ||
		len(ext) != 1 || ext["slump"] != "180" {
		t.Fatalf("updateSampleExt 应整体替换 ext: %v", jsonBody(t, rec)["ext"])
	}

	// delete 204 + 复删 404
	rec = doReq(t, s, "DELETE", "/api/samples/"+id, "", hdr)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete 期望 204，得 %d", rec.Code)
	}
	rec = doReq(t, s, "DELETE", "/api/samples/"+id, "", hdr)
	if rec.Code != 404 {
		t.Fatalf("复删应 404，得 %d", rec.Code)
	}
}

// TestTestRecordCycle 检测记录六件套：4 键必填 400 → create → 信封 list +
// sampleId 过滤 → 跨租户 404 → PATCH → setVerdict 改判 → delete 204/复删 404。
// fn: M03.F03.I06
// fn: M03.F03.I07
// fn: M03.F03.I08
// fn: M03.F03.I09
// fn: M03.F03.I10
// fn: M03.F03.I11
func TestTestRecordCycle(t *testing.T) {
	s := dictServer(t)
	hdr := dictHdr(t)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	receiptID := seedReceipt(t, s, hdr, suffix)
	prmCode := "tr-prm-" + suffix
	mustCreateParameter(t, s, hdr, prmCode, suffix) // testrec_param_fk
	body := map[string]any{"sampleId": "", "parameterCode": prmCode,
		"requirement": "≥30MPa", "result": "32.5"}

	// 建样品母体（test_records_sample_fk）
	rec := doJSON(t, s, "POST", "/api/samples",
		map[string]any{"receiptId": receiptID, "sampleCode": "sp-" + suffix}, hdr)
	if rec.Code != 200 {
		t.Fatalf("sample create 期望 200，得 %d: %s", rec.Code, rec.Body.String())
	}
	sampleID := jsonBody(t, rec)["id"].(string)
	body["sampleId"] = sampleID

	// create：缺 result 400
	miss := map[string]any{"sampleId": sampleID, "parameterCode": prmCode,
		"requirement": "≥30MPa"}
	rec = doJSON(t, s, "POST", "/api/test-records", miss, hdr)
	if rec.Code != 400 {
		t.Fatalf("缺 result 应 400，得 %d", rec.Code)
	}
	// 正常 create
	rec = doJSON(t, s, "POST", "/api/test-records", body, hdr)
	if rec.Code != 200 {
		t.Fatalf("create 期望 200，得 %d: %s", rec.Code, rec.Body.String())
	}
	created := jsonBody(t, rec)
	if !strings.HasPrefix(created["id"].(string), "TR-") {
		t.Fatalf("id 应 TR- 前缀: %v", created["id"])
	}
	if created["tenantId"] != "TENANT-001" || created["verdict"] != nil {
		t.Fatalf("create 缺省不符: %v", created)
	}
	id := created["id"].(string)

	// list：business 信封 + sampleId 过滤
	rec = doReq(t, s, "GET", "/api/test-records?sampleId="+sampleID, "", hdr)
	m := jsonBody(t, rec)
	if m["page"].(float64) != 1 || m["pageSize"].(float64) != 20 || m["total"].(float64) != 1 {
		t.Fatalf("list 信封不符: %v", m)
	}
	rec = doReq(t, s, "GET", "/api/test-records?sampleId=S-other", "", hdr)
	if got := len(jsonBody(t, rec)["items"].([]any)); got != 0 {
		t.Fatalf("sampleId 过滤不应命中，得 %d", got)
	}

	// get：本租户 200 / 跨租户 404
	rec = doReq(t, s, "GET", "/api/test-records/"+id, "", hdr)
	if jsonBody(t, rec)["parameterCode"] != prmCode {
		t.Fatalf("get 不符: %s", rec.Body.String())
	}
	rec = doReq(t, s, "GET", "/api/test-records/"+id, "",
		map[string]string{"Authorization": "Bearer " + catalogToken(t, "TENANT-002")})
	if rec.Code != 404 {
		t.Fatalf("跨租户 get 应 404，得 %d", rec.Code)
	}

	// update PATCH：result/verdict 写入，未提字段保留
	rec = doJSON(t, s, "PUT", "/api/test-records/"+id,
		map[string]any{"result": "33.1", "verdict": "PASS"}, hdr)
	if rec.Code != 200 {
		t.Fatalf("update 期望 200，得 %d: %s", rec.Code, rec.Body.String())
	}
	updated := jsonBody(t, rec)
	if updated["result"] != "33.1" || updated["verdict"] != "PASS" ||
		updated["requirement"] != "≥30MPa" {
		t.Fatalf("PATCH 不符: %v", updated)
	}

	// setVerdict 改判（PATCH 动词，2026-09-18 以 .tsp 为准）
	rec = doJSON(t, s, "PATCH", "/api/test-records/"+id+"/verdict",
		map[string]any{"verdict": "FAIL"}, hdr)
	if rec.Code != 200 {
		t.Fatalf("setVerdict 期望 200，得 %d: %s", rec.Code, rec.Body.String())
	}
	if jsonBody(t, rec)["verdict"] != "FAIL" {
		t.Fatalf("setVerdict 应改判: %v", jsonBody(t, rec))
	}

	// delete 204 + 复删 404
	rec = doReq(t, s, "DELETE", "/api/test-records/"+id, "", hdr)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete 期望 204，得 %d", rec.Code)
	}
	rec = doReq(t, s, "DELETE", "/api/test-records/"+id, "", hdr)
	if rec.Code != 404 {
		t.Fatalf("复删应 404，得 %d", rec.Code)
	}
}
