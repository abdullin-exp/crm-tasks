package httpapi

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"crm-tasks/internal/service"
	"crm-tasks/internal/store"
)

func taskID(w http.ResponseWriter, r *http.Request) (string, bool) {
	id := chi.URLParam(r, "id")
	if _, err := uuid.Parse(id); err != nil {
		badRequest(w, "некорректный id задачи")
		return "", false
	}
	return id, true
}

func (s *Server) listTasks(w http.ResponseWriter, r *http.Request) {
	f, ok := parseFilter(w, r)
	if !ok {
		return
	}
	items, total, err := s.Store.ListTasks(r.Context(), f)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": total})
}

func (s *Server) taskStats(w http.ResponseWriter, r *http.Request) {
	f, ok := parseFilter(w, r)
	if !ok {
		return
	}
	st, err := s.Store.Stats(r.Context(), f)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func parseFilter(w http.ResponseWriter, r *http.Request) (store.TaskFilter, bool) {
	q := r.URL.Query()
	f := store.TaskFilter{
		TypeID:       queryInt(r, "type_id"),
		ResultID:     queryInt(r, "result_id"),
		ExecutorID:   q.Get("executor_user_id"),
		ContractorID: q.Get("contractor_id"),
		ContactID:    q.Get("contact_contractor_id"),
		ObserverID:   q.Get("observer_user_id"),
		Source:       q.Get("source"),
		Query:        q.Get("q"),
		Overdue:      q.Get("overdue") == "1",
		CreatedFrom:  queryTime(r, "created_from"),
		CreatedTo:    queryTime(r, "created_to"),
		PlanFrom:     queryTime(r, "plan_from"),
		PlanTo:       queryTime(r, "plan_to"),
		Sort:         q.Get("sort"),
		Desc:         q.Get("order") == "desc",
		Page:         queryInt(r, "page"),
		PerPage:      queryInt(r, "per_page"),
	}
	for _, v := range queryList(r, "status_id") {
		if n, err := strconv.Atoi(v); err == nil {
			f.StatusIDs = append(f.StatusIDs, n)
		}
	}
	for _, v := range []string{f.ExecutorID, f.ContractorID, f.ContactID, f.ObserverID} {
		if v != "" {
			if _, err := uuid.Parse(v); err != nil {
				badRequest(w, "некорректный uuid в фильтре")
				return f, false
			}
		}
	}
	return f, true
}

func (s *Server) getTask(w http.ResponseWriter, r *http.Request) {
	id, ok := taskID(w, r)
	if !ok {
		return
	}
	t, err := s.Store.GetTask(r.Context(), s.Store.Pool, id)
	if err != nil {
		writeError(w, err)
		return
	}
	tt, err := s.Store.GetType(r.Context(), s.Store.Pool, t.TaskTypeID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"task": t, "type": tt})
}

func (s *Server) createTask(w http.ResponseWriter, r *http.Request) {
	var in service.TaskInput
	if !decode(w, r, &in) {
		return
	}
	// Задачи из UI — manual; внешние системы передают external_id и получают идемпотентность.
	source := service.SourceManual
	if in.ExternalID != nil && strings.TrimSpace(*in.ExternalID) != "" {
		source = service.SourceAPI
	}
	t, err := s.Service.Create(r.Context(), in, source)
	if errors.Is(err, service.ErrDuplicate) {
		writeJSON(w, http.StatusOK, t)
		return
	}
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, t)
}

