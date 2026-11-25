package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type Task struct {
	ID                  string         `json:"id"`
	Number              int64          `json:"number"`
	TaskTypeID          int            `json:"task_type_id"`
	TypeName            string         `json:"type_name"`
	TypeColor           string         `json:"type_color"`
	FormVersion         int            `json:"form_version"`
	StatusID            int            `json:"status_id"`
	StatusName          string         `json:"status_name"`
	ResultID            *int           `json:"result_id"`
	ResultName          *string        `json:"result_name"`
	ResultColor         *string        `json:"result_color"`
	Priority            int            `json:"priority"`
	Title               string         `json:"title"`
	Description         string         `json:"description"`
	ExecutorUserID      *string        `json:"executor_user_id"`
	ExecutorName        *string        `json:"executor_name"`
	ContractorID        *string        `json:"contractor_id"`
	ContractorName      *string        `json:"contractor_name"`
	ContactContractorID *string        `json:"contact_contractor_id"`
	ContactName         *string        `json:"contact_contractor_name"`
	SolvePlanAt         *time.Time     `json:"solve_plan_at"`
	SolveFactAt         *time.Time     `json:"solve_fact_at"`
	Data                map[string]any `json:"data"`
	Source              string         `json:"source"`
	ExternalID          *string        `json:"external_id"`
	LegacyTaskID        *string        `json:"legacy_task_id"`
	Observers           []RefItem      `json:"observers"`
	CreatedAt           time.Time      `json:"created_at"`
	UpdatedAt           time.Time      `json:"updated_at"`
	Overdue             bool           `json:"overdue"`
}

func (s *Store) taskSelect() string {
	return s.q(`SELECT t.id, t.number, t.task_type_id, tt.name, tt.color, t.form_version,
		t.status_id, st.name, t.result_id, r.name, r.color, t.priority, t.title, t.description,
		t.executor_user_id, u.name, t.contractor_id, ` + cname("c") + `, t.contact_contractor_id, ` + cname("cc") + `,
		t.solve_plan_at, t.solve_fact_at, t.data, t.source, t.external_id, t.legacy_task_id, t.created_at, t.updated_at,
		COALESCE(t.solve_plan_at < now() AND NOT st.is_final, false),
		COALESCE((SELECT json_agg(json_build_object('id', o.user_id::text, 'name', COALESCE(ou.name, o.user_id::text)) ORDER BY ou.name)
			FROM tasks.task_observer o LEFT JOIN {crm}.ref_user ou ON ou.id = o.user_id WHERE o.task_id = t.id), '[]')
	FROM tasks.task t
	JOIN tasks.task_type tt ON tt.id = t.task_type_id
	JOIN tasks.task_status st ON st.id = t.status_id
	LEFT JOIN tasks.task_type_result r ON r.id = t.result_id
	LEFT JOIN {crm}.ref_user u ON u.id = t.executor_user_id
	LEFT JOIN {crm}.ref_contractor c ON c.id = t.contractor_id
	LEFT JOIN {crm}.ref_contractor cc ON cc.id = t.contact_contractor_id`)
}

func scanTask(row pgx.Row) (*Task, error) {
	var t Task
	var data, observers []byte
	err := row.Scan(&t.ID, &t.Number, &t.TaskTypeID, &t.TypeName, &t.TypeColor, &t.FormVersion,
		&t.StatusID, &t.StatusName, &t.ResultID, &t.ResultName, &t.ResultColor, &t.Priority, &t.Title, &t.Description,
		&t.ExecutorUserID, &t.ExecutorName, &t.ContractorID, &t.ContractorName, &t.ContactContractorID, &t.ContactName,
		&t.SolvePlanAt, &t.SolveFactAt, &data, &t.Source, &t.ExternalID, &t.LegacyTaskID, &t.CreatedAt, &t.UpdatedAt, &t.Overdue, &observers)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	t.Observers = []RefItem{}
	if err := json.Unmarshal(observers, &t.Observers); err != nil {
		return nil, err
	}
	t.Data = map[string]any{}
	if len(data) > 0 {
		if err := json.Unmarshal(data, &t.Data); err != nil {
			return nil, err
		}
	}
	return &t, nil
}

func (s *Store) GetTask(ctx context.Context, q Querier, id string) (*Task, error) {
	return scanTask(q.QueryRow(ctx, s.taskSelect()+` WHERE t.id=$1`, id))
}

type TaskFilter struct {
	TypeID       int
	StatusIDs    []int
	ResultID     int
	ExecutorID   string
	ContractorID string
	ContactID    string
	Source       string
	Query        string
	Overdue      bool
	CreatedFrom  *time.Time
	CreatedTo    *time.Time
	PlanFrom     *time.Time
	PlanTo       *time.Time
	ExcludeID    string
	ObserverID   string
	Sort         string
	Desc         bool
	Page         int
	PerPage      int
}

var sortColumns = map[string]string{
	"number":        "t.number",
	"created_at":    "t.created_at",
	"solve_plan_at": "t.solve_plan_at",
	"priority":      "t.priority",
	"status":        "t.status_id",
	"type":          "tt.name",
	"executor":      "u.name",
	"updated_at":    "t.updated_at",
}

