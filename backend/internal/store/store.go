package store

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("not found")

// Querier — общий интерфейс пула и транзакции.
type Querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type Store struct {
	Pool *pgxpool.Pool
	crm  string // схема CRM (public)
}

var identRe = regexp.MustCompile(`^[a-z_][a-z0-9_]*$`)

func New(pool *pgxpool.Pool, crmSchema string) (*Store, error) {
	if !identRe.MatchString(crmSchema) {
		return nil, fmt.Errorf("invalid CRM schema name %q", crmSchema)
	}
	return &Store{Pool: pool, crm: crmSchema}, nil
}

// q подставляет имя схемы CRM вместо {crm} в тексте запроса.
func (s *Store) q(sql string) string { return strings.ReplaceAll(sql, "{crm}", s.crm) }

// Q — экспортируемый вариант q для сервисного слоя.
func (s *Store) Q(sql string) string { return s.q(sql) }

// contractorName — SQL-выражение имени контрагента (для физлиц собирается из ФИО).
const contractorNameExpr = `COALESCE(NULLIF(%[1]s.name, ''), NULLIF(concat_ws(' ', %[1]s.last_name, %[1]s.first_name, %[1]s.middle_name), ''))`

func cname(alias string) string { return fmt.Sprintf(contractorNameExpr, alias) }

// WithTx выполняет fn в транзакции.
func (s *Store) WithTx(ctx context.Context, fn func(tx pgx.Tx) error) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func IsUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
