package api

// business_feature_test.go — 商务面 Feature 测试（REQ-2026-004 Task 9）。
//
// 镜像 rails Contracts::ContractService + ReportNames::InspectionReportNameService
// + ReportNameJunctions：
// - contracts（M02.F01）：tenant 收口；business 信封（pageSize 缺省 20）；
//   必填 6 键 400；status 枚举 active/archived（非法 400）；id 前缀 C-
// - report-names（M06.F07）：code 主键平台级；create 同 code upsert merge；
//   extFields jsonb DTO 端 null→[]；wrapDict 信封
// - 三组 junction link/unlink/list：link 幂等 204、unlink 幂等 204、短信封
//
// 真库 lab_test，WithTx 整体回滚（testsupport.Tx）——不污染种子态。

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

// TestContractCycle 合同五件套：必填 400 → create 缺省 status → 信封 list + 过滤
// → PATCH → delete。
// fn: M02.F01.I01
// fn: M02.F01.I02
// fn: M02.F01.I03
// fn: M02.F01.I04
// fn: M02.F01.I05
func TestContractCycle(t *testing.T) {
	s := dictServer(t)
	hdr := dictHdr(t)
	hdr2 := map[string]string{"Authorization": "Bearer " + catalogToken(t, "TENANT-002")}
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	code := "CT-" + suffix
	spCode := mustCreateSpecialty(t, s, hdr, "ct-sp-"+suffix, suffix) // contracts_specialty_fk
	body := map[string]any{
		"contractCode": code, "clientUnit": "甲方" + suffix, "projectName": "项目" + suffix,
		"constructionUnit": "施工" + suffix, "witnessUnit": "监理" + suffix,
		"witness": "张工", "inspectionSpecialtyCode": spCode}

	// create：必填缺失 400
	miss := map[string]any{}
	for k, v := range body {
		if k != "witness" {
			miss[k] = v
		}
	}
	rec := doJSON(t, s, "POST", "/api/contracts", miss, hdr)
	if rec.Code != 400 {
		t.Fatalf("缺 witness 应 400，得 %d", rec.Code)
	}
	// 非法 status 400
	bad := map[string]any{}
	for k, v := range body {
		bad[k] = v
	}
	bad["status"] = "bogus"
	rec = doJSON(t, s, "POST", "/api/contracts", bad, hdr)
	if rec.Code != 400 {
		t.Fatalf("非法 status 应 400，得 %d", rec.Code)
	}
	// 正常 create
	rec = doJSON(t, s, "POST", "/api/contracts", body, hdr)
	if rec.Code != 200 {
		t.Fatalf("create 期望 200，得 %d: %s", rec.Code, rec.Body.String())
	}
	created := jsonBody(t, rec)
	if created["status"] != "active" || created["tenantId"] != "TENANT-001" {
		t.Fatalf("create 缺省不符: %v", created)
	}
	if !strings.HasPrefix(created["id"].(string), "C-") {
		t.Fatalf("id 应 C- 前缀: %v", created["id"])
	}
	id := created["id"].(string)

	// list：信封（pageSize 缺省 20）+ keyword + status 过滤
	rec = doReq(t, s, "GET", "/api/contracts?keyword="+suffix, "", hdr)
	m := jsonBody(t, rec)
	if m["page"].(float64) != 1 || m["pageSize"].(float64) != 20 || m["total"].(float64) != 1 {
		t.Fatalf("list 信封不符: %v", m)
	}
	rec = doReq(t, s, "GET", "/api/contracts?keyword="+suffix+"&status=archived", "", hdr)
	if got := len(jsonBody(t, rec)["items"].([]any)); got != 0 {
		t.Fatalf("status=archived 不应命中 active 行，得 %d", got)
	}
	rec = doReq(t, s, "GET", "/api/contracts?keyword="+suffix+"&status=bogus", "", hdr)
	if rec.Code != 400 {
		t.Fatalf("list 非法 status 应 400，得 %d", rec.Code)
	}

	// get：本租户 200 / 跨租户 404
	rec = doReq(t, s, "GET", "/api/contracts/"+id, "", hdr)
	if jsonBody(t, rec)["contractCode"] != code {
		t.Fatalf("get 不符: %s", rec.Body.String())
	}
	rec = doReq(t, s, "GET", "/api/contracts/"+id, "", hdr2)
	if rec.Code != 404 {
		t.Fatalf("跨租户 get 应 404，得 %d", rec.Code)
	}

	// update PATCH：status + projectName；contractCode 不可变
	rec = doJSON(t, s, "PUT", "/api/contracts/"+id,
		map[string]any{"status": "archived", "projectName": "项目改"}, hdr)
	if rec.Code != 200 {
		t.Fatalf("update 期望 200，得 %d: %s", rec.Code, rec.Body.String())
	}
	updated := jsonBody(t, rec)
	if updated["status"] != "archived" || updated["projectName"] != "项目改" ||
		updated["clientUnit"] != "甲方"+suffix {
		t.Fatalf("PATCH 不符: %v", updated)
	}

	// delete 204 + 复删 404
	rec = doReq(t, s, "DELETE", "/api/contracts/"+id, "", hdr)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete 期望 204，得 %d", rec.Code)
	}
	rec = doReq(t, s, "DELETE", "/api/contracts/"+id, "", hdr)
	if rec.Code != 404 {
		t.Fatalf("复删应 404，得 %d", rec.Code)
	}
}