func (s *Server) updateTask(w http.ResponseWriter, r *http.Request) {
	id, ok := taskID(w, r)
	if !ok {
		return
	}
	var in service.TaskInput
	if !decode(w, r, &in) {
		return
	}
	t, err := s.Service.Update(r.Context(), id, in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func (s *Server) setStatus(w http.ResponseWriter, r *http.Request) {
	id, ok := taskID(w, r)
	if !ok {
		return
	}
	var in struct {
		StatusID int `json:"status_id"`
	}
	if !decode(w, r, &in) {
		return
	}
	t, err := s.Service.SetStatus(r.Context(), id, in.StatusID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func (s *Server) transferTask(w http.ResponseWriter, r *http.Request) {
	id, ok := taskID(w, r)
	if !ok {
		return
	}
	var in service.TransferInput
	if !decode(w, r, &in) {
		return
	}
	t, err := s.Service.Transfer(r.Context(), id, in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func (s *Server) bulkTransfer(w http.ResponseWriter, r *http.Request) {
	var in struct {
		service.TransferInput
		TaskIDs []string `json:"task_ids"`
	}
	if !decode(w, r, &in) {
		return
	}
	res, err := s.Service.BulkTransfer(r.Context(), in.TaskIDs, in.TransferInput)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) listEvents(w http.ResponseWriter, r *http.Request) {
	id, ok := taskID(w, r)
	if !ok {
		return
	}
	ev, err := s.Store.ListEvents(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, ev)
}

func (s *Server) contractorTasks(w http.ResponseWriter, r *http.Request) {
	id, ok := taskID(w, r)
	if !ok {
		return
	}
	t, err := s.Store.GetTask(r.Context(), s.Store.Pool, id)
	if err != nil {
		writeError(w, err)
		return
	}
	res, err := s.Store.ContractorTasks(r.Context(), t.ContractorID, t.ContactContractorID, id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) clientActivity(w http.ResponseWriter, r *http.Request) {
	id, ok := taskID(w, r)
	if !ok {
		return
	}
	t, err := s.Store.GetTask(r.Context(), s.Store.Pool, id)
	if err != nil {
		writeError(w, err)
		return
	}
	res, err := s.Store.ClientActivity(r.Context(), t.ContractorID, t.ContactContractorID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// ---- комментарии ----

func (s *Server) listComments(w http.ResponseWriter, r *http.Request) {
	id, ok := taskID(w, r)
	if !ok {
		return
	}
	res, err := s.Store.ListComments(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) addComment(w http.ResponseWriter, r *http.Request) {
	id, ok := taskID(w, r)
	if !ok {
		return
	}
	var in struct {
		Text         string  `json:"text"`
		AuthorUserID *string `json:"author_user_id"`
	}
	if !decode(w, r, &in) {
		return
	}
	if strings.TrimSpace(in.Text) == "" {
		badRequest(w, "пустой комментарий")
		return
	}
	if in.AuthorUserID != nil && *in.AuthorUserID == "" {
		in.AuthorUserID = nil
	}
	if in.AuthorUserID != nil {
		if _, err := uuid.Parse(*in.AuthorUserID); err != nil {
			badRequest(w, "некорректный author_user_id")
			return
		}
		if ok, err := s.Store.UserExists(r.Context(), *in.AuthorUserID); err != nil || !ok {
			in.AuthorUserID = nil // неизвестный автор — сохраняем без автора
		}
	}
	if _, err := s.Store.GetTask(r.Context(), s.Store.Pool, id); err != nil {
		writeError(w, err)
		return
	}
	c, err := s.Store.AddComment(r.Context(), id, strings.TrimSpace(in.Text), in.AuthorUserID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, c)
}

func (s *Server) updateComment(w http.ResponseWriter, r *http.Request) {
	id, ok := taskID(w, r)
	if !ok {
		return
	}
	var in struct {
		Text       *string `json:"text"`
		IsFavorite *bool   `json:"is_favorite"`
	}
	if !decode(w, r, &in) {
		return
	}
	if in.Text != nil && strings.TrimSpace(*in.Text) == "" {
		badRequest(w, "пустой комментарий")
		return
	}
	c, err := s.Store.UpdateComment(r.Context(), id, in.Text, in.IsFavorite)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, c)
}

func (s *Server) deleteComment(w http.ResponseWriter, r *http.Request) {
	id, ok := taskID(w, r)
	if !ok {
		return
	}
	if err := s.Store.DeleteComment(r.Context(), id); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- справочники CRM ----

func (s *Server) refUsers(w http.ResponseWriter, r *http.Request) {
	res, err := s.Store.SearchUsers(r.Context(), strings.TrimSpace(r.URL.Query().Get("q")), queryList(r, "ids"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) refContractors(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	res, err := s.Store.SearchContractors(r.Context(), strings.TrimSpace(q.Get("q")), q.Get("kind"), queryList(r, "ids"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}
