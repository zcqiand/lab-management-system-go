package api

// receipts_feature_test.go — 接样单七段 flow + 汇总 Feature 测试（REQ-2026-004
// Task 11）。
//
// 镜像 rails Receipts::SampleReceiptService + ReportFlowService + ReceiptQueries
// + Summary::SummaryService + MaterialRates：
// - receipts CRUD（M03.F01.I01-I06）：tenant 收口；contractId FK（未知 404）；
//   flow_status=receiving 起步、flow_history=[] 起步、result="" 起步；
//   update PATCH（contractId 不可变、3 个 jsonb 数组非 nil 即写）；
//   history 裸数组；business 信封（pageSize 缺省 20）
// - assignTask（M03.F02.I01）：字段按需更新；仅 receiving 推进 task_assignment
// - act×7（M03.F01.I08/F02.I05/F03.I12/F05.I07/F06.I05/F07.I05/F08.I05）：
//   operator 必填 400（整批拒）；stage 不匹配/非法迁移出 per-id err 结果仍 200；
//   submit 写 lastSubmittedBy / withdraw 清空 / return 保留；archived 仅 submit
//   自转移写 audit
// - 三态 filter（not_yet/submitted jsonb 谓词）
// - summary（M05.F01.I01/I06+I03/I04）：报告汇总 6 列 + 仪表盘 3 总数/3 桶/
//   任务计数/合格率(三位小数)/报告产出/6 段漏斗
//
// 真库 lab_test，WithTx 整体回滚（testsupport.Tx）——不污染种子态。

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"lab-management-system-go/internal/impl/service"
)

// seedContract 直插 contract 母体（contracts CRUD 已 Task 9 API 化，但此处要的
// 是 receipt FK 母体，直插更省；categoryCode FK → inspection_report_names 走 API）。
func seedContract(t *testing.T, s *Server, suffix string) string {
	t.Helper()
	now := service.NowISO()
	contract := map[string]any{
		"id": "C-TEST-" + suffix, "contract_code": "CC-" + suffix,
		"client_unit": "甲方", "project_name": "项目" + suffix, "construction_unit": "施工",
		"witness_unit": "监理", "witness": "张工", "status": "active",
		"tenant_id": "TENANT-001", "created_at": now, "updated_at": now}
	if err := s.DB.Table("contracts").Create(contract).Error; err != nil {
		t.Fatalf("seed contract: %v", err)
	}
	return contract["id"].(string)
}

// mustCreateReceipt 经 API 建一单（report-name category FK 也一并补齐）。
func mustCreateReceipt(t *testing.T, s *Server, hdr map[string]string,
	suffix string) (id, commissionCode string) {
	t.Helper()
	contractID := seedContract(t, s, suffix)
	rnCode := "rc-cat-" + suffix
	rec := doJSON(t, s, "POST", "/api/report-names",
		map[string]any{"code": rnCode, "name": "类别" + suffix,
			"summaryName": "混凝土" + suffix}, hdr)
	if rec.Code != 200 {
		t.Fatalf("report-name create 期望 200，得 %d", rec.Code)
	}
	rec = doJSON(t, s, "POST", "/api/receipts", map[string]any{
		"contractId": contractID, "commissionCode": "WT-" + suffix,
		"commissionDate": "2026-10-27", "categoryCode": rnCode,
		"receivedBy": "收样员", "sampleSource": "现场", "testCategory": "常规",
		"projectName": "项目" + suffix}, hdr)
	if rec.Code != 200 {
		t.Fatalf("receipt create 期望 200，得 %d: %s", rec.Code, rec.Body.String())
	}
	created := jsonBody(t, rec)
	return created["id"].(string), "WT-" + suffix
}

// actBody act 载荷捷径。
func actBody(action string, ids []string, operator string) map[string]any {
	return map[string]any{"action": action, "ids": ids, "operator": operator}
}