// TestReportNameCycle 报告名称五件套：code/name 必填 → 同 code upsert merge →
// extFields jsonb → 信封 list → PATCH → delete。
// fn: M06.F07.I01
// fn: M06.F07.I02
// fn: M06.F07.I03
// fn: M06.F07.I04
// fn: M06.F07.I05
func TestReportNameCycle(t *testing.T) {
	s := dictServer(t)
	hdr := dictHdr(t)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	code := "rn-" + suffix

	rec := doJSON(t, s, "POST", "/api/report-names",
		map[string]any{"code": code}, hdr)
	if rec.Code != 400 {
		t.Fatalf("缺 name 应 400，得 %d", rec.Code)
	}
	rec = doJSON(t, s, "POST", "/api/report-names",
		map[string]any{"code": code, "name": "报告甲"}, hdr)
	if rec.Code != 200 {
		t.Fatalf("create 期望 200，得 %d: %s", rec.Code, rec.Body.String())
	}
	created := jsonBody(t, rec)
	if ext, ok := created["extFields"].([]any); !ok || len(ext) != 0 {
		t.Fatalf("extFields 缺省应 []: %v", created["extFields"])
	}
	if created["sortOrder"].(float64) != 0 {
		t.Fatalf("sortOrder 缺省 0: %v", created)
	}
	// 同 code 再 create：upsert merge
	rec = doJSON(t, s, "POST", "/api/report-names",
		map[string]any{"code": code, "name": "报告甲改", "summaryName": "摘要"}, hdr)
	if rec.Code != 200 || jsonBody(t, rec)["name"] != "报告甲改" {
		t.Fatalf("同 code create 应 merge 200: %d %s", rec.Code, rec.Body.String())
	}

	// get + extFields jsonb 写读
	rec = doJSON(t, s, "PUT", "/api/report-names/"+code, map[string]any{
		"extFields": []map[string]any{{"key": "strength", "label": "强度", "type": "number"}}}, hdr)
	if rec.Code != 200 {
		t.Fatalf("update 期望 200，得 %d: %s", rec.Code, rec.Body.String())
	}
	updated := jsonBody(t, rec)
	ext := updated["extFields"].([]any)
	if len(ext) != 1 || ext[0].(map[string]any)["key"] != "strength" {
		t.Fatalf("extFields jsonb 回读不符: %v", updated["extFields"])
	}
	if updated["summaryName"] != "摘要" {
		t.Fatalf("merge 未提字段应保留: %v", updated)
	}
	rec = doReq(t, s, "GET", "/api/report-names/nope-"+suffix, "", hdr)
	if rec.Code != 404 {
		t.Fatalf("未知 code get 应 404，得 %d", rec.Code)
	}

	// list：wrapDict 信封 + keyword
	rec = doReq(t, s, "GET", "/api/report-names?keyword="+code, "", hdr)
	m := jsonBody(t, rec)
	if m["page"].(float64) != 1 || m["total"].(float64) != 1 {
		t.Fatalf("list 信封不符: %v", m)
	}

	rec = doReq(t, s, "DELETE", "/api/report-names/"+code, "", hdr)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete 期望 204，得 %d", rec.Code)
	}
	rec = doReq(t, s, "DELETE", "/api/report-names/"+code, "", hdr)
	if rec.Code != 404 {
		t.Fatalf("复删应 404，得 %d", rec.Code)
	}
}

