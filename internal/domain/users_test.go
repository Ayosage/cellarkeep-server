package domain_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ayosage/cellarkeep-server/internal/domain"
	"github.com/ayosage/cellarkeep-server/internal/store"
	"github.com/ayosage/cellarkeep-server/internal/store/testdb"
)

func txStore(t *testing.T) *store.Store {
	t.Helper()
	return testdb.TxStore(t, testdb.Open(t))
}

func TestSetupOnceThenInviteOnly(t *testing.T) {
	ctx := context.Background()
	s := txStore(t)
	users := domain.Users{S: s}
	admin, err := users.Setup(ctx, "Brandon@Example.com", "Brandon", "correct-horse-battery")
	if err != nil || admin.Role != "admin" || admin.Email != "brandon@example.com" {
		t.Fatalf("%+v %v", admin, err)
	}
	if _, err := users.Setup(ctx, "b@x.com", "B", "passwordpassword"); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("second setup: %v", err)
	}
	if _, err := users.Register(ctx, "nope", "c@x.com", "C", "passwordpassword", time.Now()); err == nil {
		t.Fatal("register without invite must fail")
	}
	inv, err := domain.Invites{S: s}.Create(ctx, admin.ID, nil, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	member, err := users.Register(ctx, inv.Token, "c@x.com", "C", "passwordpassword", time.Now())
	if err != nil || member.Role != "member" {
		t.Fatalf("%+v %v", member, err)
	}
	if _, err := users.Register(ctx, inv.Token, "d@x.com", "D", "passwordpassword", time.Now()); err == nil {
		t.Fatal("invite must be single use")
	}
	if _, err := users.Login(ctx, "c@x.com", "wrong"); !errors.Is(err, domain.ErrForbidden) {
		t.Fatal("wrong password must be forbidden")
	}
	if _, err := users.Login(ctx, "c@x.com", "passwordpassword"); err != nil {
		t.Fatal(err)
	}
}

func TestExpiredInviteIsInvalid(t *testing.T) {
	ctx := context.Background()
	s := txStore(t)
	admin, _ := domain.Users{S: s}.Setup(ctx, "a@x.com", "A", "passwordpassword")
	inv, _ := domain.Invites{S: s}.Create(ctx, admin.ID, nil, time.Now().Add(-8*24*time.Hour))
	if _, err := (domain.Invites{S: s}).FindValid(ctx, inv.Token, time.Now()); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("got %v", err)
	}
}
