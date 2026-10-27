package api

// inspection_dictionary_impl.go — 检测能力字典 handler（REQ-2026-004 Task 7）。
//
// 28 端点：四实体 CRUD（16）+ 四组 junction link/unlink/list（12），M06.F01-F04。
// 薄层装配：守卫 → service → 信封/状态码。实体 list 信封 page 缺省 1、pageSize
// 缺省 = items.size（wrapDict 语义）；junction list 短信封 page 恒 1、pageSize=total。
// link/unlink 恒 204；unlink 缺失幂等 no-op。

import (
	"encoding/json"
	"net/http"

	genapi "lab-management-system-go/internal/gen/api"
	familyerrors "lab-management-system-go/internal/impl/errors"
	"lab-management-system-go/internal/impl/render"
	"lab-management-system-go/internal/impl/service"
)

// dictSvc 域服务（每请求构造，DB 注入）。
func (s *Server) dictSvc() *service.InspectionDictionaryService {
	return service.NewInspectionDictionaryService(s.DB)
}

// dictList 实体 list 薄壳：守卫 → service → wrapDict 信封。
func (s *Server) dictList(w http.ResponseWriter, r *http.Request,
	list func() ([]map[string]any, error), page, pageSize *int32) {
	if _, ok := s.authenticated(w, r); !ok {
		return
	}
	items, err := list()
	if err != nil {
		familyerrors.Write(w, err)
		return
	}
	p := 1
	if page != nil {
		p = int(*page)
	}
	ps := len(items)
	if pageSize != nil {
		ps = int(*pageSize)
	}
	render.WriteJSON(w, http.StatusOK, map[string]any{
		"items": items, "page": p, "pageSize": ps, "total": len(items)})
}

// junctionList 短信封（page 恒 1，镜像 springboot 200Response 硬编码）。
func (s *Server) junctionList(w http.ResponseWriter, r *http.Request,
	list func() ([]map[string]any, error)) {
	if _, ok := s.authenticated(w, r); !ok {
		return
	}
	items, err := list()
	if err != nil {
		familyerrors.Write(w, err)
		return
	}
	render.WriteJSON(w, http.StatusOK, map[string]any{
		"items": items, "page": 1, "pageSize": len(items), "total": len(items)})
}

// dictMut 有返回值变更薄壳（create/update）。
func (s *Server) dictMut(w http.ResponseWriter, r *http.Request,
	mut func() (map[string]any, error)) {
	if _, ok := s.authenticated(w, r); !ok {
		return
	}
	dto, err := mut()
	if err != nil {
		familyerrors.Write(w, err)
		return
	}
	render.WriteJSON(w, http.StatusOK, dto)
}