// TestReportNameLinks 三组 junction：object↔report-name / report-name↔standard(role)
// / report-name↔parameter；link 重复幂等、unlink 幂等 204、list 短信封。
// fn: M06.F07.I06
// fn: M06.F07.I07
// fn: M06.F07.I08
// fn: M06.F04.I05
// fn: M06.F04.I06
// fn: M06.F04.I07
func TestReportNameLinks(t *testing.T) {
	s := dictServer(t)
	hdr := dictHdr(t)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	objCode := seedObject(t, s, hdr, suffix)
	stdCode, prmCode := "rn-std-"+suffix, "rn-prm-"+suffix
	mustCreateStandard(t, s, hdr, stdCode, suffix)
	mustCreateParameter(t, s, hdr, prmCode, suffix)
	rnCode := "rn-link-" + suffix
	rec := doJSON(t, s, "POST", "/api/report-names",
		map[string]any{"code": rnCode, "name": "报告" + suffix}, hdr)
	if rec.Code != 200 {
		t.Fatalf("report-name create 期望 200，得 %d", rec.Code)
	}

	// object ↔ report-name：重复 link 幂等（upsert 覆盖 remark）
	for i, remark := range []string{"首次", "覆盖"} {
		rec = doJSON(t, s, "POST", "/api/report-names/links/object", map[string]any{
			"inspectionObjectCode": objCode, "reportNameCode": rnCode, "remark": remark}, hdr)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("object link #%d 期望 204，得 %d", i, rec.Code)
		}
	}
	rec = doReq(t, s, "GET",
		"/api/report-names/links/object?inspectionObjectCode="+objCode, "", hdr)
	m := jsonBody(t, rec)
	if m["page"].(float64) != 1 || m["total"].(float64) != 1 {
		t.Fatalf("object links 信封不符: %v", m)
	}
	if item := m["items"].([]any)[0].(map[string]any); item["remark"] != "覆盖" {
		t.Fatalf("upsert 应覆盖 remark: %v", item)
	}

	// report-name ↔ standard：role 必填
	rec = doJSON(t, s, "POST", "/api/report-names/links/standard", map[string]any{
		"reportNameCode": rnCode, "inspectionStandardCode": stdCode}, hdr)
	if rec.Code != 400 {
		t.Fatalf("role 必填应 400，得 %d", rec.Code)
	}
	rec = doJSON(t, s, "POST", "/api/report-names/links/standard", map[string]any{
		"reportNameCode": rnCode, "inspectionStandardCode": stdCode, "role": "TESTING"}, hdr)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("standard link 期望 204，得 %d", rec.Code)
	}
	rec = doReq(t, s, "GET",
		"/api/report-names/links/standard?reportNameCode="+rnCode+"&role=TESTING", "", hdr)
	item := jsonBody(t, rec)["items"].([]any)[0].(map[string]any)
	if item["role"] != "TESTING" {
		t.Fatalf("standard links DTO 不符: %v", item)
	}

	// report-name ↔ parameter
	rec = doJSON(t, s, "POST", "/api/report-names/links/parameter", map[string]any{
		"reportNameCode": rnCode, "inspectionParameterCode": prmCode}, hdr)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("parameter link 期望 204，得 %d", rec.Code)
	}
	rec = doReq(t, s, "GET",
		"/api/report-names/links/parameter?reportNameCode="+rnCode+
			"&inspectionParameterCode="+prmCode, "", hdr)
	if got := len(jsonBody(t, rec)["items"].([]any)); got != 1 {
		t.Fatalf("parameter links 应 1 条，得 %d", got)
	}

	// 三组 unlink 幂等 204
	for i := 0; i < 2; i++ {
		rec = doJSON(t, s, "DELETE", "/api/report-names/links/object", map[string]any{
			"inspectionObjectCode": objCode, "reportNameCode": rnCode}, hdr)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("object unlink #%d 期望幂等 204，得 %d", i, rec.Code)
		}
		rec = doJSON(t, s, "DELETE", "/api/report-names/links/standard", map[string]any{
			"reportNameCode": rnCode, "inspectionStandardCode": stdCode, "role": "TESTING"}, hdr)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("standard unlink #%d 期望幂等 204，得 %d", i, rec.Code)
		}
		rec = doJSON(t, s, "DELETE", "/api/report-names/links/parameter", map[string]any{
			"reportNameCode": rnCode, "inspectionParameterCode": prmCode}, hdr)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("parameter unlink #%d 期望幂等 204，得 %d", i, rec.Code)
		}
	}
}
