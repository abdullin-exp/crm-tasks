package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"crm-tasks/internal/form"
)

type Result struct {
	ID         int    `json:"id"`
	TaskTypeID int    `json:"task_type_id"`
	Name       string `json:"name"`
	Color      string `json:"color"`
	IsClosing  bool   `json:"is_closing"`
	Sort       int    `json:"sort"`
	IsActive   bool   `json:"is_active"`
}

type TaskType struct {
	ID              int         `json:"id"`
	Code            string      `json:"code"`
	Name            string      `json:"name"`
	Description     string      `json:"description"`
	Color           string      `json:"color"`
	IsActive        bool        `json:"is_active"`
	DefaultPriority int         `json:"default_priority"`
	SolveMinutes    *int        `json:"solve_minutes"`
	LegacyEntityID  *int        `json:"legacy_entity_id"`
	Form            form.Schema `json:"form"`
	FormVersion     int         `json:"form_version"`
	CreatedAt       time.Time   `json:"created_at"`
	UpdatedAt       time.Time   `json:"updated_at"`
	Results         []Result    `json:"results"`
	TasksCount      int         `json:"tasks_count"`
}

const typeCols = `t.id, t.code, t.name, t.description, t.color, t.is_active, t.default_priority, t.solve_minutes,
	t.legacy_entity_id, t.form_schema, t.form_version, t.created_at, t.updated_at,
	(SELECT count(*) FROM tasks.task x WHERE x.task_type_id = t.id)`

func scanType(row pgx.Row) (*TaskType, error) {
	var tt TaskType
	var schemaRaw []byte
	err := row.Scan(&tt.ID, &tt.Code, &tt.Name, &tt.Description, &tt.Color, &tt.IsActive, &tt.DefaultPriority,
		&tt.SolveMinutes, &tt.LegacyEntityID, &schemaRaw, &tt.FormVersion, &tt.CreatedAt, &tt.UpdatedAt, &tt.TasksCount)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(schemaRaw, &tt.Form); err != nil {
		return nil, err
	}
	tt.Form.Normalize()
	tt.Results = []Result{}
	return &tt, nil
}