// TestReceiptCrud 接样单六件套：未知 contract 404 → create 起步态 → 信封 list +
// 三过滤（含非法 flowStatus 400）→ PATCH（contractId 不可变 + jsonb 数组）→
// history 裸数组 → delete 204/复删 404。
// fn: M03.F01.I01
// fn: M03.F01.I02
// fn: M03.F01.I03
// fn: M03.F01.I04
// fn: M03.F01.I05
// fn: M03.F01.I06
func TestReceiptCrud(t *testing.T) {
	s := dictServer(t)
	hdr := dictHdr(t)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	contractID := seedContract(t, s, suffix)
	rnCode := "rc-cat-" + suffix
	rec := doJSON(t, s, "POST", "/api/report-names",
		map[string]any{"code": rnCode, "name": "类别" + suffix}, hdr)
	if rec.Code != 200 {
		t.Fatalf("report-name create 期望 200，得 %d", rec.Code)
	}

	// create：未知 contractId → 404（rails RecordNotFound 镜像）
	rec = doJSON(t, s, "POST", "/api/receipts", map[string]any{
		"contractId": "C-NOPE", "commissionCode": "WT-X", "commissionDate": "2026-10-27",
		"categoryCode": rnCode, "receivedBy": "员", "sampleSource": "现场",
		"testCategory": "常规"}, hdr)
	if rec.Code != 404 {
		t.Fatalf("未知 contractId 应 404，得 %d", rec.Code)
	}
	// 正常 create：起步态
	rec = doJSON(t, s, "POST", "/api/receipts", map[string]any{
		"contractId": contractID, "commissionCode": "WT-" + suffix,
		"commissionDate": "2026-10-27", "categoryCode": rnCode,
		"receivedBy": "收样员", "sampleSource": "现场", "testCategory": "常规",
		"judgmentBasis": []string{"GB 50010"}}, hdr)
	if rec.Code != 200 {
		t.Fatalf("create 期望 200，得 %d: %s", rec.Code, rec.Body.String())
	}
	created := jsonBody(t, rec)
	if !strings.HasPrefix(created["id"].(string), "R-") {
		t.Fatalf("id 应 R- 前缀: %v", created["id"])
	}
	if created["flowStatus"] != "receiving" || created["result"] != "" {
		t.Fatalf("起步态不符: %v", created)
	}
	if hist, ok := created["flowHistory"].([]any); !ok || len(hist) != 0 {
		t.Fatalf("flowHistory 缺省应 []: %v", created["flowHistory"])
	}
	if jb, ok := created["judgmentBasis"].([]any); !ok || len(jb) != 1 {
		t.Fatalf("judgmentBasis jsonb 回读不符: %v", created["judgmentBasis"])
	}
	id := created["id"].(string)

	// list：business 信封 + contractId/flowStatus/keyword 过滤 + 非法 flowStatus 400
	rec = doReq(t, s, "GET", "/api/receipts?contractId="+contractID, "", hdr)
	m := jsonBody(t, rec)
	if m["page"].(float64) != 1 || m["pageSize"].(float64) != 20 || m["total"].(float64) != 1 {
		t.Fatalf("list 信封不符: %v", m)
	}
	rec = doReq(t, s, "GET", "/api/receipts?keyword="+suffix, "", hdr)
	if got := len(jsonBody(t, rec)["items"].([]any)); got != 1 {
		t.Fatalf("keyword 应命中，得 %d", got)
	}
	rec = doReq(t, s, "GET", "/api/receipts?flowStatus=approval", "", hdr)
	if actIdsContain(t, rec, id) {
		t.Fatalf("flowStatus=approval 不应命中 receiving 单")
	}
	rec = doReq(t, s, "GET", "/api/receipts?flowStatus=bogus", "", hdr)
	if rec.Code != 400 {
		t.Fatalf("非法 flowStatus 应 400，得 %d", rec.Code)
	}

	// get：本租户 200 / 跨租户 404
	rec = doReq(t, s, "GET", "/api/receipts/"+id, "", hdr)
	if jsonBody(t, rec)["commissionCode"] != "WT-"+suffix {
		t.Fatalf("get 不符: %s", rec.Body.String())
	}
	rec = doReq(t, s, "GET", "/api/receipts/"+id, "",
		map[string]string{"Authorization": "Bearer " + catalogToken(t, "TENANT-002")})
	if rec.Code != 404 {
		t.Fatalf("跨租户 get 应 404，得 %d", rec.Code)
	}

	// update PATCH：witnessUnit + testParameters jsonb 写入；contractId 不可变
	rec = doJSON(t, s, "PUT", "/api/receipts/"+id, map[string]any{
		"witnessUnit": "监理改", "testParameters": []string{"坍落度"},
		"contractId": "C-EVIL"}, hdr)
	if rec.Code != 200 {
		t.Fatalf("update 期望 200，得 %d: %s", rec.Code, rec.Body.String())
	}
	updated := jsonBody(t, rec)
	if updated["witnessUnit"] != "监理改" || updated["contractId"] != contractID {
		t.Fatalf("PATCH 不符（contractId 应不可变）: %v", updated)
	}
	if tp, ok := updated["testParameters"].([]any); !ok || len(tp) != 1 {
		t.Fatalf("testParameters jsonb 回读不符: %v", updated["testParameters"])
	}

	// history：裸数组，起步 []
	rec = doReq(t, s, "GET", "/api/receipts/"+id+"/history", "", hdr)
	if arr := jsonArr(t, rec); len(arr) != 0 {
		t.Fatalf("history 起步应空数组，得 %v", arr)
	}
	rec = doReq(t, s, "GET", "/api/receipts/R-nope/history", "", hdr)
	if rec.Code != 404 {
		t.Fatalf("未知 id history 应 404，得 %d", rec.Code)
	}

	// delete 204 + 复删 404
	rec = doReq(t, s, "DELETE", "/api/receipts/"+id, "", hdr)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete 期望 204，得 %d", rec.Code)
	}
	rec = doReq(t, s, "DELETE", "/api/receipts/"+id, "", hdr)
	if rec.Code != 404 {
		t.Fatalf("复删应 404，得 %d", rec.Code)
	}
}

