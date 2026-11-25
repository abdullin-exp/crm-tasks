package store

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Типы исходящих событий
const (
	EventTaskCreated     = "task.created"
	EventTaskUpdated     = "task.updated"
	EventTaskClosed      = "task.closed"
	EventTaskReopened    = "task.reopened"
	EventTaskTaken       = "task.taken" // взята в работу
	EventTaskTransferred = "task.transferred"
)

// OutboxMessage — сообщение, которое уйдёт в Kafka.
type OutboxMessage struct {
	EventID    string         `json:"event_id"`
	EventType  string         `json:"event_type"`
	OccurredAt time.Time      `json:"occurred_at"`
	Task       *Task          `json:"task"`
	Details    map[string]any `json:"details,omitempty"`
}

// AddOutbox пишет событие с актуальным снимком задачи (вызывается внутри транзакции изменения).
func (s *Store) AddOutbox(ctx context.Context, tx pgx.Tx, eventType, taskID string, details map[string]any) error {
	t, err := s.GetTask(ctx, tx, taskID)
	if err != nil {
		return err
	}
	msg := OutboxMessage{EventID: uuid.NewString(), EventType: eventType, OccurredAt: time.Now(), Task: t, Details: details}
	raw, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO tasks.outbox (event_id, event_type, task_id, payload) VALUES ($1,$2,$3,$4)`,
		msg.EventID, eventType, taskID, raw)
	return err
}

type OutboxRow struct {
	ID        int64
	EventID   string
	EventType string
	TaskID    string
	Payload   []byte
}

// ProcessOutbox берёт пачку неотправленных событий под блокировкой (SKIP LOCKED — можно запускать несколько реплик),
// вызывает send и помечает отправленными. При ошибке send транзакция откатывается, увеличивается счётчик попыток.
func (s *Store) ProcessOutbox(ctx context.Context, limit int, send func([]OutboxRow) error) (int, error) {
	var sent int
	var sendErr error
	err := s.WithTx(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT id, event_id::text, event_type, task_id::text, payload FROM tasks.outbox
			WHERE sent_at IS NULL ORDER BY id LIMIT $1 FOR UPDATE SKIP LOCKED`, limit)
		if err != nil {
			return err
		}
		var batch []OutboxRow
		for rows.Next() {
			var r OutboxRow
			if err := rows.Scan(&r.ID, &r.EventID, &r.EventType, &r.TaskID, &r.Payload); err != nil {
				rows.Close()
				return err
			}
			batch = append(batch, r)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		if len(batch) == 0 {
			return nil
		}
		ids := make([]int64, len(batch))
		for i, r := range batch {
			ids[i] = r.ID
		}
		if sendErr = send(batch); sendErr != nil {
			_, err := tx.Exec(ctx, `UPDATE tasks.outbox SET attempts=attempts+1, last_error=$2 WHERE id = ANY($1)`, ids, sendErr.Error())
			return err // коммитим только счётчик попыток
		}
		_, err = tx.Exec(ctx, `UPDATE tasks.outbox SET sent_at=now(), attempts=attempts+1, last_error=NULL WHERE id = ANY($1)`, ids)
		sent = len(batch)
		return err
	})
	if err != nil {
		return 0, err
	}
	return sent, sendErr
}

// OutboxPending — число неотправленных событий.
func (s *Store) OutboxPending(ctx context.Context) (int, error) {
	var n int
	err := s.Pool.QueryRow(ctx, `SELECT count(*) FROM tasks.outbox WHERE sent_at IS NULL`).Scan(&n)
	return n, err
}

// CleanupOutbox удаляет отправленные события старше заданного срока.
func (s *Store) CleanupOutbox(ctx context.Context, olderThan time.Duration) error {
	_, err := s.Pool.Exec(ctx, `DELETE FROM tasks.outbox WHERE sent_at < $1`, time.Now().Add(-olderThan))
	return err
}
