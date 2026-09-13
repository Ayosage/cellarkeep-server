// Package testdb opens the test database once per process, migrates it, and
// hands each test a transaction that rolls back on cleanup. Tests skip when
// TEST_DATABASE_URL is unset so the pure suites run without Postgres.
package testdb

import (
	"context"
	"os"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ayosage/cellarkeep-server/internal/migrations"
	"github.com/ayosage/cellarkeep-server/internal/store"
	"github.com/ayosage/cellarkeep-server/internal/store/gen"
)

var (
	once sync.Once
	pool *pgxpool.Pool
	perr error
)

func Open(t *testing.T) *store.Store {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	once.Do(func() {
		pool, perr = pgxpool.New(context.Background(), url)
		if perr == nil {
			perr = migrations.Up(context.Background(), pool)
		}
	})
	if perr != nil {
		t.Fatalf("testdb: %v", perr)
	}
	return store.New(pool)
}

// Tx returns queries bound to a transaction that is rolled back when the test ends.
func Tx(t *testing.T, s *store.Store) *gen.Queries {
	t.Helper()
	tx, err := s.Pool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
	return s.Q.WithTx(tx)
}

// TxStore returns a Store bound to a transaction that is rolled back when the
// test ends. Its WithTx runs callbacks directly against that transaction
// instead of opening a nested one, so domain code under test behaves the same
// whether it is called from a handler or from a test.
func TxStore(t *testing.T, s *store.Store) *store.Store {
	t.Helper()
	tx, err := s.Pool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
	return &store.Store{Pool: s.Pool, Q: s.Q.WithTx(tx), Tx: tx}
}