// dictDelete 删除薄壳（204 无体；未知 404 由 service 上抛）。
func (s *Server) dictDelete(w http.ResponseWriter, r *http.Request, del func() error) {
	if _, ok := s.authenticated(w, r); !ok {
		return
	}
	if err := del(); err != nil {
		familyerrors.Write(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// dictLink junction link 薄壳（恒 204）。
func (s *Server) dictLink(w http.ResponseWriter, r *http.Request, link func() error) {
	if _, ok := s.authenticated(w, r); !ok {
		return
	}
	if err := link(); err != nil {
		familyerrors.Write(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// decodeBodyInto 解析请求体（解析失败 → 零值结构，rails rescue {} 语义）。
func decodeBodyInto(r *http.Request, v any) {
	_ = json.NewDecoder(r.Body).Decode(v)
}

// ---- M06.F01 specialties ----

func (s *Server) InspectionDictionaryListSpecialties(w http.ResponseWriter, r *http.Request, p genapi.InspectionDictionaryListSpecialtiesParams) {
	s.dictList(w, r, func() ([]map[string]any, error) {
		return s.dictSvc().ListSpecialties(derefStr(p.Keyword))
	}, p.Page, p.PageSize)
}

func (s *Server) InspectionDictionaryCreateSpecialty(w http.ResponseWriter, r *http.Request) {
	var body genapi.CreateInspectionSpecialtyRequest
	decodeBodyInto(r, &body)
	s.dictMut(w, r, func() (map[string]any, error) {
		return s.dictSvc().CreateSpecialty(service.CreateSpecialtyInput{
			Code: body.Code, OfficialNo: body.OfficialNo, Name: body.Name,
			IsOfficial: body.IsOfficial, Enabled: body.Enabled, SortOrder: body.SortOrder,
		})
	})
}

func (s *Server) InspectionDictionaryUpdateSpecialty(w http.ResponseWriter, r *http.Request, code string) {
	var body genapi.UpdateInspectionSpecialtyRequest
	decodeBodyInto(r, &body)
	s.dictMut(w, r, func() (map[string]any, error) {
		return s.dictSvc().UpdateSpecialty(code, body.OfficialNo, body.Name,
			body.IsOfficial, body.Enabled, body.SortOrder)
	})
}

func (s *Server) InspectionDictionaryDeleteSpecialty(w http.ResponseWriter, r *http.Request, code string) {
	s.dictDelete(w, r, func() error { return s.dictSvc().DeleteSpecialty(code) })
}

// ---- M06.F02 objects ----

func (s *Server) InspectionDictionaryListObjects(w http.ResponseWriter, r *http.Request, p genapi.InspectionDictionaryListObjectsParams) {
	s.dictList(w, r, func() ([]map[string]any, error) {
		return s.dictSvc().ListObjects(derefStr(p.Keyword), derefStr(p.InspectionSpecialtyCode))
	}, p.Page, p.PageSize)
}

func (s *Server) InspectionDictionaryCreateObject(w http.ResponseWriter, r *http.Request) {
	var body genapi.CreateInspectionObjectRequest
	decodeBodyInto(r, &body)
	s.dictMut(w, r, func() (map[string]any, error) {
		return s.dictSvc().CreateObject(service.CreateObjectInput{
			Code: body.Code, InspectionSpecialtyCode: body.InspectionSpecialtyCode,
			SourceProjectNo: body.SourceProjectNo, SourceProjectName: body.SourceProjectName,
			Name:                       body.Name,
			IsOptionalForQualification: body.IsOptionalForQualification,
			IsOfficial:                 body.IsOfficial, Enabled: body.Enabled,
			SortOrder: body.SortOrder,
		})
	})
}

func (s *Server) InspectionDictionaryUpdateObject(w http.ResponseWriter, r *http.Request, code string) {
	var body genapi.UpdateInspectionObjectRequest
	decodeBodyInto(r, &body)
	s.dictMut(w, r, func() (map[string]any, error) {
		return s.dictSvc().UpdateObject(code, body.InspectionSpecialtyCode,
			body.SourceProjectNo, body.SourceProjectName, body.Name,
			body.IsOptionalForQualification, body.IsOfficial, body.Enabled, body.SortOrder)
	})
}

func (s *Server) InspectionDictionaryDeleteObject(w http.ResponseWriter, r *http.Request, code string) {
	s.dictDelete(w, r, func() error { return s.dictSvc().DeleteObject(code) })
}

// ---- M06.F03 parameters ----

func (s *Server) InspectionDictionaryListParameters(w http.ResponseWriter, r *http.Request, p genapi.InspectionDictionaryListParametersParams) {
	s.dictList(w, r, func() ([]map[string]any, error) {
		return s.dictSvc().ListParameters(derefStr(p.Keyword), enumStr(p.SourceType))
	}, p.Page, p.PageSize)
}

func (s *Server) InspectionDictionaryCreateParameter(w http.ResponseWriter, r *http.Request) {
	var body genapi.CreateInspectionParameterRequest
	decodeBodyInto(r, &body)
	s.dictMut(w, r, func() (map[string]any, error) {
		in := service.CreateParameterInput{
			Code: body.Code, Name: body.Name, RawName: body.RawName,
			CanonicalName: body.CanonicalName, MethodText: body.MethodText,
			Unit: body.Unit, SortOrder: body.SortOrder,
		}
		if body.Aliases != nil {
			in.Aliases = *body.Aliases
		}
		if body.SourceType != nil {
			in.SourceType = string(*body.SourceType)
		}
		return s.dictSvc().CreateParameter(in)
	})
}

func (s *Server) InspectionDictionaryUpdateParameter(w http.ResponseWriter, r *http.Request, code string) {
	var body genapi.UpdateInspectionParameterRequest
	decodeBodyInto(r, &body)
	s.dictMut(w, r, func() (map[string]any, error) {
		var aliases []string
		if body.Aliases != nil {
			aliases = *body.Aliases
		}
		var sourceType *string
		if body.SourceType != nil {
			st := string(*body.SourceType)
			sourceType = &st
		}
		return s.dictSvc().UpdateParameter(code, body.Name, body.RawName,
			body.CanonicalName, body.MethodText, aliases, body.Unit,
			sourceType, body.SortOrder)
	})
}

func (s *Server) InspectionDictionaryDeleteParameter(w http.ResponseWriter, r *http.Request, code string) {
	s.dictDelete(w, r, func() error { return s.dictSvc().DeleteParameter(code) })
}

// ---- M06.F04 standards ----

func (s *Server) InspectionDictionaryListStandards(w http.ResponseWriter, r *http.Request, p genapi.InspectionDictionaryListStandardsParams) {
	s.dictList(w, r, func() ([]map[string]any, error) {
		return s.dictSvc().ListStandards(derefStr(p.Keyword), enumStr(p.Status))
	}, p.Page, p.PageSize)
}

func (s *Server) InspectionDictionaryCreateStandard(w http.ResponseWriter, r *http.Request) {
	var body genapi.CreateInspectionStandardRequest
	decodeBodyInto(r, &body)
	s.dictMut(w, r, func() (map[string]any, error) {
		in := service.CreateStandardInput{
			Code: body.Code, Name: body.Name, SourceDocumentId: body.SourceDocumentId,
			SourceHash: body.SourceHash, SortOrder: body.SortOrder,
		}
		if body.Version != nil {
			in.Version = *body.Version
		}
		if body.Status != nil {
			in.Status = string(*body.Status)
		}
		return s.dictSvc().CreateStandard(in)
	})
}

func (s *Server) InspectionDictionaryUpdateStandard(w http.ResponseWriter, r *http.Request, code string) {
	var body genapi.UpdateInspectionStandardRequest
	decodeBodyInto(r, &body)
	s.dictMut(w, r, func() (map[string]any, error) {
		var status *string
		if body.Status != nil {
			st := string(*body.Status)
			status = &st
		}
		return s.dictSvc().UpdateStandard(code, body.Name, status,
			body.Version, body.SourceDocumentId, body.SourceHash, body.SortOrder)
	})
}

func (s *Server) InspectionDictionaryDeleteStandard(w http.ResponseWriter, r *http.Request, code string) {
	s.dictDelete(w, r, func() error { return s.dictSvc().DeleteStandard(code) })
}

// enumStr 泛化枚举指针解引（nil → 空串；sourceType/status/role 复用）。
func enumStr[T ~string](v *T) string {
	if v == nil {
		return ""
	}
	return string(*v)
}

// ---- junction link/unlink/list（12 端点） ----

// linkSpecialtyObjectBody POST/DELETE /api/inspection/links/specialty-object 载荷。
type linkSpecialtyObjectBody struct {
	InspectionSpecialtyCode string  `json:"inspectionSpecialtyCode"`
	InspectionObjectCode    string  `json:"inspectionObjectCode"`
	Remark                  *string `json:"remark"`
}

// linkObjectParameterBody object-parameter 载荷。
type linkObjectParameterBody struct {
	InspectionObjectCode    string  `json:"inspectionObjectCode"`
	InspectionParameterCode string  `json:"inspectionParameterCode"`
	QualificationLevel      string  `json:"qualificationLevel"`
	SourcePage              *int32  `json:"sourcePage"`
	Remark                  *string `json:"remark"`
}

// linkObjectStandardBody object-standard 载荷（role 必填）。
type linkObjectStandardBody struct {
	InspectionObjectCode   string  `json:"inspectionObjectCode"`
	InspectionStandardCode string  `json:"inspectionStandardCode"`
	Role                   string  `json:"role"`
	Remark                 *string `json:"remark"`
}

// linkStandardParameterBody standard-parameter 载荷（无载荷双键）。
type linkStandardParameterBody struct {
	InspectionStandardCode  string `json:"inspectionStandardCode"`
	InspectionParameterCode string `json:"inspectionParameterCode"`
}

func (s *Server) InspectionDictionaryLinkSpecialtyObject(w http.ResponseWriter, r *http.Request) {
	var body linkSpecialtyObjectBody
	decodeBodyInto(r, &body)
	s.dictLink(w, r, func() error {
		return s.dictSvc().LinkSpecialtyObject(body.InspectionSpecialtyCode,
			body.InspectionObjectCode, body.Remark)
	})
}

func (s *Server) InspectionDictionaryUnlinkSpecialtyObject(w http.ResponseWriter, r *http.Request) {
	var body linkSpecialtyObjectBody
	decodeBodyInto(r, &body)
	s.dictLink(w, r, func() error {
		return s.dictSvc().UnlinkSpecialtyObject(body.InspectionSpecialtyCode,
			body.InspectionObjectCode)
	})
}

func (s *Server) InspectionDictionaryListSpecialtyObjectLinks(w http.ResponseWriter, r *http.Request, p genapi.InspectionDictionaryListSpecialtyObjectLinksParams) {
	s.junctionList(w, r, func() ([]map[string]any, error) {
		return s.dictSvc().ListSpecialtyObjectLinks(derefStr(p.InspectionSpecialtyCode))
	})
}

func (s *Server) InspectionDictionaryLinkObjectParameter(w http.ResponseWriter, r *http.Request) {
	var body linkObjectParameterBody
	decodeBodyInto(r, &body)
	s.dictLink(w, r, func() error {
		return s.dictSvc().LinkObjectParameter(body.InspectionObjectCode,
			body.InspectionParameterCode, body.QualificationLevel, body.SourcePage, body.Remark)
	})
}

func (s *Server) InspectionDictionaryUnlinkObjectParameter(w http.ResponseWriter, r *http.Request) {
	var body linkObjectParameterBody
	decodeBodyInto(r, &body)
	s.dictLink(w, r, func() error {
		return s.dictSvc().UnlinkObjectParameter(body.InspectionObjectCode,
			body.InspectionParameterCode)
	})
}

func (s *Server) InspectionDictionaryListObjectParameterLinks(w http.ResponseWriter, r *http.Request, p genapi.InspectionDictionaryListObjectParameterLinksParams) {
	s.junctionList(w, r, func() ([]map[string]any, error) {
		return s.dictSvc().ListObjectParameterLinks(derefStr(p.InspectionObjectCode),
			derefStr(p.InspectionParameterCode))
	})
}

func (s *Server) InspectionDictionaryLinkObjectStandard(w http.ResponseWriter, r *http.Request) {
	var body linkObjectStandardBody
	decodeBodyInto(r, &body)
	s.dictLink(w, r, func() error {
		return s.dictSvc().LinkObjectStandard(body.InspectionObjectCode,
			body.InspectionStandardCode, body.Role, body.Remark)
	})
}

func (s *Server) InspectionDictionaryUnlinkObjectStandard(w http.ResponseWriter, r *http.Request) {
	var body linkObjectStandardBody
	decodeBodyInto(r, &body)
	s.dictLink(w, r, func() error {
		return s.dictSvc().UnlinkObjectStandard(body.InspectionObjectCode,
			body.InspectionStandardCode, body.Role)
	})
}

func (s *Server) InspectionDictionaryListObjectStandardLinks(w http.ResponseWriter, r *http.Request, p genapi.InspectionDictionaryListObjectStandardLinksParams) {
	s.junctionList(w, r, func() ([]map[string]any, error) {
		return s.dictSvc().ListObjectStandardLinks(derefStr(p.InspectionObjectCode),
			enumStr(p.Role))
	})
}

func (s *Server) InspectionDictionaryLinkStandardParameter(w http.ResponseWriter, r *http.Request) {
	var body linkStandardParameterBody
	decodeBodyInto(r, &body)
	s.dictLink(w, r, func() error {
		return s.dictSvc().LinkStandardParameter(body.InspectionStandardCode,
			body.InspectionParameterCode)
	})
}

func (s *Server) InspectionDictionaryUnlinkStandardParameter(w http.ResponseWriter, r *http.Request) {
	var body linkStandardParameterBody
	decodeBodyInto(r, &body)
	s.dictLink(w, r, func() error {
		return s.dictSvc().UnlinkStandardParameter(body.InspectionStandardCode,
			body.InspectionParameterCode)
	})
}

func (s *Server) InspectionDictionaryListStandardParameterLinks(w http.ResponseWriter, r *http.Request, p genapi.InspectionDictionaryListStandardParameterLinksParams) {
	s.junctionList(w, r, func() ([]map[string]any, error) {
		return s.dictSvc().ListStandardParameterLinks(derefStr(p.InspectionStandardCode),
			derefStr(p.InspectionParameterCode))
	})
}
