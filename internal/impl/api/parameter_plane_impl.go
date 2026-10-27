package api

// parameter_plane_impl.go — 参数面 handler（REQ-2026-004 Task 8）。
//
// 18 端点：计算方法 5（M06.F05）+ 参数界面 8（M06.F08 + M06.F03.I07 unlink）+
// 技术要求 5（M06.F06）。薄层装配：守卫 → service → 裸数组/信封/状态码。
// 计算方法与技术要求 list 为裸数组（rails render_camel(service.list) 直出，
// 无信封）；参数界面 list 用 wrapDict 信封、list-links 用短信封。

import (
	"net/http"

	genapi "lab-management-system-go/internal/gen/api"
	familyerrors "lab-management-system-go/internal/impl/errors"
	"lab-management-system-go/internal/impl/render"
	"lab-management-system-go/internal/impl/service"
)

// ---- M06.F05 calculation-methods ----

func (s *Server) CalculationMethodsListCalculationMethods(w http.ResponseWriter, r *http.Request, p genapi.CalculationMethodsListCalculationMethodsParams) {
	if _, ok := s.authenticated(w, r); !ok {
		return
	}
	items, err := s.calcSvc().ListCalcMethods(derefStr(p.InspectionObjectCode),
		derefStr(p.InspectionParameterCode))
	if err != nil {
		familyerrors.Write(w, err)
		return
	}
	render.WriteJSON(w, http.StatusOK, items)
}

func (s *Server) calcSvc() *service.CalculationMethodService {
	return service.NewCalculationMethodService(s.DB)
}

func (s *Server) CalculationMethodsGetCalculationMethod(w http.ResponseWriter, r *http.Request, inspectionObjectCode string, inspectionParameterCode string) {
	if _, ok := s.authenticated(w, r); !ok {
		return
	}
	dto, err := s.calcSvc().GetCalcMethod(inspectionObjectCode, inspectionParameterCode)
	if err != nil {
		familyerrors.Write(w, err)
		return
	}
	render.WriteJSON(w, http.StatusOK, dto)
}

func (s *Server) CalculationMethodsCreateCalculationMethod(w http.ResponseWriter, r *http.Request) {
	var body genapi.CreateCalculationMethodRequest
	decodeBodyInto(r, &body)
	if _, ok := s.authenticated(w, r); !ok {
		return
	}
	in := service.CreateCalcMethodInput{
		InspectionObjectCode:    body.InspectionObjectCode,
		InspectionParameterCode: body.InspectionParameterCode,
		TestingStandardCode:     body.TestingStandardCode,
		ReportNameCode:          body.ReportNameCode,
		SpecimenCount:           body.SpecimenCount,
		Formula:                 body.Formula,
		Conditions:              body.Conditions,
		RoundingRule:            body.RoundingRule,
		Remark:                  body.Remark,
		SortOrder:               body.SortOrder,
	}
	if body.AlgorithmType != nil {
		in.AlgorithmType = string(*body.AlgorithmType)
	}
	dto, err := s.calcSvc().CreateCalcMethod(in)
	if err != nil {
		familyerrors.Write(w, err)
		return
	}
	render.WriteJSON(w, http.StatusOK, dto)
}

func (s *Server) CalculationMethodsUpdateCalculationMethod(w http.ResponseWriter, r *http.Request, inspectionObjectCode string, inspectionParameterCode string) {
	var body genapi.UpdateCalculationMethodRequest
	decodeBodyInto(r, &body)
	if _, ok := s.authenticated(w, r); !ok {
		return
	}
	var algorithmType string
	if body.AlgorithmType != nil {
		algorithmType = string(*body.AlgorithmType)
	}
	dto, err := s.calcSvc().UpdateCalcMethod(inspectionObjectCode, inspectionParameterCode,
		derefStr(body.TestingStandardCode), derefStr(body.ReportNameCode), algorithmType,
		body.SpecimenCount, body.SortOrder, body.Formula, body.Conditions,
		body.RoundingRule, body.Remark)
	if err != nil {
		familyerrors.Write(w, err)
		return
	}
	render.WriteJSON(w, http.StatusOK, dto)
}

