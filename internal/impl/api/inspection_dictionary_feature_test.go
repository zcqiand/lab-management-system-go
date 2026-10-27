package api

// inspection_dictionary_feature_test.go — 检测能力字典 Feature 测试（REQ-2026-004 Task 7）。
//
// 镜像 rails InspectionDictionary::Specialties/Objects/Parameters/Standards +
// InspectionJunctionService 语义：四实体 CRUD（业务 code 主键，平台级无 tenant），
// list 信封（page 缺省 1、pageSize 缺省 = items.size），keyword 大小写不敏感
// LIKE code/name，更新 PATCH 语义（truthy/strict/key 三模式）；junction link
// 重复幂等 204（upsert 整行覆盖）、unlink 缺失幂等 204、list 短信封
// （page 恒 1、pageSize=total、DTO 无时间戳）、link 关键键缺失 400。
//
// 真库 lab_test，WithTx 整体回滚（testsupport.Tx）——不污染种子态。

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"lab-management-system-go/internal/testsupport"
)

// dictServer 每用例一条事务贯穿（复用 catalog 的 env 签发链）。
func dictServer(t *testing.T) *Server {
	t.Helper()
	setHandlerEnv(t)
	return &Server{DB: testsupport.Tx(t)}
}

// dictHdr Bearer 头（字典端点全部 JwtGuard 保护）。
func dictHdr(t *testing.T) map[string]string {
	return map[string]string{"Authorization": "Bearer " + catalogToken(t, "TENANT-001")}
}

// TestSpecialtyCycle 专项四件套：必填 400 → create 默认值 → list 信封+keyword →
// PATCH 更新（未提字段保留）→ delete 204/复删 404。
// fn: M06.F01.I01
// fn: M06.F01.I02
// fn: M06.F01.I03
// fn: M06.F01.I04
func TestSpecialtyCycle(t *testing.T) {
	s := dictServer(t)
	hdr := dictHdr(t)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())

	// create：code/officialNo/name 必填，缺 officialNo 400
	rec := doJSON(t, s, "POST", "/api/inspection/specialties",
		map[string]any{"code": "spc-" + suffix, "name": "缺号专项"}, hdr)
	if rec.Code != 400 {
		t.Fatalf("缺 officialNo 应 400，得 %d", rec.Code)
	}
	rec = doJSON(t, s, "POST", "/api/inspection/specialties", map[string]any{
		"code": "spc-" + suffix, "officialNo": "GB" + suffix, "name": "专项甲"}, hdr)
	if rec.Code != 200 {
		t.Fatalf("create 期望 200，得 %d: %s", rec.Code, rec.Body.String())
	}
	created := jsonBody(t, rec)
	if created["officialNo"] != "GB"+suffix || created["isOfficial"] != true ||
		created["enabled"] != true || created["sortOrder"].(float64) != 0 {
		t.Fatalf("create 默认值不符: %v", created)
	}

	// list：keyword 圈定 + 信封（page 缺省 1、pageSize 缺省 = items.size）
	rec = doReq(t, s, "GET", "/api/inspection/specialties?keyword=spc-"+suffix, "", hdr)
	if rec.Code != 200 {
		t.Fatalf("list 期望 200，得 %d", rec.Code)
	}
	m := jsonBody(t, rec)
	if m["page"].(float64) != 1 || m["total"].(float64) != 1 || m["pageSize"].(float64) != 1 {
		t.Fatalf("信封不符: %v", m)
	}
	// keyword 大小写不敏感
	rec = doReq(t, s, "GET", "/api/inspection/specialties?keyword=SPC-"+suffix, "", hdr)
	if got := len(jsonBody(t, rec)["items"].([]any)); got != 1 {
		t.Fatalf("keyword 大小写不敏感应 1 条，得 %d", got)
	}

	// update PATCH：只传 name，officialNo 保留
	rec = doJSON(t, s, "PUT", "/api/inspection/specialties/spc-"+suffix,
		map[string]any{"name": "专项甲改"}, hdr)
	if rec.Code != 200 {
		t.Fatalf("update 期望 200，得 %d: %s", rec.Code, rec.Body.String())
	}
	updated := jsonBody(t, rec)
	if updated["name"] != "专项甲改" || updated["officialNo"] != "GB"+suffix {
		t.Fatalf("PATCH 未提字段应保留: %v", updated)
	}
	rec = doJSON(t, s, "PUT", "/api/inspection/specialties/nope-"+suffix,
		map[string]any{"name": "x"}, hdr)
	if rec.Code != 404 {
		t.Fatalf("未知 code update 应 404，得 %d", rec.Code)
	}

	// delete 204 + 复删 404
	rec = doReq(t, s, "DELETE", "/api/inspection/specialties/spc-"+suffix, "", hdr)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete 期望 204，得 %d", rec.Code)
	}
	rec = doReq(t, s, "DELETE", "/api/inspection/specialties/spc-"+suffix, "", hdr)
	if rec.Code != 404 {
		t.Fatalf("复删应 404，得 %d", rec.Code)
	}
}

