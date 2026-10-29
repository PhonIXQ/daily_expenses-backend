package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Store wraps the generated queries with a connection pool and transactions.
type Store struct {
	*Queries
	pool *pgxpool.Pool
}

func Open(pool *pgxpool.Pool) *Store {
	return &Store{Queries: &Queries{db: pool}, pool: pool}
}

func (s *Store) Ping(ctx context.Context) error { return s.pool.Ping(ctx) }

// InTx runs fn in a transaction, committing if it returns nil.
func (s *Store) InTx(ctx context.Context, fn func(q *Queries) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after commit
	if err := fn(s.WithTx(tx)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// IsNotFound reports whether err is a "no rows" result.
func IsNotFound(err error) bool { return errors.Is(err, pgx.ErrNoRows) }

// IsUniqueViolation reports whether err is a unique/primary-key conflict.
func IsUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