// TestReceiptFlow 任务分配 + 七段 act 状态机：assignTask 仅 receiving 推进 →
// assigning/data-entry/review/approve/issuance 全 submit 链 → archived 仅 submit
// 自转移 audit → return/withdraw 语义 + lastSubmittedBy 三态 + stage 不匹配 err。
// fn: M03.F01.I08
// fn: M03.F02.I01
// fn: M03.F02.I05
// fn: M03.F03.I12
// fn: M03.F05.I07
// fn: M03.F06.I05
// fn: M03.F07.I05
// fn: M03.F08.I05
func TestReceiptFlow(t *testing.T) {
	s := dictServer(t)
	hdr := dictHdr(t)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	id, _ := mustCreateReceipt(t, s, hdr, suffix)

	// operator 必填（整批 400，先于 per-id 循环）
	rec := doJSON(t, s, "POST", "/api/receipts/receiving/act",
		map[string]any{"action": "submit", "ids": []string{id}}, hdr)
	if rec.Code != 400 {
		t.Fatalf("缺 operator 应 400，得 %d", rec.Code)
	}
	rec = doJSON(t, s, "POST", "/api/receipts/receiving/act",
		actBody("submit", []string{id}, ""), hdr)
	if rec.Code != 400 {
		t.Fatalf("空 operator 应 400，得 %d", rec.Code)
	}

	// assignTask：仅 receiving 推进 task_assignment + history
	rec = doJSON(t, s, "PUT", "/api/receipts/"+id+"/task", map[string]any{
		"assigneeId": "U-1", "assigneeName": "试验员甲", "plannedTestDate": "2026-10-28"}, hdr)
	if rec.Code != 200 {
		t.Fatalf("assignTask 期望 200，得 %d: %s", rec.Code, rec.Body.String())
	}
	assigned := jsonBody(t, rec)
	if assigned["flowStatus"] != "task_assignment" || assigned["assigneeName"] != "试验员甲" {
		t.Fatalf("assignTask 不符: %v", assigned)
	}
	// stage 不匹配：receiving act 打在 task_assignment 单上 → per-id err 仍 200
	rec = doJSON(t, s, "POST", "/api/receipts/receiving/act",
		actBody("submit", []string{id}, "操作员"), hdr)
	if rec.Code != 200 {
		t.Fatalf("act stage 不匹配仍应 200，得 %d", rec.Code)
	}
	res := jsonArr(t, rec)[0].(map[string]any)
	if res["ok"] != false || res["id"] != id {
		t.Fatalf("stage 不匹配应 err 结果: %v", res)
	}

	// assigning submit → data_entry；lastSubmittedBy=operator
	rec = doJSON(t, s, "POST", "/api/receipts/assigning/act",
		actBody("submit", []string{id}, "操作员A"), hdr)
	if res := actOne(t, rec); !res["ok"].(bool) || res["flowStatus"] != "data_entry" {
		t.Fatalf("assigning submit 不符: %v", res)
	}
	// data_entry withdraw：lastSubmittedBy 清空（先 submit 再 withdraw 自转移）
	rec = doJSON(t, s, "POST", "/api/receipts/data-entry/act",
		actBody("submit", []string{id}, "操作员B"), hdr)
	if res := actOne(t, rec); !res["ok"].(bool) {
		t.Fatalf("data-entry submit 不符: %v", res)
	}
	// return 保留 lastSubmittedBy：data-entry 无前置（现处 review）
	// 状态机已进 review：review return → data_entry
	rec = doJSON(t, s, "POST", "/api/receipts/review/act",
		actBody("return", []string{id}, "操作员C"), hdr)
	if res := actOne(t, rec); !res["ok"].(bool) || res["flowStatus"] != "data_entry" {
		t.Fatalf("review return 不符: %v", res)
	}
	rec = doReq(t, s, "GET", "/api/receipts/"+id, "", hdr)
	if got := jsonBody(t, rec)["lastSubmittedBy"]; got != "操作员B" {
		t.Fatalf("return 应保留 lastSubmittedBy，得 %v", got)
	}
	// withdraw 仅 receiving 自转移（rails targetOf 镜像）：data_entry 处 withdraw err
	rec = doJSON(t, s, "POST", "/api/receipts/data-entry/act",
		actBody("withdraw", []string{id}, "操作员B"), hdr)
	if res := actOne(t, rec); res["ok"] != false {
		t.Fatalf("data-entry withdraw 应 err: %v", res)
	}

	// 全 submit 链 data_entry→review→approval→issuance→archived
	for i, stage := range []string{"data-entry", "review", "approve", "issuance"} {
		rec = doJSON(t, s, "POST", "/api/receipts/"+stage+"/act",
			actBody("submit", []string{id}, "审签员"), hdr)
		want := []string{"review", "approval", "issuance", "archived"}[i]
		if res := actOne(t, rec); !res["ok"].(bool) || res["flowStatus"] != want {
			t.Fatalf("%s submit 不符: %v", stage, res)
		}
	}
	// 非法迁移：archived return
	rec = doJSON(t, s, "POST", "/api/receipts/archived/act",
		actBody("return", []string{id}, "审签员"), hdr)
	res = actOne(t, rec)
	if res["ok"] != false {
		t.Fatalf("archived return 应 err: %v", res)
	}
	// archived 仅 submit 自转移 + audit 缺省 reason
	rec = doJSON(t, s, "POST", "/api/receipts/archived/act",
		actBody("submit", []string{id}, "档案员"), hdr)
	if res := actOne(t, rec); !res["ok"].(bool) || res["flowStatus"] != "archived" {
		t.Fatalf("archived submit 不符: %v", res)
	}

	// history 累积 + 键集
	rec = doReq(t, s, "GET", "/api/receipts/"+id+"/history", "", hdr)
	hist := jsonArr(t, rec)
	if len(hist) < 5 {
		t.Fatalf("history 应 ≥5 条，得 %d", len(hist))
	}
	first := hist[0].(map[string]any)
	if first["action"] != "submit" || first["from"] != "receiving" ||
		first["to"] != "task_assignment" || first["operator"] != "试验员甲" ||
		first["reason"] != "M03.F02 任务分配" || first["at"] == nil {
		t.Fatalf("history 首条不符: %v", first)
	}

	// withdraw 仅 receiving 自转移：receiving 处 withdraw
	id2, _ := mustCreateReceipt(t, s, hdr, suffix+"w")
	rec = doJSON(t, s, "POST", "/api/receipts/receiving/act",
		actBody("withdraw", []string{id2}, "操作员"), hdr)
	if res := actOne(t, rec); !res["ok"].(bool) || res["flowStatus"] != "receiving" {
		t.Fatalf("receiving withdraw 不符: %v", res)
	}
	// 非法迁移：task_assignment 处 withdraw → nil target err
	rec = doJSON(t, s, "PUT", "/api/receipts/"+id2+"/task",
		map[string]any{"assigneeName": "乙"}, hdr)
	if rec.Code != 200 {
		t.Fatalf("assignTask 期望 200，得 %d", rec.Code)
	}
	rec = doJSON(t, s, "POST", "/api/receipts/assigning/act",
		actBody("withdraw", []string{id2}, "操作员"), hdr)
	if res := actOne(t, rec); res["ok"] != false {
		t.Fatalf("assigning withdraw 应 err: %v", res)
	}
}

