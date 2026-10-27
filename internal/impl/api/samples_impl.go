package api

// samples_impl.go — 样品面 handler（REQ-2026-004 Task 10）。
//
// 12 端点：samples 6（M03.F03.I01-I05，business 信封 pageSize 缺省 20 +
// updateSampleExt ext 独立端点）+ test-records 6（M03.F03.I06-I11，
// setVerdict 独立动词 PATCH /{id}/verdict）。薄层装配：守卫 → service →
// 信封/状态码。

import (
	"net/http"

	genapi "lab-management-system-go/internal/gen/api"
	familyerrors "lab-management-system-go/internal/impl/errors"
	"lab-management-system-go/internal/impl/render"
	"lab-management-system-go/internal/impl/service"
)

// ---- M03.F03 samples ----

func (s *Server) spSvc() *service.SampleService { return service.NewSampleService(s.DB) }

// spTenant Bearer claim 的 tenant_id。
func (s *Server) spTenant(w http.ResponseWriter, r *http.Request) (string, bool) {
	claims, ok := s.authenticated(w, r)
	if !ok {
		return "", false
	}
	return claims["tenant_id"].(string), true
}

func (s *Server) SamplesListSamples(w http.ResponseWriter, r *http.Request, p genapi.SamplesListSamplesParams) {
	tenantID, ok := s.spTenant(w, r)
	if !ok {
		return
	}
	items, err := s.spSvc().ListSamples(tenantID, derefStr(p.ReceiptId), derefStr(p.Keyword))
	if err != nil {
		familyerrors.Write(w, err)
		return
	}
	page, pageSize := 1, 20
	if p.Page != nil {
		page = int(*p.Page)
	}
	if p.PageSize != nil {
		pageSize = int(*p.PageSize)
	}
	render.WriteJSON(w, http.StatusOK, map[string]any{
		"items": items, "page": page, "pageSize": pageSize, "total": len(items)})
}

func (s *Server) SamplesGetSample(w http.ResponseWriter, r *http.Request, id string) {
	tenantID, ok := s.spTenant(w, r)
	if !ok {
		return
	}
	dto, err := s.spSvc().GetSample(tenantID, id)
	if err != nil {
		familyerrors.Write(w, err)
		return
	}
	render.WriteJSON(w, http.StatusOK, dto)
}

func (s *Server) SamplesCreateSample(w http.ResponseWriter, r *http.Request) {
	var body genapi.CreateSampleRequest
	decodeBodyInto(r, &body)
	tenantID, ok := s.spTenant(w, r)
	if !ok {
		return
	}
	in := service.CreateSampleInput{
		TenantID: tenantID, ReceiptID: body.ReceiptId, SampleCode: body.SampleCode,
		SampleName: body.SampleName, Model: body.Model, Specification: body.Specification,
		Grade: body.Grade, Brand: body.Brand, Manufacturer: body.Manufacturer,
		StructuralPart: body.StructuralPart, RepresentQuantity: body.RepresentQuantity,
		SampleQuantity: body.SampleQuantity, BatchNumber: body.BatchNumber,
		SupplyUnit: body.SupplyUnit, ArrivalDate: body.ArrivalDate,
		SamplingDate: body.SamplingDate, CuringCondition: body.CuringCondition,
		Age: body.Age, Ext: body.Ext, Remark: body.Remark,
	}
	dto, err := s.spSvc().CreateSample(in)
	if err != nil {
		familyerrors.Write(w, err)
		return
	}
	render.WriteJSON(w, http.StatusOK, dto)
}

func (s *Server) SamplesUpdateSample(w http.ResponseWriter, r *http.Request, id string) {
	var body genapi.UpdateSampleRequest
	decodeBodyInto(r, &body)
	tenantID, ok := s.spTenant(w, r)
	if !ok {
		return
	}
	dto, err := s.spSvc().UpdateSample(tenantID, id,
		body.SampleName, body.Model, body.Specification, body.Grade, body.Brand,
		body.Manufacturer, body.StructuralPart, body.RepresentQuantity,
		body.SampleQuantity, body.BatchNumber, body.SupplyUnit,
		body.ArrivalDate, body.SamplingDate, body.CuringCondition, body.Age,
		body.Ext, body.Remark)
	if err != nil {
		familyerrors.Write(w, err)
		return
	}
	render.WriteJSON(w, http.StatusOK, dto)
}