// TestObjectCycle 项目四件套：5 必填（缺 sourceProjectNo 400）+ 默认
// isOptionalForQualification=false + 专项过滤 list。
// fn: M06.F02.I01
// fn: M06.F02.I02
// fn: M06.F02.I03
// fn: M06.F02.I04
func TestObjectCycle(t *testing.T) {
	s := dictServer(t)
	hdr := dictHdr(t)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	spCode := "sp-obj-" + suffix
	mustCreateSpecialty(t, s, hdr, spCode, suffix)

	rec := doJSON(t, s, "POST", "/api/inspection/objects", map[string]any{
		"code": "obj-" + suffix, "inspectionSpecialtyCode": spCode,
		"sourceProjectName": "项目甲", "name": "对象甲"}, hdr)
	if rec.Code != 400 {
		t.Fatalf("缺 sourceProjectNo 应 400，得 %d", rec.Code)
	}
	rec = doJSON(t, s, "POST", "/api/inspection/objects", map[string]any{
		"code": "obj-" + suffix, "inspectionSpecialtyCode": spCode,
		"sourceProjectNo": "PRJ-" + suffix, "sourceProjectName": "项目甲",
		"name": "对象甲"}, hdr)
	if rec.Code != 200 {
		t.Fatalf("create 期望 200，得 %d: %s", rec.Code, rec.Body.String())
	}
	created := jsonBody(t, rec)
	if created["isOptionalForQualification"] != false || created["isOfficial"] != true {
		t.Fatalf("create 默认值不符: %v", created)
	}

	// list：专项过滤 + keyword
	rec = doReq(t, s, "GET",
		"/api/inspection/objects?inspectionSpecialtyCode="+spCode, "", hdr)
	if got := len(jsonBody(t, rec)["items"].([]any)); got != 1 {
		t.Fatalf("专项过滤应 1 条，得 %d", got)
	}

	// update PATCH
	rec = doJSON(t, s, "PUT", "/api/inspection/objects/obj-"+suffix,
		map[string]any{"name": "对象甲改"}, hdr)
	if rec.Code != 200 || jsonBody(t, rec)["sourceProjectNo"] != "PRJ-"+suffix {
		t.Fatalf("PATCH 不符: %d %s", rec.Code, rec.Body.String())
	}
	// delete
	rec = doReq(t, s, "DELETE", "/api/inspection/objects/obj-"+suffix, "", hdr)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete 期望 204，得 %d", rec.Code)
	}
	rec = doReq(t, s, "DELETE", "/api/inspection/objects/obj-"+suffix, "", hdr)
	if rec.Code != 404 {
		t.Fatalf("复删应 404，得 %d", rec.Code)
	}
}

