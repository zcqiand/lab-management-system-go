package api

// business_plane_impl.go — 商务面 handler（REQ-2026-004 Task 9）。
//
// 19 端点：contracts 5（M02.F01，business 信封 pageSize 缺省 20 + enum_param!
// 非法 400）+ report-names 14（M06.F07 CRUD×5 + 三组 junction link/unlink/list，
// wrapDict 信封/短信封）。薄层装配：守卫 → service → 信封/状态码。

import (
	"net/http"

	genapi "lab-management-system-go/internal/gen/api"
	familyerrors "lab-management-system-go/internal/impl/errors"
	"lab-management-system-go/internal/impl/render"
	"lab-management-system-go/internal/impl/service"
)

// ---- M02.F01 contracts ----

func (s *Server) cSvc() *service.ContractService { return service.NewContractService(s.DB) }

// cTenant Bearer claim 的 tenant_id。
func (s *Server) cTenant(w http.ResponseWriter, r *http.Request) (string, bool) {
	claims, ok := s.authenticated(w, r)
	if !ok {
		return "", false
	}
	return claims["tenant_id"].(string), true
}

func (s *Server) ContractsListContracts(w http.ResponseWriter, r *http.Request, p genapi.ContractsListContractsParams) {
	tenantID, ok := s.cTenant(w, r)
	if !ok {
		return
	}
	status := enumStr(p.Status)
	if status != "" && !service.ValidContractStatus(status) {
		familyerrors.Write(w, familyerrors.ErrBadRequest("unexpected status '"+status+"'"))
		return
	}
	items, err := s.cSvc().ListContracts(tenantID, derefStr(p.Keyword), status)
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

func (s *Server) ContractsGetContract(w http.ResponseWriter, r *http.Request, id string) {
	tenantID, ok := s.cTenant(w, r)
	if !ok {
		return
	}
	dto, err := s.cSvc().GetContract(tenantID, id)
	if err != nil {
		familyerrors.Write(w, err)
		return
	}
	render.WriteJSON(w, http.StatusOK, dto)
}

func (s *Server) ContractsCreateContract(w http.ResponseWriter, r *http.Request) {
	var body genapi.CreateContractRequest
	decodeBodyInto(r, &body)
	tenantID, ok := s.cTenant(w, r)
	if !ok {
		return
	}
	in := service.CreateContractInput{
		TenantID: tenantID, ContractCode: body.ContractCode,
		ClientUnit: body.ClientUnit, ProjectName: body.ProjectName,
		ProjectLocation: body.ProjectLocation, ConstructionUnit: body.ConstructionUnit,
		InspectionSpecialtyCode: body.InspectionSpecialtyCode,
		BuildingUnit:            body.BuildingUnit, SupervisorUnit: body.SupervisorUnit,
		InspectionPerson: body.InspectionPerson, InspectionPhone: body.InspectionPhone,
		WitnessUnit: body.WitnessUnit, Witness: body.Witness,
		WitnessPhone: body.WitnessPhone, ContactPerson: body.ContactPerson,
		ContactPhone: body.ContactPhone, EntrustedDate: body.EntrustedDate,
	}
	if body.Status != nil {
		in.Status = string(*body.Status)
	}
	dto, err := s.cSvc().CreateContract(in)
	if err != nil {
		familyerrors.Write(w, err)
		return
	}
	render.WriteJSON(w, http.StatusOK, dto)
}

func (s *Server) ContractsUpdateContract(w http.ResponseWriter, r *http.Request, id string) {
	var body genapi.UpdateContractRequest
	decodeBodyInto(r, &body)
	tenantID, ok := s.cTenant(w, r)
	if !ok {
		return
	}
	var status string
	if body.Status != nil {
		status = string(*body.Status)
	}
	dto, err := s.cSvc().UpdateContract(tenantID, id,
		derefStr(body.ClientUnit), derefStr(body.ProjectName), body.ProjectLocation,
		derefStr(body.ConstructionUnit), body.InspectionSpecialtyCode,
		body.BuildingUnit, body.SupervisorUnit, body.InspectionPerson, body.InspectionPhone,
		derefStr(body.WitnessUnit), derefStr(body.Witness), body.WitnessPhone,
		body.ContactPerson, body.ContactPhone, body.EntrustedDate, status)
	if err != nil {
		familyerrors.Write(w, err)
		return
	}
	render.WriteJSON(w, http.StatusOK, dto)
}

func (s *Server) ContractsDeleteContract(w http.ResponseWriter, r *http.Request, id string) {
	tenantID, ok := s.cTenant(w, r)
	if !ok {
		return
	}
	if err := s.cSvc().DeleteContract(tenantID, id); err != nil {
		familyerrors.Write(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- M06.F07 report-names ----

func (s *Server) rnSvc() *service.ReportNameService { return service.NewReportNameService(s.DB) }

func (s *Server) ReportNamesListReportNames(w http.ResponseWriter, r *http.Request, p genapi.ReportNamesListReportNamesParams) {
	if _, ok := s.authenticated(w, r); !ok {
		return
	}
	items, err := s.rnSvc().ListReportNames(derefStr(p.Keyword))
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

func (s *Server) ReportNamesGetReportName(w http.ResponseWriter, r *http.Request, code string) {
	if _, ok := s.authenticated(w, r); !ok {
		return
	}
	dto, err := s.rnSvc().GetReportName(code)
	if err != nil {
		familyerrors.Write(w, err)
		return
	}
	render.WriteJSON(w, http.StatusOK, dto)
}

func (s *Server) ReportNamesCreateReportName(w http.ResponseWriter, r *http.Request) {
	var body genapi.CreateInspectionReportNameRequest
	decodeBodyInto(r, &body)
	if _, ok := s.authenticated(w, r); !ok {
		return
	}
	in := service.CreateReportNameInput{
		Code: body.Code, Name: body.Name,
		TemplatePath: derefStr(body.TemplatePath),
		SummaryName:  derefStr(body.SummaryName), Description: derefStr(body.Description),
		SortOrder: body.SortOrder,
	}
	if body.FullName != nil {
		in.FullName = *body.FullName
	}
	if body.ExtFields != nil {
		for _, e := range *body.ExtFields {
			in.ExtFields = append(in.ExtFields, e)
		}
	}
	dto, err := s.rnSvc().CreateReportName(in)
	if err != nil {
		familyerrors.Write(w, err)
		return
	}
	render.WriteJSON(w, http.StatusOK, dto)
}

func (s *Server) ReportNamesUpdateReportName(w http.ResponseWriter, r *http.Request, code string) {
	var body genapi.UpdateInspectionReportNameRequest
	decodeBodyInto(r, &body)
	if _, ok := s.authenticated(w, r); !ok {
		return
	}
	var extFields []any
	if body.ExtFields != nil {
		for _, e := range *body.ExtFields {
			extFields = append(extFields, e)
		}
	}
	dto, err := s.rnSvc().UpdateReportName(code, derefStr(body.Name), derefStr(body.FullName),
		derefStr(body.TemplatePath), derefStr(body.SummaryName), extFields,
		derefStr(body.Description), body.SortOrder)
	if err != nil {
		familyerrors.Write(w, err)
		return
	}
	render.WriteJSON(w, http.StatusOK, dto)
}

func (s *Server) ReportNamesDeleteReportName(w http.ResponseWriter, r *http.Request, code string) {
	if _, ok := s.authenticated(w, r); !ok {
		return
	}
	if err := s.rnSvc().DeleteReportName(code); err != nil {
		familyerrors.Write(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- report-name junction link/unlink/list（9 端点） ----

// linkObjectReportNameBody object ↔ report-name 载荷。
type linkObjectReportNameBody struct {
	InspectionObjectCode string  `json:"inspectionObjectCode"`
	ReportNameCode       string  `json:"reportNameCode"`
	Remark               *string `json:"remark"`
}

// linkReportNameStandardBody report-name ↔ standard 载荷（role 必填）。
type linkReportNameStandardBody struct {
	ReportNameCode         string  `json:"reportNameCode"`
	InspectionStandardCode string  `json:"inspectionStandardCode"`
	Role                   string  `json:"role"`
	Remark                 *string `json:"remark"`
}

// linkReportNameParameterBody report-name ↔ parameter 载荷。
type linkReportNameParameterBody struct {
	ReportNameCode          string  `json:"reportNameCode"`
	InspectionParameterCode string  `json:"inspectionParameterCode"`
	Remark                  *string `json:"remark"`
}

func (s *Server) ReportNamesLinkObjectReportName(w http.ResponseWriter, r *http.Request) {
	var body linkObjectReportNameBody
	decodeBodyInto(r, &body)
	if _, ok := s.authenticated(w, r); !ok {
		return
	}
	if err := s.rnSvc().LinkObjectReportName(body.InspectionObjectCode,
		body.ReportNameCode, body.Remark); err != nil {
		familyerrors.Write(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) ReportNamesUnlinkObjectReportName(w http.ResponseWriter, r *http.Request) {
	var body linkObjectReportNameBody
	decodeBodyInto(r, &body)
	if _, ok := s.authenticated(w, r); !ok {
		return
	}
	if err := s.rnSvc().UnlinkObjectReportName(body.InspectionObjectCode,
		body.ReportNameCode); err != nil {
		familyerrors.Write(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) ReportNamesListObjectReportNameLinks(w http.ResponseWriter, r *http.Request, p genapi.ReportNamesListObjectReportNameLinksParams) {
	if _, ok := s.authenticated(w, r); !ok {
		return
	}
	items, err := s.rnSvc().ListObjectReportNameLinks(derefStr(p.InspectionObjectCode),
		derefStr(p.ReportNameCode))
	if err != nil {
		familyerrors.Write(w, err)
		return
	}
	render.WriteJSON(w, http.StatusOK, map[string]any{
		"items": items, "page": 1, "pageSize": len(items), "total": len(items)})
}

func (s *Server) ReportNamesLinkReportNameStandard(w http.ResponseWriter, r *http.Request) {
	var body linkReportNameStandardBody
	decodeBodyInto(r, &body)
	if _, ok := s.authenticated(w, r); !ok {
		return
	}
	if err := s.rnSvc().LinkReportNameStandard(body.ReportNameCode,
		body.InspectionStandardCode, body.Role, body.Remark); err != nil {
		familyerrors.Write(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) ReportNamesUnlinkReportNameStandard(w http.ResponseWriter, r *http.Request) {
	var body linkReportNameStandardBody
	decodeBodyInto(r, &body)
	if _, ok := s.authenticated(w, r); !ok {
		return
	}
	if err := s.rnSvc().UnlinkReportNameStandard(body.ReportNameCode,
		body.InspectionStandardCode, body.Role); err != nil {
		familyerrors.Write(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) ReportNamesListReportNameStandardLinks(w http.ResponseWriter, r *http.Request, p genapi.ReportNamesListReportNameStandardLinksParams) {
	if _, ok := s.authenticated(w, r); !ok {
		return
	}
	items, err := s.rnSvc().ListReportNameStandardLinks(derefStr(p.ReportNameCode),
		enumStr(p.Role))
	if err != nil {
		familyerrors.Write(w, err)
		return
	}
	render.WriteJSON(w, http.StatusOK, map[string]any{
		"items": items, "page": 1, "pageSize": len(items), "total": len(items)})
}

func (s *Server) ReportNamesLinkReportNameParameter(w http.ResponseWriter, r *http.Request) {
	var body linkReportNameParameterBody
	decodeBodyInto(r, &body)
	if _, ok := s.authenticated(w, r); !ok {
		return
	}
	if err := s.rnSvc().LinkReportNameParameter(body.ReportNameCode,
		body.InspectionParameterCode, body.Remark); err != nil {
		familyerrors.Write(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) ReportNamesUnlinkReportNameParameter(w http.ResponseWriter, r *http.Request) {
	var body linkReportNameParameterBody
	decodeBodyInto(r, &body)
	if _, ok := s.authenticated(w, r); !ok {
		return
	}
	if err := s.rnSvc().UnlinkReportNameParameter(body.ReportNameCode,
		body.InspectionParameterCode); err != nil {
		familyerrors.Write(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) ReportNamesListReportNameParameterLinks(w http.ResponseWriter, r *http.Request, p genapi.ReportNamesListReportNameParameterLinksParams) {
	if _, ok := s.authenticated(w, r); !ok {
		return
	}
	items, err := s.rnSvc().ListReportNameParameterLinks(derefStr(p.ReportNameCode),
		derefStr(p.InspectionParameterCode))
	if err != nil {
		familyerrors.Write(w, err)
		return
	}
	render.WriteJSON(w, http.StatusOK, map[string]any{
		"items": items, "page": 1, "pageSize": len(items), "total": len(items)})
}