func (s *Server) CalculationMethodsDeleteCalculationMethod(w http.ResponseWriter, r *http.Request, inspectionObjectCode string, inspectionParameterCode string) {
	if _, ok := s.authenticated(w, r); !ok {
		return
	}
	if err := s.calcSvc().DeleteCalcMethod(inspectionObjectCode, inspectionParameterCode); err != nil {
		familyerrors.Write(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- M06.F08 param-interfaces ----

func (s *Server) piSvc() *service.ParamInterfaceService {
	return service.NewParamInterfaceService(s.DB)
}

func (s *Server) ParamInterfacesListParamInterfaces(w http.ResponseWriter, r *http.Request, p genapi.ParamInterfacesListParamInterfacesParams) {
	if _, ok := s.authenticated(w, r); !ok {
		return
	}
	items, err := s.piSvc().ListParamInterfaces(derefStr(p.Keyword))
	if err != nil {
		familyerrors.Write(w, err)
		return
	}
	page, pageSize := 1, len(items)
	if p.Page != nil {
		page = int(*p.Page)
	}
	if p.PageSize != nil {
		pageSize = int(*p.PageSize)
	}
	render.WriteJSON(w, http.StatusOK, map[string]any{
		"items": items, "page": page, "pageSize": pageSize, "total": len(items)})
}

func (s *Server) ParamInterfacesGetParamInterface(w http.ResponseWriter, r *http.Request, code string) {
	if _, ok := s.authenticated(w, r); !ok {
		return
	}
	dto, err := s.piSvc().GetParamInterface(code)
	if err != nil {
		familyerrors.Write(w, err)
		return
	}
	render.WriteJSON(w, http.StatusOK, dto)
}

func (s *Server) ParamInterfacesCreateParamInterface(w http.ResponseWriter, r *http.Request) {
	var body genapi.CreateParamInterfaceRequest
	decodeBodyInto(r, &body)
	if _, ok := s.authenticated(w, r); !ok {
		return
	}
	in := service.CreateParamInterfaceInput{
		Code: body.Code, ComponentPath: body.ComponentPath,
	}
	if body.Config != nil {
		in.Config = *body.Config
	}
	if body.Name != nil {
		in.Name = *body.Name
	}
	if body.Description != nil {
		in.Description = *body.Description
	}
	dto, err := s.piSvc().CreateParamInterface(in)
	if err != nil {
		familyerrors.Write(w, err)
		return
	}
	render.WriteJSON(w, http.StatusOK, dto)
}

func (s *Server) ParamInterfacesUpdateParamInterface(w http.ResponseWriter, r *http.Request, code string) {
	var body genapi.UpdateParamInterfaceRequest
	decodeBodyInto(r, &body)
	if _, ok := s.authenticated(w, r); !ok {
		return
	}
	var name, description string
	if body.Name != nil {
		name = *body.Name
	}
	if body.Description != nil {
		description = *body.Description
	}
	var cfg map[string]any
	if body.Config != nil {
		cfg = *body.Config
	}
	dto, err := s.piSvc().UpdateParamInterface(code, name, derefStr(body.ComponentPath), description,
		body.IsOfficial, body.SortOrder, cfg)
	if err != nil {
		familyerrors.Write(w, err)
		return
	}
	render.WriteJSON(w, http.StatusOK, dto)
}

func (s *Server) ParamInterfacesDeleteParamInterface(w http.ResponseWriter, r *http.Request, code string) {
	if _, ok := s.authenticated(w, r); !ok {
		return
	}
	if err := s.piSvc().DeleteParamInterface(code); err != nil {
		familyerrors.Write(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- M06.F08.I06 / M06.F03.I07 parameter ↔ param-interface junction ----

func (s *Server) ParamInterfacesLinkParamInterface(w http.ResponseWriter, r *http.Request) {
	var body genapi.ParamInterfaceLink
	decodeBodyInto(r, &body)
	if _, ok := s.authenticated(w, r); !ok {
		return
	}
	var cfg map[string]any
	if body.Config != nil {
		cfg = *body.Config
	}
	if err := s.piSvc().LinkParamInterface(body.InspectionParameterCode,
		body.ParamInterfaceCode, body.ReportNameCode, cfg); err != nil {
		familyerrors.Write(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) ParamInterfacesUnlinkParamInterface(w http.ResponseWriter, r *http.Request) {
	var body genapi.ParamInterfacesUnlinkParamInterfaceJSONBody
	decodeBodyInto(r, &body)
	if _, ok := s.authenticated(w, r); !ok {
		return
	}
	if err := s.piSvc().UnlinkParamInterface(body.InspectionParameterCode,
		body.ParamInterfaceCode); err != nil {
		familyerrors.Write(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) ParamInterfacesListParamInterfaceLinks(w http.ResponseWriter, r *http.Request, p genapi.ParamInterfacesListParamInterfaceLinksParams) {
	if _, ok := s.authenticated(w, r); !ok {
		return
	}
	items, err := s.piSvc().ListParamInterfaceLinks(derefStr(p.InspectionParameterCode),
		derefStr(p.ParamInterfaceCode))
	if err != nil {
		familyerrors.Write(w, err)
		return
	}
	render.WriteJSON(w, http.StatusOK, map[string]any{
		"items": items, "page": 1, "pageSize": len(items), "total": len(items)})
}

// ---- M06.F06 technical-requirements ----

func (s *Server) trSvc() *service.TechnicalRequirementService {
	return service.NewTechnicalRequirementService(s.DB)
}

// trTenant Bearer claim 的 tenant_id（守卫保证存在）。
func (s *Server) trTenant(w http.ResponseWriter, r *http.Request) (string, bool) {
	claims, ok := s.authenticated(w, r)
	if !ok {
		return "", false
	}
	return claims["tenant_id"].(string), true
}

func (s *Server) TechnicalRequirementsListTechnicalRequirements(w http.ResponseWriter, r *http.Request, p genapi.TechnicalRequirementsListTechnicalRequirementsParams) {
	tenantID, ok := s.trTenant(w, r)
	if !ok {
		return
	}
	items, err := s.trSvc().ListTechReqs(tenantID, derefStr(p.InspectionObjectCode),
		derefStr(p.InspectionParameterCode), derefStr(p.JudgmentStandardCode),
		enumStr(p.VerificationStatus))
	if err != nil {
		familyerrors.Write(w, err)
		return
	}
	render.WriteJSON(w, http.StatusOK, items)
}

func (s *Server) TechnicalRequirementsGetTechnicalRequirement(w http.ResponseWriter, r *http.Request, inspectionObjectCode string, inspectionParameterCode string, judgmentStandardCode string) {
	tenantID, ok := s.trTenant(w, r)
	if !ok {
		return
	}
	dto, err := s.trSvc().GetTechReq(tenantID, inspectionObjectCode,
		inspectionParameterCode, judgmentStandardCode)
	if err != nil {
		familyerrors.Write(w, err)
		return
	}
	render.WriteJSON(w, http.StatusOK, dto)
}

func (s *Server) TechnicalRequirementsCreateTechnicalRequirement(w http.ResponseWriter, r *http.Request) {
	var body genapi.CreateTechnicalRequirementRequest
	decodeBodyInto(r, &body)
	tenantID, ok := s.trTenant(w, r)
	if !ok {
		return
	}
	in := service.CreateTechReqInput{
		TenantID:                tenantID,
		InspectionObjectCode:    body.InspectionObjectCode,
		InspectionParameterCode: body.InspectionParameterCode,
		JudgmentStandardCode:    body.JudgmentStandardCode,
		Conditions:              body.Conditions,
		TargetValue:             body.TargetValue,
		Expression:              body.Expression,
		Unit:                    body.Unit,
		MinValue:                body.MinValue,
		MaxValue:                body.MaxValue,
		SourcePage:              body.SourcePage,
		Clause:                  body.Clause,
		SourceHash:              body.SourceHash,
		Brand:                   body.Brand,
		Model:                   body.Model,
		Grade:                   body.Grade,
		Spec:                    body.Spec,
		Sieve:                   body.Sieve,
		Remark:                  body.Remark,
		SortOrder:               body.SortOrder,
	}
	if body.ValueType != nil {
		in.ValueType = string(*body.ValueType)
	}
	if body.Comparison != nil {
		in.Comparison = string(*body.Comparison)
	}
	if body.JudgmentMode != nil {
		in.JudgmentMode = string(*body.JudgmentMode)
	}
	if body.VerificationStatus != nil {
		in.VerificationStatus = string(*body.VerificationStatus)
	}
	dto, err := s.trSvc().CreateTechReq(in)
	if err != nil {
		familyerrors.Write(w, err)
		return
	}
	render.WriteJSON(w, http.StatusOK, dto)
}

func (s *Server) TechnicalRequirementsUpdateTechnicalRequirement(w http.ResponseWriter, r *http.Request, inspectionObjectCode string, inspectionParameterCode string, judgmentStandardCode string) {
	var body genapi.UpdateTechnicalRequirementRequest
	decodeBodyInto(r, &body)
	tenantID, ok := s.trTenant(w, r)
	if !ok {
		return
	}
	in := service.UpdateTechReqInput{
		Conditions: body.Conditions, TargetValue: body.TargetValue,
		Expression: body.Expression, Unit: body.Unit,
		MinValue: body.MinValue, MaxValue: body.MaxValue, SourcePage: body.SourcePage,
		Clause: body.Clause, SourceHash: body.SourceHash,
		Brand: body.Brand, Model: body.Model, Grade: body.Grade,
		Spec: body.Spec, Sieve: body.Sieve, Remark: body.Remark, SortOrder: body.SortOrder,
	}
	if body.ValueType != nil {
		in.ValueType = strPtrVal(string(*body.ValueType))
	}
	if body.Comparison != nil {
		in.Comparison = strPtrVal(string(*body.Comparison))
	}
	if body.JudgmentMode != nil {
		in.JudgmentMode = strPtrVal(string(*body.JudgmentMode))
	}
	if body.VerificationStatus != nil {
		in.VerificationStatus = strPtrVal(string(*body.VerificationStatus))
	}
	dto, err := s.trSvc().UpdateTechReq(tenantID, inspectionObjectCode,
		inspectionParameterCode, judgmentStandardCode, in)
	if err != nil {
		familyerrors.Write(w, err)
		return
	}
	render.WriteJSON(w, http.StatusOK, dto)
}

func (s *Server) TechnicalRequirementsDeleteTechnicalRequirement(w http.ResponseWriter, r *http.Request, inspectionObjectCode string, inspectionParameterCode string, judgmentStandardCode string) {
	tenantID, ok := s.trTenant(w, r)
	if !ok {
		return
	}
	if err := s.trSvc().DeleteTechReq(tenantID, inspectionObjectCode,
		inspectionParameterCode, judgmentStandardCode); err != nil {
		familyerrors.Write(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// strPtrVal 值 → 指针。
func strPtrVal(s string) *string { return &s }
