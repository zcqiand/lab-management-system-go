package api

// inspection_catalog_impl.go — 目录谱系 handler（REQ-2026-004 Task 6）。
//
// 16 端点 4 字典同构（brand/model/spec/grade，M04.F06-F09）：薄层装配，
// 业务在 service.CatalogService。list 全量 + Page 信封（page 缺省 1、pageSize
// 缺省 = items.size——2026-09-16 T11 live 语义，只回显不切页）；
// create/update/delete 语义同 rails CatalogController。

import (
	"net/http"

	genapi "lab-management-system-go/internal/gen/api"
	familyerrors "lab-management-system-go/internal/impl/errors"
	"lab-management-system-go/internal/impl/render"
	"lab-management-system-go/internal/impl/service"
)

// catalogSvc 域服务（每请求构造，DB 注入）。
func (s *Server) catalogSvc() *service.CatalogService { return service.NewCatalogService(s.DB) }

// catalogList list 薄壳：守卫 → service → 信封 camel 直出。
func (s *Server) catalogList(w http.ResponseWriter, r *http.Request, kind string,
	page, pageSize *int32, objectCode, keyword *string) {
	claims, ok := s.authenticated(w, r)
	if !ok {
		return
	}
	items, err := s.catalogSvc().List(kind, claims["tenant_id"].(string),
		derefStr(objectCode), derefStr(keyword))
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
		"items": items, "page": p, "pageSize": ps, "total": len(items),
	})
}

func derefStr(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

// catalogCreate create 薄壳。
func (s *Server) catalogCreate(w http.ResponseWriter, r *http.Request, kind string) {
	claims, ok := s.authenticated(w, r)
	if !ok {
		return
	}
	var body genapi.CreateCatalogEntryRequest
	decodeBody(r, &body)
	entry, err := s.catalogSvc().Create(kind, claims["tenant_id"].(string),
		body.Code, body.Name, body.InspectionObjectCode, body.Remark, body.SortOrder)
	if err != nil {
		familyerrors.Write(w, err)
		return
	}
	render.WriteJSON(w, http.StatusOK, entry)
}

// catalogUpdate update 薄壳。
func (s *Server) catalogUpdate(w http.ResponseWriter, r *http.Request, kind, code string) {
	claims, ok := s.authenticated(w, r)
	if !ok {
		return
	}
	var body genapi.UpdateCatalogEntryRequest
	decodeBody(r, &body)
	entry, err := s.catalogSvc().Update(kind, claims["tenant_id"].(string), code,
		body.InspectionObjectCode, body.Name, body.Remark, body.SortOrder)
	if err != nil {
		familyerrors.Write(w, err)
		return
	}
	render.WriteJSON(w, http.StatusOK, entry)
}

// catalogDelete delete 薄壳（204 无体）。
func (s *Server) catalogDelete(w http.ResponseWriter, r *http.Request, kind, code string) {
	claims, ok := s.authenticated(w, r)
	if !ok {
		return
	}
	if err := s.catalogSvc().Delete(kind, claims["tenant_id"].(string), code); err != nil {
		familyerrors.Write(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- Brand (M04.F09) ----

func (s *Server) CatalogListBrands(w http.ResponseWriter, r *http.Request, p genapi.CatalogListBrandsParams) {
	s.catalogList(w, r, "brand", p.Page, p.PageSize, p.InspectionObjectCode, p.Keyword)
}

func (s *Server) CatalogCreateBrand(w http.ResponseWriter, r *http.Request) {
	s.catalogCreate(w, r, "brand")
}

func (s *Server) CatalogUpdateBrand(w http.ResponseWriter, r *http.Request, code string) {
	s.catalogUpdate(w, r, "brand", code)
}

func (s *Server) CatalogDeleteBrand(w http.ResponseWriter, r *http.Request, code string) {
	s.catalogDelete(w, r, "brand", code)
}

// ---- Model (M04.F06) ----

func (s *Server) CatalogListModels(w http.ResponseWriter, r *http.Request, p genapi.CatalogListModelsParams) {
	s.catalogList(w, r, "model", p.Page, p.PageSize, p.InspectionObjectCode, p.Keyword)
}

func (s *Server) CatalogCreateModel(w http.ResponseWriter, r *http.Request) {
	s.catalogCreate(w, r, "model")
}

func (s *Server) CatalogUpdateModel(w http.ResponseWriter, r *http.Request, code string) {
	s.catalogUpdate(w, r, "model", code)
}

func (s *Server) CatalogDeleteModel(w http.ResponseWriter, r *http.Request, code string) {
	s.catalogDelete(w, r, "model", code)
}

// ---- Spec (M04.F07) ----

func (s *Server) CatalogListSpecs(w http.ResponseWriter, r *http.Request, p genapi.CatalogListSpecsParams) {
	s.catalogList(w, r, "spec", p.Page, p.PageSize, p.InspectionObjectCode, p.Keyword)
}

func (s *Server) CatalogCreateSpec(w http.ResponseWriter, r *http.Request) {
	s.catalogCreate(w, r, "spec")
}

func (s *Server) CatalogUpdateSpec(w http.ResponseWriter, r *http.Request, code string) {
	s.catalogUpdate(w, r, "spec", code)
}

func (s *Server) CatalogDeleteSpec(w http.ResponseWriter, r *http.Request, code string) {
	s.catalogDelete(w, r, "spec", code)
}

// ---- Grade (M04.F08) ----

func (s *Server) CatalogListGrades(w http.ResponseWriter, r *http.Request, p genapi.CatalogListGradesParams) {
	s.catalogList(w, r, "grade", p.Page, p.PageSize, p.InspectionObjectCode, p.Keyword)
}

func (s *Server) CatalogCreateGrade(w http.ResponseWriter, r *http.Request) {
	s.catalogCreate(w, r, "grade")
}

func (s *Server) CatalogUpdateGrade(w http.ResponseWriter, r *http.Request, code string) {
	s.catalogUpdate(w, r, "grade", code)
}

func (s *Server) CatalogDeleteGrade(w http.ResponseWriter, r *http.Request, code string) {
	s.catalogDelete(w, r, "grade", code)
}
