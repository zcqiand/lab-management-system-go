package api

// parameter_plane_feature_test.go — 参数面 Feature 测试（REQ-2026-004 Task 8）。
//
// 镜像 rails CalculationMethod/ParamInterface/TechnicalRequirement 三 service：
// - 计算方法（M06.F05）：复合双键，平台级；create 同 PK upsert merge（springboot
//   save=merge 镜像，2026-09-29 live 实锤）；缺省 algorithmType=manual /
//   specimenCount=1 / sortOrder=0；list 裸数组（无信封）
// - 参数界面（M06.F08）：code/componentPath 必填；config jsonb DTO 端 null→{}；
//   list 信封；link/unlink 幂等 204、list-links 短信封
// - 技术要求（M06.F06）：业务三键 + tenant 收口；缺省 numeric/≥/manual/draft；
//   list 裸数组 4 过滤
//
// 真库 lab_test，WithTx 整体回滚（testsupport.Tx）——不污染种子态。

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// jsonArr 裸数组响应解析（list 无信封端点）。
func jsonArr(t *testing.T, rec *httptest.ResponseRecorder) []any {
	t.Helper()
	var arr []any
	if err := json.Unmarshal(rec.Body.Bytes(), &arr); err != nil {
		t.Fatalf("响应非数组: %v: %s", err, rec.Body.String())
	}
	return arr
}

// TestCalculationMethodCycle 计算方法五件套：缺键 400 → create 缺省 → 同 PK
// upsert merge → get/list/update/delete。
// fn: M06.F05.I01
// fn: M06.F05.I02
// fn: M06.F05.I03
// fn: M06.F05.I04
// fn: M06.F05.I05
func TestCalculationMethodCycle(t *testing.T) {
	s := dictServer(t)
	hdr := dictHdr(t)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	// 双键列均带 FK —— 先造真父行（object/parameter，同事务）
	objCode := seedObject(t, s, hdr, suffix)
	prmCode := "cm-prm-" + suffix
	mustCreateParameter(t, s, hdr, prmCode, suffix)

	// create：双键必填
	rec := doJSON(t, s, "POST", "/api/calculation-methods",
		map[string]any{"inspectionObjectCode": objCode}, hdr)
	if rec.Code != 400 {
		t.Fatalf("缺 inspectionParameterCode 应 400，得 %d", rec.Code)
	}
	rec = doJSON(t, s, "POST", "/api/calculation-methods", map[string]any{
		"inspectionObjectCode": objCode, "inspectionParameterCode": prmCode,
		"formula": "F = P/B"}, hdr)
	if rec.Code != 200 {
		t.Fatalf("create 期望 200，得 %d: %s", rec.Code, rec.Body.String())
	}
	created := jsonBody(t, rec)
	if created["algorithmType"] != "manual" || created["specimenCount"].(float64) != 1 ||
		created["sortOrder"].(float64) != 0 {
		t.Fatalf("create 缺省值不符: %v", created)
	}

	// 同 PK 再 create：upsert merge（非 500）
	rec = doJSON(t, s, "POST", "/api/calculation-methods", map[string]any{
		"inspectionObjectCode": objCode, "inspectionParameterCode": prmCode,
		"formula": "F = P/B 改", "specimenCount": 3}, hdr)
	if rec.Code != 200 {
		t.Fatalf("同 PK create 应 merge 200，得 %d: %s", rec.Code, rec.Body.String())
	}

	// get 复合键 + 未知 404
	rec = doReq(t, s, "GET",
		"/api/calculation-methods/"+objCode+"/"+prmCode, "", hdr)
	got := jsonBody(t, rec)
	if got["formula"] != "F = P/B 改" || got["specimenCount"].(float64) != 3 {
		t.Fatalf("merge 后 get 不符: %v", got)
	}
	rec = doReq(t, s, "GET", "/api/calculation-methods/nope/x", "", hdr)
	if rec.Code != 404 {
		t.Fatalf("未知键 get 应 404，得 %d", rec.Code)
	}

	// list：裸数组 + 双过滤
	rec = doReq(t, s, "GET",
		"/api/calculation-methods?inspectionObjectCode="+objCode+
			"&inspectionParameterCode="+prmCode, "", hdr)
	arr := jsonArr(t, rec)
	if len(arr) != 1 {
		t.Fatalf("list 双过滤应 1 条，得 %d", len(arr))
	}
	// list 裸数组（无信封）已由上一行断言（[]any 直解）

	// update PATCH
	rec = doJSON(t, s, "PUT", "/api/calculation-methods/"+objCode+"/"+prmCode,
		map[string]any{"roundingRule": "四舍五入"}, hdr)
	if rec.Code != 200 || jsonBody(t, rec)["roundingRule"] != "四舍五入" {
		t.Fatalf("PATCH 不符: %d %s", rec.Code, rec.Body.String())
	}

	// delete 204 + 复删 404
	rec = doReq(t, s, "DELETE", "/api/calculation-methods/"+objCode+"/"+prmCode, "", hdr)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete 期望 204，得 %d", rec.Code)
	}
	rec = doReq(t, s, "DELETE", "/api/calculation-methods/"+objCode+"/"+prmCode, "", hdr)
	if rec.Code != 404 {
		t.Fatalf("复删应 404，得 %d", rec.Code)
	}
}