// buildWhere собирает условие WHERE и аргументы по фильтру (общий для списка и счётчиков).
func buildWhere(f TaskFilter) (string, []any) {
	where := []string{"TRUE"}
	args := []any{}
	add := func(cond string, v any) {
		args = append(args, v)
		where = append(where, strings.ReplaceAll(cond, "?", fmt.Sprintf("$%d", len(args))))
	}
	if f.TypeID > 0 {
		add("t.task_type_id = ?", f.TypeID)
	}
	if len(f.StatusIDs) > 0 {
		add("t.status_id = ANY(?)", f.StatusIDs)
	}
	if f.ResultID > 0 {
		add("t.result_id = ?", f.ResultID)
	}
	if f.ExecutorID != "" {
		add("t.executor_user_id = ?", f.ExecutorID)
	}
	if f.ContractorID != "" {
		add("t.contractor_id = ?", f.ContractorID)
	}
	if f.ContactID != "" {
		add("t.contact_contractor_id = ?", f.ContactID)
	}
	if f.Source != "" {
		add("t.source = ?", f.Source)
	}
	if f.ObserverID != "" {
		add("EXISTS (SELECT 1 FROM tasks.task_observer o WHERE o.task_id = t.id AND o.user_id = ?)", f.ObserverID)
	}
	if f.ExcludeID != "" {
		add("t.id <> ?", f.ExcludeID)
	}
	if f.Overdue {
		where = append(where, "t.solve_plan_at < now() AND NOT st.is_final")
	}
	if f.CreatedFrom != nil {
		add("t.created_at >= ?", *f.CreatedFrom)
	}
	if f.CreatedTo != nil {
		add("t.created_at < ?", *f.CreatedTo)
	}
	if f.PlanFrom != nil {
		add("t.solve_plan_at >= ?", *f.PlanFrom)
	}
	if f.PlanTo != nil {
		add("t.solve_plan_at < ?", *f.PlanTo)
	}
	if q := strings.TrimSpace(f.Query); q != "" {
		// номер — точное совпадение, текст — по цели, описанию и значениям полей
		args = append(args, "%"+q+"%", strings.TrimLeft(q, "№# "))
		like, num := len(args)-1, len(args)
		where = append(where, fmt.Sprintf(`(t.number::text = $%[2]d OR t.title ILIKE $%[1]d OR t.description ILIKE $%[1]d OR t.data::text ILIKE $%[1]d)`, like, num))
	}
	return strings.Join(where, " AND "), args
}

func (s *Store) ListTasks(ctx context.Context, f TaskFilter) ([]Task, int, error) {
	cond, args := buildWhere(f)
	base := s.taskSelect()
	from := base[strings.Index(base, "FROM tasks.task t"):]

	var total int
	if err := s.Pool.QueryRow(ctx, "SELECT count(*) "+from+" WHERE "+cond, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	order := "t.created_at"
	if col, ok := sortColumns[f.Sort]; ok {
		order = col
	}
	dir := "ASC"
	if f.Desc || f.Sort == "" {
		dir = "DESC"
	}
	if f.PerPage <= 0 || f.PerPage > 200 {
		f.PerPage = 20
	}
	if f.Page <= 0 {
		f.Page = 1
	}
	sql := fmt.Sprintf("%s WHERE %s ORDER BY %s %s NULLS LAST, t.number DESC LIMIT %d OFFSET %d",
		base, cond, order, dir, f.PerPage, (f.Page-1)*f.PerPage)
	rows, err := s.Pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	res := []Task{}
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, 0, err
		}
		res = append(res, *t)
	}
	return res, total, rows.Err()
}

// TaskRow — значения колонок для вставки/обновления.
type TaskRow struct {
	ID                  string
	TaskTypeID          int
	FormVersion         int
	StatusID            int
	ResultID            *int
	Priority            int
	Title               string
	Description         string
	ExecutorUserID      *string
	ContractorID        *string
	ContactContractorID *string
	SolvePlanAt         *time.Time
	SolveFactAt         *time.Time
	Data                map[string]any
	Source              string
	ExternalID          *string
	LegacyTaskID        *string
	CreatedBy           *string
}

// InsertTask вставляет задачу. Если задача с таким (source, external_id) уже есть — возвращает (false, nil).
func (s *Store) InsertTask(ctx context.Context, q Querier, r TaskRow) (bool, error) {
	data, err := json.Marshal(r.Data)
	if err != nil {
		return false, err
	}
	ct, err := q.Exec(ctx, `INSERT INTO tasks.task (id, task_type_id, form_version, status_id, result_id, priority, title, description,
		executor_user_id, contractor_id, contact_contractor_id, solve_plan_at, solve_fact_at, data, source, external_id, legacy_task_id, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18)
		ON CONFLICT (source, external_id) WHERE external_id IS NOT NULL DO NOTHING`,
		r.ID, r.TaskTypeID, r.FormVersion, r.StatusID, r.ResultID, r.Priority, r.Title, r.Description,
		r.ExecutorUserID, r.ContractorID, r.ContactContractorID, r.SolvePlanAt, r.SolveFactAt, data, r.Source, r.ExternalID, r.LegacyTaskID, r.CreatedBy)
	if err != nil {
		return false, err
	}
	return ct.RowsAffected() > 0, nil
}