// TestParameterCycle 参数四件套：rawName/canonicalName 缺省 = name、aliases
// 缺省 []、sourceType 缺省 official；sourceType 过滤；aliases 整体替换。
// fn: M06.F03.I01
// fn: M06.F03.I02
// fn: M06.F03.I03
// fn: M06.F03.I04
func TestParameterCycle(t *testing.T) {
	s := dictServer(t)
	hdr := dictHdr(t)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())

	rec := doJSON(t, s, "POST", "/api/inspection/parameters",
		map[string]any{"code": "prm-" + suffix, "name": "参数甲"}, hdr)
	if rec.Code != 200 {
		t.Fatalf("create 期望 200，得 %d: %s", rec.Code, rec.Body.String())
	}
	created := jsonBody(t, rec)
	if created["rawName"] != "参数甲" || created["canonicalName"] != "参数甲" ||
		created["sourceType"] != "official" {
		t.Fatalf("create 缺省派生不符: %v", created)
	}
	if aliases, ok := created["aliases"].([]any); !ok || len(aliases) != 0 {
		t.Fatalf("aliases 缺省应 []: %v", created["aliases"])
	}

	// list：sourceType 过滤（自定义的不在此列）
	rec = doReq(t, s, "GET", "/api/inspection/parameters?sourceType=custom&keyword=prm-"+suffix, "", hdr)
	if got := len(jsonBody(t, rec)["items"].([]any)); got != 0 {
		t.Fatalf("sourceType=custom 不应命中 official 行，得 %d", got)
	}
	rec = doReq(t, s, "GET", "/api/inspection/parameters?keyword=prm-"+suffix, "", hdr)
	if got := len(jsonBody(t, rec)["items"].([]any)); got != 1 {
		t.Fatalf("keyword 应命中 1 条，得 %d", got)
	}

	// update：aliases 传则整体替换；methodText 可补
	rec = doJSON(t, s, "PUT", "/api/inspection/parameters/prm-"+suffix, map[string]any{
		"aliases": []string{"alias-a", "alias-b"}, "methodText": "称重法"}, hdr)
	if rec.Code != 200 {
		t.Fatalf("update 期望 200，得 %d: %s", rec.Code, rec.Body.String())
	}
	updated := jsonBody(t, rec)
	aliases := updated["aliases"].([]any)
	if len(aliases) != 2 || aliases[0] != "alias-a" || updated["methodText"] != "称重法" {
		t.Fatalf("aliases 替换不符: %v", updated)
	}
	// delete
	rec = doReq(t, s, "DELETE", "/api/inspection/parameters/prm-"+suffix, "", hdr)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete 期望 204，得 %d", rec.Code)
	}
	rec = doReq(t, s, "DELETE", "/api/inspection/parameters/prm-"+suffix, "", hdr)
	if rec.Code != 404 {
		t.Fatalf("复删应 404，得 %d", rec.Code)
	}
}

// TestStandardCycle 标准四件套：status 缺省 active；status 过滤。
// fn: M06.F04.I01
// fn: M06.F04.I02
// fn: M06.F04.I03
// fn: M06.F04.I04
func TestStandardCycle(t *testing.T) {
	s := dictServer(t)
	hdr := dictHdr(t)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())

	rec := doJSON(t, s, "POST", "/api/inspection/standards",
		map[string]any{"code": "std-" + suffix, "name": "标准甲"}, hdr)
	if rec.Code != 200 {
		t.Fatalf("create 期望 200，得 %d: %s", rec.Code, rec.Body.String())
	}
	created := jsonBody(t, rec)
	if created["status"] != "active" || created["sortOrder"].(float64) != 0 {
		t.Fatalf("create 默认值不符: %v", created)
	}

	rec = doReq(t, s, "GET", "/api/inspection/standards?status=superseded&keyword=std-"+suffix, "", hdr)
	if got := len(jsonBody(t, rec)["items"].([]any)); got != 0 {
		t.Fatalf("status=superseded 不应命中 active 行，得 %d", got)
	}

	rec = doJSON(t, s, "PUT", "/api/inspection/standards/std-"+suffix,
		map[string]any{"status": "superseded", "version": "2026"}, hdr)
	if rec.Code != 200 || jsonBody(t, rec)["status"] != "superseded" {
		t.Fatalf("PATCH status 不符: %d %s", rec.Code, rec.Body.String())
	}

	rec = doReq(t, s, "DELETE", "/api/inspection/standards/std-"+suffix, "", hdr)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete 期望 204，得 %d", rec.Code)
	}
	rec = doReq(t, s, "DELETE", "/api/inspection/standards/std-"+suffix, "", hdr)
	if rec.Code != 404 {
		t.Fatalf("复删应 404，得 %d", rec.Code)
	}
}