// TestParamInterfaceCycle 参数界面五件套 + link/unlink/list-links：必填 400、
// config 缺省 {}、config jsonb 落库、信封、link 载荷。
// fn: M06.F08.I01
// fn: M06.F08.I02
// fn: M06.F08.I03
// fn: M06.F08.I04
// fn: M06.F08.I05
// fn: M06.F08.I06
// fn: M06.F03.I07
func TestParamInterfaceCycle(t *testing.T) {
	s := dictServer(t)
	hdr := dictHdr(t)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	prmCode := "pi-prm-" + suffix
	mustCreateParameter(t, s, hdr, prmCode, suffix)

	// create：code/componentPath 必填
	rec := doJSON(t, s, "POST", "/api/param-interfaces",
		map[string]any{"code": "pi-" + suffix}, hdr)
	if rec.Code != 400 {
		t.Fatalf("缺 componentPath 应 400，得 %d", rec.Code)
	}
	rec = doJSON(t, s, "POST", "/api/param-interfaces", map[string]any{
		"code": "pi-" + suffix, "componentPath": "/ui/psi.vue", "name": "界面甲"}, hdr)
	if rec.Code != 200 {
		t.Fatalf("create 期望 200，得 %d: %s", rec.Code, rec.Body.String())
	}
	created := jsonBody(t, rec)
	if cfg, ok := created["config"].(map[string]any); !ok || len(cfg) != 0 {
		t.Fatalf("config 缺省应 {}: %v", created["config"])
	}
	if created["sortOrder"].(float64) != 0 {
		t.Fatalf("sortOrder 缺省 0: %v", created)
	}

	// update：config 写入 + PATCH
	rec = doJSON(t, s, "PUT", "/api/param-interfaces/pi-"+suffix, map[string]any{
		"config": map[string]any{"mode": "auto"}}, hdr)
	if rec.Code != 200 {
		t.Fatalf("update 期望 200，得 %d: %s", rec.Code, rec.Body.String())
	}
	updated := jsonBody(t, rec)
	if cfg := updated["config"].(map[string]any); cfg["mode"] != "auto" {
		t.Fatalf("config jsonb 回读不符: %v", updated["config"])
	}
	// get：config 反序列化恒对象
	rec = doReq(t, s, "GET", "/api/param-interfaces/pi-"+suffix, "", hdr)
	if jsonBody(t, rec)["componentPath"] != "/ui/psi.vue" {
		t.Fatalf("get 不符: %s", rec.Body.String())
	}
	rec = doReq(t, s, "GET", "/api/param-interfaces/nope-"+suffix, "", hdr)
	if rec.Code != 404 {
		t.Fatalf("未知 code get 应 404，得 %d", rec.Code)
	}

	// list：信封 + keyword
	rec = doReq(t, s, "GET", "/api/param-interfaces?keyword=pi-"+suffix, "", hdr)
	m := jsonBody(t, rec)
	if m["page"].(float64) != 1 || m["total"].(float64) != 1 {
		t.Fatalf("list 信封不符: %v", m)
	}

	// link：缺键 400 → link（config jsonb 载荷）→ 重复幂等
	rec = doJSON(t, s, "POST", "/api/param-interfaces/links",
		map[string]any{"inspectionParameterCode": prmCode}, hdr)
	if rec.Code != 400 {
		t.Fatalf("link 缺 paramInterfaceCode 应 400，得 %d", rec.Code)
	}
	for i := 0; i < 2; i++ {
		rec = doJSON(t, s, "POST", "/api/param-interfaces/links", map[string]any{
			"inspectionParameterCode": prmCode, "paramInterfaceCode": "pi-" + suffix,
			"config": map[string]any{"order": 1}}, hdr)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("link #%d 期望 204，得 %d: %s", i, rec.Code, rec.Body.String())
		}
	}

	// list-links：短信封 + DTO 展开（config 对象回读）
	rec = doReq(t, s, "GET",
		"/api/param-interfaces/links?inspectionParameterCode="+prmCode, "", hdr)
	lm := jsonBody(t, rec)
	if lm["page"].(float64) != 1 || lm["total"].(float64) != 1 {
		t.Fatalf("list-links 信封不符（重复 link 应仍 1 行）: %v", lm)
	}
	item := lm["items"].([]any)[0].(map[string]any)
	if item["paramInterfaceCode"] != "pi-"+suffix {
		t.Fatalf("list-links DTO 不符: %v", item)
	}
	if cfg := item["config"].(map[string]any); cfg["order"].(float64) != 1 {
		t.Fatalf("link config jsonb 回读不符: %v", item["config"])
	}

	// unlink：幂等 204（body）
	for i := 0; i < 2; i++ {
		rec = doJSON(t, s, "DELETE", "/api/param-interfaces/links", map[string]any{
			"inspectionParameterCode": prmCode, "paramInterfaceCode": "pi-" + suffix}, hdr)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("unlink #%d 期望幂等 204，得 %d", i, rec.Code)
		}
	}

	// delete 实体 204 + 复删 404
	rec = doReq(t, s, "DELETE", "/api/param-interfaces/pi-"+suffix, "", hdr)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete 期望 204，得 %d", rec.Code)
	}
	rec = doReq(t, s, "DELETE", "/api/param-interfaces/pi-"+suffix, "", hdr)
	if rec.Code != 404 {
		t.Fatalf("复删应 404，得 %d", rec.Code)
	}
}