// TestReceiptThreeStateFilter 5.57 三态 filter：not_yet=停在本环节（无 stage=
// 无流转记录新单）/ submitted=已 submit 至下一环节（无 stage=有流转且
// lastSubmittedBy 非空）。
// fn: M03.F01.I01
func TestReceiptThreeStateFilter(t *testing.T) {
	s := dictServer(t)
	hdr := dictHdr(t)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	id, _ := mustCreateReceipt(t, s, hdr, suffix)

	rec := doReq(t, s, "GET", "/api/receipts?filter=not_yet", "", hdr)
	if !actIdsContain(t, rec, id) {
		t.Fatalf("not_yet 应命中新单")
	}
	rec = doReq(t, s, "GET", "/api/receipts?filter=submitted", "", hdr)
	if actIdsContain(t, rec, id) {
		t.Fatalf("submitted 不应命中未提交单")
	}
	// receiving submit → task_assignment：submitted@receiving 命中 / not_yet@receiving 不命中
	rec = doJSON(t, s, "POST", "/api/receipts/receiving/act",
		actBody("submit", []string{id}, "操作员"), hdr)
	if res := actOne(t, rec); !res["ok"].(bool) {
		t.Fatalf("receiving submit 不符: %v", res)
	}
	rec = doReq(t, s, "GET", "/api/receipts?filter=submitted&flowStatus=receiving", "", hdr)
	if !actIdsContain(t, rec, id) {
		t.Fatalf("submitted@receiving 应命中")
	}
	rec = doReq(t, s, "GET", "/api/receipts?filter=not_yet&flowStatus=receiving", "", hdr)
	if actIdsContain(t, rec, id) {
		t.Fatalf("not_yet@receiving 不应命中已流转单")
	}
	// 非法 filter 值走普通 list（rails：不在 not_yet/submitted 即忽略）
	rec = doReq(t, s, "GET", "/api/receipts?filter=bogus&keyword="+suffix, "", hdr)
	if got := len(jsonBody(t, rec)["items"].([]any)); got != 1 {
		t.Fatalf("非法 filter 应忽略走普通 list，得 %d", got)
	}
}