// TestJunctionSpecialtyObject 专项↔项目：link 重复幂等 204（upsert 覆盖 remark）、
// unlink 缺失幂等 204、list 短信封 + DTO 无时间戳。
// fn: M06.F02.I05
// fn: M06.F02.I06
// fn: M06.F02.I09
func TestJunctionSpecialtyObject(t *testing.T) {
	s := dictServer(t)
	hdr := dictHdr(t)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	spCode, objCode := mustCreateSpecialty(t, s, hdr, "sp-j1-"+suffix, suffix), ""
	rec := doJSON(t, s, "POST", "/api/inspection/objects", map[string]any{
		"code": "obj-j1-" + suffix, "inspectionSpecialtyCode": spCode,
		"sourceProjectNo": "PRJ-" + suffix, "sourceProjectName": "项目", "name": "对象"}, hdr)
	if rec.Code != 200 {
		t.Fatalf("object create 期望 200，得 %d", rec.Code)
	}
	objCode = "obj-j1-" + suffix

	// link：body 必填键缺失 400
	rec = doJSON(t, s, "POST", "/api/inspection/links/specialty-object",
		map[string]any{"inspectionSpecialtyCode": spCode}, hdr)
	if rec.Code != 400 {
		t.Fatalf("link 缺键应 400，得 %d", rec.Code)
	}
	// link + 重复 link（幂等 204，整行覆盖 remark）
	for i, remark := range []string{"首次", "覆盖"} {
		rec = doJSON(t, s, "POST", "/api/inspection/links/specialty-object",
			map[string]any{"inspectionSpecialtyCode": spCode,
				"inspectionObjectCode": objCode, "remark": remark}, hdr)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("link #%d 期望 204，得 %d", i, rec.Code)
		}
	}

	// list：短信封 page 恒 1 + DTO 展开形状（无时间戳）
	rec = doReq(t, s, "GET", "/api/inspection/links/specialty-object?inspectionSpecialtyCode="+spCode, "", hdr)
	m := jsonBody(t, rec)
	if m["page"].(float64) != 1 || m["total"].(float64) != 1 || m["pageSize"].(float64) != 1 {
		t.Fatalf("list 信封不符（重复 link 应仍 1 行）: %v", m)
	}
	item := m["items"].([]any)[0].(map[string]any)
	if item["inspectionSpecialtyCode"] != spCode || item["inspectionObjectCode"] != objCode ||
		item["remark"] != "覆盖" {
		t.Fatalf("list DTO 不符（upsert 应覆盖 remark）: %v", item)
	}
	if _, has := item["createdAt"]; has {
		t.Fatalf("junction DTO 不含时间戳: %v", item)
	}

	// unlink：命中 + 缺失均幂等 204
	for i := 0; i < 2; i++ {
		rec = doJSON(t, s, "DELETE", "/api/inspection/links/specialty-object",
			map[string]any{"inspectionSpecialtyCode": spCode,
				"inspectionObjectCode": objCode}, hdr)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("unlink #%d 期望幂等 204，得 %d", i, rec.Code)
		}
	}
	rec = doReq(t, s, "GET", "/api/inspection/links/specialty-object?inspectionSpecialtyCode="+spCode, "", hdr)
	if got := len(jsonBody(t, rec)["items"].([]any)); got != 0 {
		t.Fatalf("unlink 后 list 应空，得 %d", got)
	}
}

