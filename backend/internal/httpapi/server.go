package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"crm-tasks/internal/form"
	"crm-tasks/internal/service"
	"crm-tasks/internal/store"
)

type Server struct {
	Store   *store.Store
	Service *service.Service
	CRMURL  string
	Kafka   KafkaStatus
}

// KafkaStatus — состояние консьюмера для /api/health и /api/meta.
type KafkaStatus interface {
	Status() map[string]any
}

func (s *Server) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID, middleware.RealIP, middleware.Recoverer, requestLogger)

	r.Route("/api", func(r chi.Router) {
		r.Get("/health", s.health)
		r.Get("/meta", s.meta)

		r.Get("/task-types", s.listTypes)
		r.Post("/task-types", s.createType)
		r.Get("/task-types/{id}", s.getType)
		r.Put("/task-types/{id}", s.updateType)
		r.Delete("/task-types/{id}", s.deleteType)
		r.Put("/task-types/{id}/form", s.saveForm)

		r.Get("/tasks", s.listTasks)
		r.Get("/tasks/stats", s.taskStats)
		r.Post("/tasks", s.createTask)
		r.Post("/tasks/bulk-transfer", s.bulkTransfer)
		r.Get("/tasks/{id}", s.getTask)
		r.Put("/tasks/{id}", s.updateTask)
		r.Post("/tasks/{id}/status", s.setStatus)
		r.Post("/tasks/{id}/transfer", s.transferTask)
		r.Get("/tasks/{id}/events", s.listEvents)
		r.Get("/tasks/{id}/comments", s.listComments)
		r.Post("/tasks/{id}/comments", s.addComment)
		r.Get("/tasks/{id}/contractor-tasks", s.contractorTasks)
		r.Get("/tasks/{id}/client-activity", s.clientActivity)
		r.Patch("/comments/{id}", s.updateComment)
		r.Delete("/comments/{id}", s.deleteComment)

		r.Get("/refs/users", s.refUsers)
		r.Get("/refs/contractors", s.refContractors)
	})
	return r
}

func requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		start := time.Now()
		next.ServeHTTP(ww, r)
		slog.Info("http", "method", r.Method, "path", r.URL.Path, "status", ww.Status(), "dur", time.Since(start).String())
	})
}

// ---- helpers ----

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, err error) {
	var verrs form.Errors
	switch {
	case errors.As(err, &verrs):
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"error": "Ошибка валидации", "fields": verrs})
	case errors.Is(err, store.ErrNotFound):
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "Не найдено"})
	case store.IsUniqueViolation(err):
		writeJSON(w, http.StatusConflict, map[string]any{"error": "Запись с таким кодом уже существует"})
	default:
		slog.Error("request failed", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "Внутренняя ошибка сервера"})
	}
}

func badRequest(w http.ResponseWriter, msg string) {
	writeJSON(w, http.StatusBadRequest, map[string]any{"error": msg})
}

func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<20))
	if err := dec.Decode(v); err != nil {
		badRequest(w, "Некорректный JSON: "+err.Error())
		return false
	}
	return true
}

func intParam(r *http.Request, name string) (int, bool) {
	v, err := strconv.Atoi(chi.URLParam(r, name))
	return v, err == nil && v > 0
}

func queryInt(r *http.Request, name string) int {
	v, _ := strconv.Atoi(r.URL.Query().Get(name))
	return v
}

func queryList(r *http.Request, name string) []string {
	var res []string
	for _, v := range r.URL.Query()[name] {
		for _, p := range strings.Split(v, ",") {
			if p = strings.TrimSpace(p); p != "" {
				res = append(res, p)
			}
		}
	}
	return res
}

func queryTime(r *http.Request, name string) *time.Time {
	v := r.URL.Query().Get(name)
	if v == "" {
		return nil
	}
	if t, err := time.Parse(time.RFC3339, v); err == nil {
		return &t
	}
	if t, err := time.ParseInLocation("2006-01-02", v, time.Local); err == nil {
		return &t
	}
	return nil
}

// ---- system ----

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	dbOK := s.Store.Pool.Ping(r.Context()) == nil
	status := http.StatusOK
	if !dbOK {
		status = http.StatusServiceUnavailable
	}
	writeJSON(w, status, map[string]any{"db": dbOK, "kafka": s.Kafka.Status()})
}

func (s *Server) meta(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"field_types":   form.FieldTypes,
		"system_fields": form.SystemFields,
		"block_kinds":   form.BlockKinds,
		"statuses": []map[string]any{
			{"id": service.StatusNew, "name": "Новая"},
			{"id": service.StatusInProgress, "name": "В работе"},
			{"id": service.StatusDone, "name": "Выполнена"},
		},
		"priorities": []map[string]any{{"id": 1, "name": "Низкий"}, {"id": 2, "name": "Средний"}, {"id": 3, "name": "Высокий"}},
		"crm_url":    s.CRMURL,
		"kafka":      s.Kafka.Status(),
	})
}