func (s *Store) FindByExternal(ctx context.Context, source, externalID string) (string, error) {
	var id string
	err := s.Pool.QueryRow(ctx, `SELECT id FROM tasks.task WHERE source=$1 AND external_id=$2`, source, externalID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	return id, err
}

func (s *Store) UpdateTask(ctx context.Context, q Querier, r TaskRow) error {
	data, err := json.Marshal(r.Data)
	if err != nil {
		return err
	}
	_, err = q.Exec(ctx, `UPDATE tasks.task SET form_version=$2, status_id=$3, result_id=$4, priority=$5, title=$6, description=$7,
		executor_user_id=$8, contractor_id=$9, contact_contractor_id=$10, solve_plan_at=$11, solve_fact_at=$12, data=$13, updated_at=now()
		WHERE id=$1`,
		r.ID, r.FormVersion, r.StatusID, r.ResultID, r.Priority, r.Title, r.Description,
		r.ExecutorUserID, r.ContractorID, r.ContactContractorID, r.SolvePlanAt, r.SolveFactAt, data)
	return err
}

// LockTask блокирует строку задачи на время транзакции обновления.
func (s *Store) LockTask(ctx context.Context, tx pgx.Tx, id string) error {
	var x string
	err := tx.QueryRow(ctx, `SELECT id FROM tasks.task WHERE id=$1 FOR UPDATE`, id).Scan(&x)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

// ---- журнал ----

type Event struct {
	ID        int64          `json:"id"`
	Kind      string         `json:"kind"`
	Message   string         `json:"message"`
	Payload   map[string]any `json:"payload"`
	CreatedAt time.Time      `json:"created_at"`
}

func (s *Store) AddEvent(ctx context.Context, q Querier, taskID, kind, message string, payload any) error {
	var raw []byte
	if payload != nil {
		raw, _ = json.Marshal(payload)
	}
	_, err := q.Exec(ctx, `INSERT INTO tasks.task_event (task_id, kind, message, payload) VALUES ($1,$2,$3,$4)`, taskID, kind, message, raw)
	return err
}

func (s *Store) ListEvents(ctx context.Context, taskID string) ([]Event, error) {
	rows, err := s.Pool.Query(ctx, `SELECT id, kind, message, payload, created_at FROM tasks.task_event WHERE task_id=$1 ORDER BY id DESC LIMIT 200`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	res := []Event{}
	for rows.Next() {
		var e Event
		var raw []byte
		if err := rows.Scan(&e.ID, &e.Kind, &e.Message, &raw, &e.CreatedAt); err != nil {
			return nil, err
		}
		if len(raw) > 0 {
			_ = json.Unmarshal(raw, &e.Payload)
		}
		res = append(res, e)
	}
	return res, rows.Err()
}

// TaskStats — счётчики для вкладок над списком.
type TaskStats struct {
	Total    int         `json:"total"`
	ByStatus map[int]int `json:"by_status"`
	Overdue  int         `json:"overdue"`
}

// Stats считает задачи по статусам с учётом фильтра (кроме фильтров статуса и просрочки — по ним и есть вкладки).
func (s *Store) Stats(ctx context.Context, f TaskFilter) (*TaskStats, error) {
	f.StatusIDs, f.Overdue = nil, false
	cond, args := buildWhere(f)
	base := s.taskSelect()
	from := base[strings.Index(base, "FROM tasks.task t"):]
	rows, err := s.Pool.Query(ctx, `SELECT t.status_id, count(*), count(*) FILTER (WHERE t.solve_plan_at < now() AND NOT st.is_final) `+
		from+` WHERE `+cond+` GROUP BY t.status_id`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	res := &TaskStats{ByStatus: map[int]int{}}
	for rows.Next() {
		var status, n, overdue int
		if err := rows.Scan(&status, &n, &overdue); err != nil {
			return nil, err
		}
		res.ByStatus[status] = n
		res.Total += n
		res.Overdue += overdue
	}
	return res, rows.Err()
}

// SetObservers заменяет список наблюдателей задачи.
func (s *Store) SetObservers(ctx context.Context, q Querier, taskID string, userIDs []string) error {
	if _, err := q.Exec(ctx, `DELETE FROM tasks.task_observer WHERE task_id=$1 AND NOT (user_id::text = ANY($2))`, taskID, userIDs); err != nil {
		return err
	}
	_, err := q.Exec(ctx, `INSERT INTO tasks.task_observer (task_id, user_id) SELECT $1, u::uuid FROM unnest($2::text[]) u ON CONFLICT DO NOTHING`, taskID, userIDs)
	return err
}

// CountUsers — сколько из переданных id есть в ref_user.
func (s *Store) CountUsers(ctx context.Context, ids []string) (int, error) {
	var n int
	err := s.Pool.QueryRow(ctx, s.q(`SELECT count(*) FROM {crm}.ref_user WHERE id::text = ANY($1)`), ids).Scan(&n)
	return n, err
}