func (s *Server) SamplesUpdateSampleExt(w http.ResponseWriter, r *http.Request, id string) {
	var body genapi.UpdateSampleExtRequest
	decodeBodyInto(r, &body)
	tenantID, ok := s.spTenant(w, r)
	if !ok {
		return
	}
	var ext *map[string]string
	if body.Ext != nil { // 缺 ext 键 → 400（rails req['ext'].nil? 镜像）
		ext = &body.Ext
	}
	dto, err := s.spSvc().UpdateSampleExt(tenantID, id, ext)
	if err != nil {
		familyerrors.Write(w, err)
		return
	}
	render.WriteJSON(w, http.StatusOK, dto)
}

func (s *Server) SamplesDeleteSample(w http.ResponseWriter, r *http.Request, id string) {
	tenantID, ok := s.spTenant(w, r)
	if !ok {
		return
	}
	if err := s.spSvc().DeleteSample(tenantID, id); err != nil {
		familyerrors.Write(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- M03.F03 test-records ----

func (s *Server) trcSvc() *service.TestRecordService {
	return service.NewTestRecordService(s.DB)
}

func (s *Server) TestRecordsListTestRecords(w http.ResponseWriter, r *http.Request, p genapi.TestRecordsListTestRecordsParams) {
	tenantID, ok := s.spTenant(w, r)
	if !ok {
		return
	}
	items, err := s.trcSvc().ListTestRecords(tenantID, derefStr(p.SampleId))
	if err != nil {
		familyerrors.Write(w, err)
		return
	}
	page, pageSize := 1, 20
	if p.Page != nil {
		page = int(*p.Page)
	}
	if p.PageSize != nil {
		pageSize = int(*p.PageSize)
	}
	render.WriteJSON(w, http.StatusOK, map[string]any{
		"items": items, "page": page, "pageSize": pageSize, "total": len(items)})
}

func (s *Server) TestRecordsGetTestRecord(w http.ResponseWriter, r *http.Request, id string) {
	tenantID, ok := s.spTenant(w, r)
	if !ok {
		return
	}
	dto, err := s.trcSvc().GetTestRecord(tenantID, id)
	if err != nil {
		familyerrors.Write(w, err)
		return
	}
	render.WriteJSON(w, http.StatusOK, dto)
}

func (s *Server) TestRecordsCreateTestRecord(w http.ResponseWriter, r *http.Request) {
	var body genapi.CreateTestRecordRequest
	decodeBodyInto(r, &body)
	tenantID, ok := s.spTenant(w, r)
	if !ok {
		return
	}
	in := service.CreateTestRecordInput{
		TenantID: tenantID, SampleID: body.SampleId, ParameterCode: body.ParameterCode,
		StandardCode: body.StandardCode, RequirementCode: body.RequirementCode,
		Requirement: body.Requirement, Result: body.Result, Verdict: body.Verdict,
	}
	dto, err := s.trcSvc().CreateTestRecord(in)
	if err != nil {
		familyerrors.Write(w, err)
		return
	}
	render.WriteJSON(w, http.StatusOK, dto)
}

func (s *Server) TestRecordsUpdateTestRecord(w http.ResponseWriter, r *http.Request, id string) {
	var body genapi.UpdateTestRecordRequest
	decodeBodyInto(r, &body)
	tenantID, ok := s.spTenant(w, r)
	if !ok {
		return
	}
	dto, err := s.trcSvc().UpdateTestRecord(tenantID, id,
		body.ParameterCode, body.StandardCode, body.RequirementCode,
		body.Requirement, body.Result, body.Verdict)
	if err != nil {
		familyerrors.Write(w, err)
		return
	}
	render.WriteJSON(w, http.StatusOK, dto)
}

func (s *Server) TestRecordsSetVerdict(w http.ResponseWriter, r *http.Request, id string) {
	var body genapi.TestRecordsSetVerdictJSONBody
	decodeBodyInto(r, &body)
	tenantID, ok := s.spTenant(w, r)
	if !ok {
		return
	}
	dto, err := s.trcSvc().SetVerdict(tenantID, id, body.Verdict)
	if err != nil {
		familyerrors.Write(w, err)
		return
	}
	render.WriteJSON(w, http.StatusOK, dto)
}

func (s *Server) TestRecordsDeleteTestRecord(w http.ResponseWriter, r *http.Request, id string) {
	tenantID, ok := s.spTenant(w, r)
	if !ok {
		return
	}
	if err := s.trcSvc().DeleteTestRecord(tenantID, id); err != nil {
		familyerrors.Write(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