// TestTechnicalRequirementCycle 技术要求五件套：三键必填 400 → 缺省
// numeric/≥/manual/draft → 同 PK merge → tenant 隔离 → list 过滤 → 删除。
// fn: M06.F06.I01
// fn: M06.F06.I02
// fn: M06.F06.I03
// fn: M06.F06.I04
// fn: M06.F06.I05
func TestTechnicalRequirementCycle(t *testing.T) {
	s := dictServer(t)
	hdr1 := map[string]string{"Authorization": "Bearer " + catalogToken(t, "TENANT-001")}
	hdr2 := map[string]string{"Authorization": "Bearer " + catalogToken(t, "TENANT-002")}
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	// 三键列均带 FK —— 先造真父行（object/parameter/standard，同事务）
	objCode := seedObject(t, s, hdr1, suffix)
	prmCode := "tr-prm-" + suffix
	stdCode := "tr-std-" + suffix
	mustCreateParameter(t, s, hdr1, prmCode, suffix)
	mustCreateStandard(t, s, hdr1, stdCode, suffix)
	body := map[string]any{
		"inspectionObjectCode": objCode, "inspectionParameterCode": prmCode,
		"judgmentStandardCode": stdCode, "minValue": 10, "unit": "MPa"}

	// create：三键必填
	miss := map[string]any{}
	for k, v := range body {
		if k != "judgmentStandardCode" {
			miss[k] = v
		}
	}
	rec := doJSON(t, s, "POST", "/api/technical-requirements", miss, hdr1)
	if rec.Code != 400 {
		t.Fatalf("缺 judgmentStandardCode 应 400，得 %d", rec.Code)
	}
	rec = doJSON(t, s, "POST", "/api/technical-requirements", body, hdr1)
	if rec.Code != 200 {
		t.Fatalf("create 期望 200，得 %d: %s", rec.Code, rec.Body.String())
	}
	created := jsonBody(t, rec)
	if created["valueType"] != "numeric" || created["comparison"] != "≥" ||
		created["judgmentMode"] != "manual" || created["verificationStatus"] != "draft" {
		t.Fatalf("create 枚举缺省不符: %v", created)
	}
	if created["tenantId"] != "TENANT-001" || created["minValue"].(float64) != 10 {
		t.Fatalf("tenant 注入/载荷不符: %v", created)
	}

	// 同 PK 再 create：upsert merge + tenant 仍来自 token
	body2 := map[string]any{}
	for k, v := range body {
		body2[k] = v
	}
	body2["remark"] = "改"
	rec = doJSON(t, s, "POST", "/api/technical-requirements", body2, hdr1)
	if rec.Code != 200 || jsonBody(t, rec)["remark"] != "改" {
		t.Fatalf("同 PK create 应 merge 200: %d %s", rec.Code, rec.Body.String())
	}

	// tenant 隔离：TENANT-002 看不到/不可寻址
	rec = doReq(t, s, "GET",
		"/api/technical-requirements/"+objCode+"/"+prmCode+"/"+stdCode,
		"", hdr2)
	if rec.Code != 404 {
		t.Fatalf("跨租户 get 应 404，得 %d", rec.Code)
	}
	rec = doReq(t, s, "GET", "/api/technical-requirements?inspectionObjectCode="+objCode, "", hdr2)
	if got := len(jsonArr(t, rec)); got != 0 {
		t.Fatalf("跨租户 list 应空，得 %d", got)
	}

	// get：复合三键
	rec = doReq(t, s, "GET",
		"/api/technical-requirements/"+objCode+"/"+prmCode+"/"+stdCode,
		"", hdr1)
	if jsonBody(t, rec)["unit"] != "MPa" {
		t.Fatalf("get 不符: %s", rec.Body.String())
	}

	// list：4 过滤（verificationStatus=active 不应命中 draft 行）
	rec = doReq(t, s, "GET",
		"/api/technical-requirements?inspectionObjectCode="+objCode+
			"&inspectionParameterCode="+prmCode+"&judgmentStandardCode="+stdCode+
			"&verificationStatus=active", "", hdr1)
	if got := len(jsonArr(t, rec)); got != 0 {
		t.Fatalf("verificationStatus=active 不应命中 draft 行，得 %d", got)
	}
	rec = doReq(t, s, "GET",
		"/api/technical-requirements?inspectionObjectCode="+objCode, "", hdr1)
	if got := len(jsonArr(t, rec)); got != 1 {
		t.Fatalf("list 应 1 条，得 %d", got)
	}

	// update PATCH
	rec = doJSON(t, s, "PUT",
		"/api/technical-requirements/"+objCode+"/"+prmCode+"/"+stdCode,
		map[string]any{"verificationStatus": "active", "clause": "6.4.2"}, hdr1)
	if rec.Code != 200 {
		t.Fatalf("update 期望 200，得 %d: %s", rec.Code, rec.Body.String())
	}
	updated := jsonBody(t, rec)
	if updated["verificationStatus"] != "active" || updated["clause"] != "6.4.2" ||
		updated["minValue"].(float64) != 10 {
		t.Fatalf("PATCH 未提字段应保留: %v", updated)
	}

	// delete 204 + 复删 404
	rec = doReq(t, s, "DELETE",
		"/api/technical-requirements/"+objCode+"/"+prmCode+"/"+stdCode,
		"", hdr1)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete 期望 204，得 %d", rec.Code)
	}
	rec = doReq(t, s, "DELETE",
		"/api/technical-requirements/"+objCode+"/"+prmCode+"/"+stdCode,
		"", hdr1)
	if rec.Code != 404 {
		t.Fatalf("复删应 404，得 %d", rec.Code)
	}
}
