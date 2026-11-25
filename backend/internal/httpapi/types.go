package httpapi

import (
	"net/http"
	"regexp"
	"strings"

	"crm-tasks/internal/form"
	"crm-tasks/internal/store"
)

type typeInput struct {
	Code            string         `json:"code"`
	Name            string         `json:"name"`
	Description     string         `json:"description"`
	Color           string         `json:"color"`
	IsActive        *bool          `json:"is_active"`
	DefaultPriority int            `json:"default_priority"`
	SolveMinutes    *int           `json:"solve_minutes"`
	LegacyEntityID  *int           `json:"legacy_entity_id"`
	Results         []store.Result `json:"results"`
	Form            *form.Schema   `json:"form"`
}

var typeCodeRe = regexp.MustCompile(`^[a-z][a-z0-9_]{1,99}$`)

func (in *typeInput) validate() form.Errors {
	errs := form.Errors{}
	in.Code = strings.TrimSpace(in.Code)
	in.Name = strings.TrimSpace(in.Name)
	if !typeCodeRe.MatchString(in.Code) {
		errs["code"] = "код: латинские буквы в нижнем регистре, цифры и _, от 2 символов"
	}
	if in.Name == "" {
		errs["name"] = "не задано название"
	}
	if in.DefaultPriority == 0 {
		in.DefaultPriority = 2
	}
	if in.DefaultPriority < 1 || in.DefaultPriority > 3 {
		errs["default_priority"] = "приоритет от 1 до 3"
	}
	if in.SolveMinutes != nil && *in.SolveMinutes <= 0 {
		in.SolveMinutes = nil
	}
	if in.Color == "" {
		in.Color = "#409eff"
	}
	for i := range in.Results {
		in.Results[i].Name = strings.TrimSpace(in.Results[i].Name)
		if in.Results[i].Name == "" {
			errs["results"] = "у каждого результата должно быть название"
		}
		if in.Results[i].Color == "" {
			in.Results[i].Color = "primary"
		}
	}
	return errs
}

func (s *Server) listTypes(w http.ResponseWriter, r *http.Request) {
	types, err := s.Store.ListTypes(r.Context(), r.URL.Query().Get("active") == "1")
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, types)
}

func (s *Server) getType(w http.ResponseWriter, r *http.Request) {
	id, ok := intParam(r, "id")
	if !ok {
		badRequest(w, "некорректный id")
		return
	}
	tt, err := s.Store.GetType(r.Context(), s.Store.Pool, id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, tt)
}

func (s *Server) createType(w http.ResponseWriter, r *http.Request) {
	var in typeInput
	if !decode(w, r, &in) {
		return
	}
	if errs := in.validate(); len(errs) > 0 {
		writeError(w, errs)
		return
	}
	tt := &store.TaskType{
		Code: in.Code, Name: in.Name, Description: in.Description, Color: in.Color, IsActive: true,
		DefaultPriority: in.DefaultPriority, SolveMinutes: in.SolveMinutes, LegacyEntityID: in.LegacyEntityID,
		Results: in.Results, Form: defaultForm(),
	}
	if in.Form != nil {
		in.Form.Normalize()
		if err := in.Form.Validate(); err != nil {
			writeError(w, err)
			return
		}
		tt.Form = *in.Form
	}
	if err := s.Store.SaveType(r.Context(), tt); err != nil {
		writeError(w, err)
		return
	}
	res, err := s.Store.GetType(r.Context(), s.Store.Pool, tt.ID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, res)
}

func (s *Server) updateType(w http.ResponseWriter, r *http.Request) {
	id, ok := intParam(r, "id")
	if !ok {
		badRequest(w, "некорректный id")
		return
	}
	var in typeInput
	if !decode(w, r, &in) {
		return
	}
	if errs := in.validate(); len(errs) > 0 {
		writeError(w, errs)
		return
	}
	active := true
	if in.IsActive != nil {
		active = *in.IsActive
	}
	tt := &store.TaskType{
		ID: id, Code: in.Code, Name: in.Name, Description: in.Description, Color: in.Color, IsActive: active,
		DefaultPriority: in.DefaultPriority, SolveMinutes: in.SolveMinutes, LegacyEntityID: in.LegacyEntityID, Results: in.Results,
	}
	if err := s.Store.SaveType(r.Context(), tt); err != nil {
		writeError(w, err)
		return
	}
	res, err := s.Store.GetType(r.Context(), s.Store.Pool, id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) deleteType(w http.ResponseWriter, r *http.Request) {
	id, ok := intParam(r, "id")
	if !ok {
		badRequest(w, "некорректный id")
		return
	}
	if err := s.Store.DeleteType(r.Context(), id); err != nil {
		if err == store.ErrNotFound {
			writeJSON(w, http.StatusConflict, map[string]any{"error": "Тип не найден или по нему уже есть задачи — отключите его вместо удаления"})
			return
		}
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) saveForm(w http.ResponseWriter, r *http.Request) {
	id, ok := intParam(r, "id")
	if !ok {
		badRequest(w, "некорректный id")
		return
	}
	var schema form.Schema
	if !decode(w, r, &schema) {
		return
	}
	schema.Normalize()
	if err := schema.Validate(); err != nil {
		writeError(w, err)
		return
	}
	if _, err := s.Store.SaveForm(r.Context(), id, schema); err != nil {
		writeError(w, err)
		return
	}
	tt, err := s.Store.GetType(r.Context(), s.Store.Pool, id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, tt)
}

// defaultForm — стартовая форма нового типа: основная информация + комментарии.
func defaultForm() form.Schema {
	s := form.Schema{Blocks: []form.Block{
		{ID: "main", Title: "Основная информация", Kind: form.BlockFields, Span: 24, Fields: []form.Field{
			{ID: "f_title", Type: form.TypeSystem, System: form.SysTitle, Label: "Цель задачи", Required: true, Span: 12},
			{ID: "f_result", Type: form.TypeSystem, System: form.SysResult, Label: "Результат", Span: 12},
			{ID: "f_executor", Type: form.TypeSystem, System: form.SysExecutor, Label: "Исполнитель", Span: 12},
			{ID: "f_plan", Type: form.TypeSystem, System: form.SysSolvePlanAt, Label: "Планируемая дата завершения", Span: 12},
			{ID: "f_contractor", Type: form.TypeSystem, System: form.SysContractor, Label: "Контрагент", Span: 12},
			{ID: "f_contact", Type: form.TypeSystem, System: form.SysContact, Label: "Контактное лицо", Span: 12},
			{ID: "f_description", Type: form.TypeSystem, System: form.SysDescription, Label: "Описание задачи", Span: 24},
		}},
		{ID: "comments", Title: "Комментарии", Kind: form.BlockComments, Span: 12, Fields: []form.Field{}},
		{ID: "history", Title: "История изменений", Kind: form.BlockHistory, Span: 12, Fields: []form.Field{}},
	}}
	s.Normalize()
	return s
}