// TestJunctionObjectParameter 项目↔参数：qualificationLevel 缺省 QUALIFIED、
// sourcePage 落库；list 双 code 过滤；unlink body 幂等。
// fn: M06.F02.I07
// fn: M06.F02.I08
// fn: M06.F02.I10
func TestJunctionObjectParameter(t *testing.T) {
	s := dictServer(t)
	hdr := dictHdr(t)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	objCode, prmCode := seedObject(t, s, hdr, suffix), "prm-j2-"+suffix
	mustCreateParameter(t, s, hdr, prmCode, suffix)

	rec := doJSON(t, s, "POST", "/api/inspection/links/object-parameter", map[string]any{
		"inspectionObjectCode": objCode, "inspectionParameterCode": prmCode,
		"sourcePage": 12}, hdr)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("link 期望 204，得 %d: %s", rec.Code, rec.Body.String())
	}

	rec = doReq(t, s, "GET",
		"/api/inspection/links/object-parameter?inspectionObjectCode="+objCode+
			"&inspectionParameterCode="+prmCode, "", hdr)
	m := jsonBody(t, rec)
	item := m["items"].([]any)[0].(map[string]any)
	if item["qualificationLevel"] != "QUALIFIED" || item["sourcePage"].(float64) != 12 {
		t.Fatalf("link 缺省/载荷不符: %v", item)
	}

	// unlink（body 传双 code）幂等 204
	for i := 0; i < 2; i++ {
		rec = doJSON(t, s, "DELETE", "/api/inspection/links/object-parameter", map[string]any{
			"inspectionObjectCode": objCode, "inspectionParameterCode": prmCode}, hdr)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("unlink #%d 期望幂等 204，得 %d", i, rec.Code)
		}
	}
}

// TestJunctionObjectStandard 项目↔标准(role)：role 必填（缺失 400）；
// list 按 role 过滤；unlink 幂等。
// fn: M06.F01.I05
// fn: M06.F01.I06
// fn: M06.F01.I07
func TestJunctionObjectStandard(t *testing.T) {
	s := dictServer(t)
	hdr := dictHdr(t)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	objCode := seedObject(t, s, hdr, suffix)
	stdCode := "std-j3-" + suffix
	mustCreateStandard(t, s, hdr, stdCode, suffix)

	rec := doJSON(t, s, "POST", "/api/inspection/links/object-standard", map[string]any{
		"inspectionObjectCode": objCode, "inspectionStandardCode": stdCode}, hdr)
	if rec.Code != 400 {
		t.Fatalf("role 必填应 400，得 %d", rec.Code)
	}
	rec = doJSON(t, s, "POST", "/api/inspection/links/object-standard", map[string]any{
		"inspectionObjectCode": objCode, "inspectionStandardCode": stdCode,
		"role": "TESTING"}, hdr)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("link 期望 204，得 %d: %s", rec.Code, rec.Body.String())
	}

	// list：role 过滤（JUDGMENT 不应命中 TESTING 行）
	rec = doReq(t, s, "GET",
		"/api/inspection/links/object-standard?inspectionObjectCode="+objCode+"&role=JUDGMENT", "", hdr)
	if got := len(jsonBody(t, rec)["items"].([]any)); got != 0 {
		t.Fatalf("role=JUDGMENT 不应命中，得 %d", got)
	}
	rec = doReq(t, s, "GET",
		"/api/inspection/links/object-standard?inspectionObjectCode="+objCode+"&role=TESTING", "", hdr)
	item := jsonBody(t, rec)["items"].([]any)[0].(map[string]any)
	if item["role"] != "TESTING" {
		t.Fatalf("list DTO role 不符: %v", item)
	}

	rec = doJSON(t, s, "DELETE", "/api/inspection/links/object-standard", map[string]any{
		"inspectionObjectCode": objCode, "inspectionStandardCode": stdCode,
		"role": "TESTING"}, hdr)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("unlink 期望 204，得 %d", rec.Code)
	}
	rec = doJSON(t, s, "DELETE", "/api/inspection/links/object-standard", map[string]any{
		"inspectionObjectCode": objCode, "inspectionStandardCode": stdCode,
		"role": "TESTING"}, hdr)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("unlink 缺失应幂等 204，得 %d", rec.Code)
	}
}

