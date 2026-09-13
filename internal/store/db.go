// Package store wraps the pgx pool and the sqlc-generated queries.
package store

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ayosage/cellarkeep-server/internal/store/gen"
)

type Store struct {
	Pool *pgxpool.Pool
	Q    *gen.Queries
	Tx   pgx.Tx
}

func New(pool *pgxpool.Pool) *Store {
	return &Store{Pool: pool, Q: gen.New(pool)}
}

// WithTx runs fn inside one transaction and commits when it returns nil. If
// the store already carries an open transaction, fn runs against it directly
// instead of starting a nested one.
func (s *Store) WithTx(ctx context.Context, fn func(q *gen.Queries) error) error {
	if s.Tx != nil {
		return fn(s.Q)
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	if err := fn(s.Q.WithTx(tx)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