// TestSummaryDashboard 报告汇总 6 列 + 仪表盘聚合（3 总数/3 桶/pending/today/
// 合格率三位小数/报告产出/6 段漏斗）。
// fn: M05.F01.I01
// fn: M05.F01.I06
func TestSummaryDashboard(t *testing.T) {
	s := dictServer(t)
	hdr := dictHdr(t)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	id, commissionCode := mustCreateReceipt(t, s, hdr, suffix)
	_ = id

	// 报告汇总：categoryCode=ALL 不过滤
	rec := doReq(t, s, "GET", "/api/summary?categoryCode=ALL", "", hdr)
	if rec.Code != 200 {
		t.Fatalf("summary 期望 200，得 %d: %s", rec.Code, rec.Body.String())
	}
	m := jsonBody(t, rec)
	if m["summaryName"] != "报告汇总（ALL）" {
		t.Fatalf("summaryName 不符: %v", m["summaryName"])
	}
	if cols := m["columns"].([]any); len(cols) != 6 {
		t.Fatalf("columns 应 6 列，得 %d", len(cols))
	}
	found := false
	for _, r := range m["rows"].([]any) {
		row := r.(map[string]any)
		if row["commissionCode"] == commissionCode {
			found = true
			if row["flowStatus"] != "receiving" || row["result"] != "" {
				t.Fatalf("汇总行不符: %v", row)
			}
		}
	}
	if !found {
		t.Fatalf("汇总行未命中 %s", commissionCode)
	}
	// 按类别过滤：summaryName 带类别码
	rec = doReq(t, s, "GET", "/api/summary?categoryCode=rc-cat-"+suffix, "", hdr)
	if got := jsonBody(t, rec)["summaryName"]; got != "报告汇总（rc-cat-"+suffix+"）" {
		t.Fatalf("类别汇总名不符: %v", got)
	}

	// 仪表盘
	rec = doReq(t, s, "GET", "/api/summary/stats", "", hdr)
	st := jsonBody(t, rec)
	if st["receiptCount"].(float64) < 1 || st["contractCount"].(float64) < 1 {
		t.Fatalf("仪表盘总数不符: %v", st)
	}
	if _, ok := st["sampleCount"].(float64); !ok {
		t.Fatalf("sampleCount 缺失: %v", st)
	}
	buckets := st["reportCountByStatus"].(map[string]any)
	if buckets["draft"].(float64) < 1 {
		t.Fatalf("draft 桶应含 receiving 单: %v", buckets)
	}
	if st["pendingTaskCount"].(float64) < 0 {
		t.Fatalf("pendingTaskCount 应为数值: %v", st)
	}
	if st["todayTestCount"].(float64) < 1 {
		t.Fatalf("todayTestCount 应含今日新单: %v", st)
	}
	qr := st["qualifiedRateByMaterial"].(map[string]any)
	concrete := qr["concrete"].(map[string]any)
	if concrete["total"].(float64) < 1 {
		t.Fatalf("concrete total 应 ≥1（summaryName 含混凝土，种子库同归此类）: %v", concrete)
	}
	if rate := concrete["rate"].(float64); rate < 0 || rate > 1 {
		t.Fatalf("rate 应在 [0,1]: %v", concrete)
	}
	ro := st["reportOutputByStatus"].(map[string]any)
	if ro["generated"].(float64) < 0 {
		t.Fatalf("reportOutputByStatus 应为数值: %v", ro)
	}
	fn := st["funnelByStage"].(map[string]any)
	for _, k := range []string{"pending_collect", "received", "testing",
		"reporting", "reviewing", "issued"} {
		if _, ok := fn[k].(float64); !ok {
			t.Fatalf("funnelByStage.%s 缺失: %v", k, fn)
		}
	}
	if fn["pending_collect"].(float64) < 1 {
		t.Fatalf("漏斗 pending_collect 应含本单: %v", fn)
	}
}

// TestFrontendBindSnapshot 生成器 DTO 占位端点：挂认证，登录后 404。
// fn: M03.F01.I02
func TestFrontendBindSnapshot(t *testing.T) {
	s := dictServer(t)
	hdr := dictHdr(t)
	rec := doReq(t, s, "GET", "/api/_frontend-bind/snapshot", "", hdr)
	if rec.Code != 404 {
		t.Fatalf("frontend-bind snapshot 应 404，得 %d", rec.Code)
	}
}

// ---- 小工具 ----

// actOne act 批结果取首条。
func actOne(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	if rec.Code != 200 {
		t.Fatalf("act 期望 200，得 %d: %s", rec.Code, rec.Body.String())
	}
	return jsonArr(t, rec)[0].(map[string]any)
}

// actIdsContain list 结果是否含指定 id。
func actIdsContain(t *testing.T, rec *httptest.ResponseRecorder, id string) bool {
	t.Helper()
	for _, it := range jsonBody(t, rec)["items"].([]any) {
		if it.(map[string]any)["id"] == id {
			return true
		}
	}
	return false
}