// TestJunctionStandardParameter 标准↔参数：无载荷双键 junction，unlink body 幂等。
// fn: M06.F03.I05
// fn: M06.F03.I06
// fn: M06.F03.I08
func TestJunctionStandardParameter(t *testing.T) {
	s := dictServer(t)
	hdr := dictHdr(t)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	stdCode, prmCode := "std-j4-"+suffix, "prm-j4-"+suffix
	mustCreateStandard(t, s, hdr, stdCode, suffix)
	mustCreateParameter(t, s, hdr, prmCode, suffix)

	rec := doJSON(t, s, "POST", "/api/inspection/links/standard-parameter", map[string]any{
		"inspectionStandardCode": stdCode, "inspectionParameterCode": prmCode}, hdr)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("link 期望 204，得 %d: %s", rec.Code, rec.Body.String())
	}
	rec = doReq(t, s, "GET",
		"/api/inspection/links/standard-parameter?inspectionStandardCode="+stdCode+
			"&inspectionParameterCode="+prmCode, "", hdr)
	m := jsonBody(t, rec)
	if m["total"].(float64) != 1 {
		t.Fatalf("list 应 1 行: %v", m)
	}
	item := m["items"].([]any)[0].(map[string]any)
	if item["inspectionStandardCode"] != stdCode || item["inspectionParameterCode"] != prmCode {
		t.Fatalf("list DTO 不符: %v", item)
	}

	for i := 0; i < 2; i++ {
		rec = doJSON(t, s, "DELETE", "/api/inspection/links/standard-parameter", map[string]any{
			"inspectionStandardCode": stdCode, "inspectionParameterCode": prmCode}, hdr)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("unlink #%d 期望幂等 204，得 %d", i, rec.Code)
		}
	}
}

// ---- 造数 helpers（均走 API，同事务） ----

func mustCreateSpecialty(t *testing.T, s *Server, hdr map[string]string, code, suffix string) string {
	t.Helper()
	rec := doJSON(t, s, "POST", "/api/inspection/specialties", map[string]any{
		"code": code, "officialNo": "GB" + suffix, "name": "专项" + suffix}, hdr)
	if rec.Code != 200 {
		t.Fatalf("specialty create 期望 200，得 %d: %s", rec.Code, rec.Body.String())
	}
	return code
}

func mustCreateParameter(t *testing.T, s *Server, hdr map[string]string, code, suffix string) {
	t.Helper()
	rec := doJSON(t, s, "POST", "/api/inspection/parameters",
		map[string]any{"code": code, "name": "参数" + suffix}, hdr)
	if rec.Code != 200 {
		t.Fatalf("parameter create 期望 200，得 %d: %s", rec.Code, rec.Body.String())
	}
}

func mustCreateStandard(t *testing.T, s *Server, hdr map[string]string, code, suffix string) {
	t.Helper()
	rec := doJSON(t, s, "POST", "/api/inspection/standards",
		map[string]any{"code": code, "name": "标准" + suffix}, hdr)
	if rec.Code != 200 {
		t.Fatalf("standard create 期望 200，得 %d: %s", rec.Code, rec.Body.String())
	}
}

// seedObject 造专项+项目（项目 FK 依赖专项）。
func seedObject(t *testing.T, s *Server, hdr map[string]string, suffix string) string {
	t.Helper()
	spCode := mustCreateSpecialty(t, s, hdr, "sp-seed-"+suffix, suffix)
	rec := doJSON(t, s, "POST", "/api/inspection/objects", map[string]any{
		"code": "obj-seed-" + suffix, "inspectionSpecialtyCode": spCode,
		"sourceProjectNo": "PRJ-" + suffix, "sourceProjectName": "项目", "name": "对象"}, hdr)
	if rec.Code != 200 {
		t.Fatalf("object create 期望 200，得 %d: %s", rec.Code, rec.Body.String())
	}
	return "obj-seed-" + suffix
}
