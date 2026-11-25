package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type Comment struct {
	ID           string    `json:"id"`
	TaskID       string    `json:"task_id"`
	Text         string    `json:"text"`
	AuthorUserID *string   `json:"author_user_id"`
	AuthorName   *string   `json:"author_name"`
	IsFavorite   bool      `json:"is_favorite"`
	IsSystem     bool      `json:"is_system"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (s *Store) commentSelect() string {
	return s.q(`SELECT c.id, c.task_id, c.text, c.author_user_id, u.name, c.is_favorite, c.is_system, c.created_at, c.updated_at
		FROM tasks.task_comment c LEFT JOIN {crm}.ref_user u ON u.id = c.author_user_id`)
}

func scanComment(row pgx.Row) (*Comment, error) {
	var c Comment
	err := row.Scan(&c.ID, &c.TaskID, &c.Text, &c.AuthorUserID, &c.AuthorName, &c.IsFavorite, &c.IsSystem, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &c, err
}

func (s *Store) ListComments(ctx context.Context, taskID string) ([]Comment, error) {
	rows, err := s.Pool.Query(ctx, s.commentSelect()+` WHERE c.task_id=$1 ORDER BY c.is_favorite DESC, c.created_at DESC`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	res := []Comment{}
	for rows.Next() {
		c, err := scanComment(rows)
		if err != nil {
			return nil, err
		}
		res = append(res, *c)
	}
	return res, rows.Err()
}

func (s *Store) GetComment(ctx context.Context, id string) (*Comment, error) {
	return scanComment(s.Pool.QueryRow(ctx, s.commentSelect()+` WHERE c.id=$1`, id))
}

func (s *Store) AddComment(ctx context.Context, taskID, text string, authorID *string) (*Comment, error) {
	id := uuid.NewString()
	if _, err := s.Pool.Exec(ctx, `INSERT INTO tasks.task_comment (id, task_id, text, author_user_id) VALUES ($1,$2,$3,$4)`,
		id, taskID, text, authorID); err != nil {
		return nil, err
	}
	return s.GetComment(ctx, id)
}

func (s *Store) UpdateComment(ctx context.Context, id string, text *string, favorite *bool) (*Comment, error) {
	ct, err := s.Pool.Exec(ctx, `UPDATE tasks.task_comment SET text=COALESCE($2, text), is_favorite=COALESCE($3, is_favorite), updated_at=now() WHERE id=$1`,
		id, text, favorite)
	if err != nil {
		return nil, err
	}
	if ct.RowsAffected() == 0 {
		return nil, ErrNotFound
	}
	return s.GetComment(ctx, id)
}

func (s *Store) DeleteComment(ctx context.Context, id string) error {
	ct, err := s.Pool.Exec(ctx, `DELETE FROM tasks.task_comment WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