func (s *Store) ListTypes(ctx context.Context, onlyActive bool) ([]TaskType, error) {
	sql := `SELECT ` + typeCols + ` FROM tasks.task_type t`
	if onlyActive {
		sql += ` WHERE t.is_active`
	}
	sql += ` ORDER BY t.name`
	rows, err := s.Pool.Query(ctx, sql)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	res := []TaskType{}
	for rows.Next() {
		tt, err := scanType(rows)
		if err != nil {
			return nil, err
		}
		res = append(res, *tt)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	results, err := s.listResults(ctx, s.Pool, 0)
	if err != nil {
		return nil, err
	}
	for i := range res {
		for _, r := range results {
			if r.TaskTypeID == res[i].ID {
				res[i].Results = append(res[i].Results, r)
			}
		}
	}
	return res, nil
}

func (s *Store) GetType(ctx context.Context, q Querier, id int) (*TaskType, error) {
	tt, err := scanType(q.QueryRow(ctx, `SELECT `+typeCols+` FROM tasks.task_type t WHERE t.id=$1`, id))
	if err != nil {
		return nil, err
	}
	if tt.Results, err = s.listResults(ctx, q, id); err != nil {
		return nil, err
	}
	return tt, nil
}

func (s *Store) GetTypeByCode(ctx context.Context, code string) (*TaskType, error) {
	var id int
	err := s.Pool.QueryRow(ctx, `SELECT id FROM tasks.task_type WHERE code=$1`, code).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return s.GetType(ctx, s.Pool, id)
}

func (s *Store) listResults(ctx context.Context, q Querier, typeID int) ([]Result, error) {
	rows, err := q.Query(ctx, `SELECT id, task_type_id, name, color, is_closing, sort, is_active
		FROM tasks.task_type_result WHERE ($1 = 0 OR task_type_id = $1) ORDER BY sort, id`, typeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	res := []Result{}
	for rows.Next() {
		var r Result
		if err := rows.Scan(&r.ID, &r.TaskTypeID, &r.Name, &r.Color, &r.IsClosing, &r.Sort, &r.IsActive); err != nil {
			return nil, err
		}
		res = append(res, r)
	}
	return res, rows.Err()
}

// SaveType создаёт (ID == 0) или обновляет тип вместе со списком результатов.
// Результаты, отсутствующие в списке, деактивируются (на них могут ссылаться задачи).
func (s *Store) SaveType(ctx context.Context, tt *TaskType) error {
	return s.WithTx(ctx, func(tx pgx.Tx) error {
		if tt.ID == 0 {
			schemaRaw, _ := json.Marshal(tt.Form)
			err := tx.QueryRow(ctx, `INSERT INTO tasks.task_type
				(code, name, description, color, is_active, default_priority, solve_minutes, legacy_entity_id, form_schema)
				VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING id`,
				tt.Code, tt.Name, tt.Description, tt.Color, tt.IsActive, tt.DefaultPriority, tt.SolveMinutes, tt.LegacyEntityID, schemaRaw,
			).Scan(&tt.ID)
			if err != nil {
				return err
			}
		} else {
			ct, err := tx.Exec(ctx, `UPDATE tasks.task_type SET code=$2, name=$3, description=$4, color=$5, is_active=$6,
				default_priority=$7, solve_minutes=$8, legacy_entity_id=$9, updated_at=now() WHERE id=$1`,
				tt.ID, tt.Code, tt.Name, tt.Description, tt.Color, tt.IsActive, tt.DefaultPriority, tt.SolveMinutes, tt.LegacyEntityID)
			if err != nil {
				return err
			}
			if ct.RowsAffected() == 0 {
				return ErrNotFound
			}
		}

		keep := []int{}
		for i := range tt.Results {
			r := &tt.Results[i]
			r.Sort = i
			if r.ID > 0 {
				ct, err := tx.Exec(ctx, `UPDATE tasks.task_type_result SET name=$3, color=$4, is_closing=$5, sort=$6, is_active=true
					WHERE id=$1 AND task_type_id=$2`, r.ID, tt.ID, r.Name, r.Color, r.IsClosing, r.Sort)
				if err != nil {
					return err
				}
				if ct.RowsAffected() > 0 {
					keep = append(keep, r.ID)
					continue
				}
			}
			if err := tx.QueryRow(ctx, `INSERT INTO tasks.task_type_result (task_type_id, name, color, is_closing, sort)
				VALUES ($1,$2,$3,$4,$5) RETURNING id`, tt.ID, r.Name, r.Color, r.IsClosing, r.Sort).Scan(&r.ID); err != nil {
				return err
			}
			keep = append(keep, r.ID)
		}
		_, err := tx.Exec(ctx, `UPDATE tasks.task_type_result SET is_active=false WHERE task_type_id=$1 AND NOT (id = ANY($2))`, tt.ID, keep)
		return err
	})
}

// SaveForm сохраняет схему формы и увеличивает её версию.
func (s *Store) SaveForm(ctx context.Context, typeID int, schema form.Schema) (int, error) {
	raw, err := json.Marshal(schema)
	if err != nil {
		return 0, err
	}
	var version int
	err = s.Pool.QueryRow(ctx, `UPDATE tasks.task_type SET form_schema=$2, form_version=form_version+1, updated_at=now()
		WHERE id=$1 RETURNING form_version`, typeID, raw).Scan(&version)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrNotFound
	}
	return version, err
}

// DeleteType удаляет тип без задач.
func (s *Store) DeleteType(ctx context.Context, id int) error {
	ct, err := s.Pool.Exec(ctx, `DELETE FROM tasks.task_type t WHERE id=$1 AND NOT EXISTS (SELECT 1 FROM tasks.task x WHERE x.task_type_id=t.id)`, id)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
