package domain

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/ayosage/cellarkeep-server/internal/auth"
	"github.com/ayosage/cellarkeep-server/internal/store"
	"github.com/ayosage/cellarkeep-server/internal/store/gen"
)

const InviteTTL = 7 * 24 * time.Hour

type Users struct{ S *store.Store }

func normEmail(e string) string { return strings.ToLower(strings.TrimSpace(e)) }

func (u Users) Setup(ctx context.Context, email, name, password string) (gen.User, error) {
	var out gen.User
	err := u.S.WithTx(ctx, func(q *gen.Queries) error {
		// Serialise first-setup attempts. Without this, two concurrent
		// requests both read a count of zero under READ COMMITTED and both
		// create an admin. The lock is transaction scoped, so it releases on
		// commit or rollback.
		if err := q.LockSetup(ctx); err != nil {
			return err
		}
		n, err := q.CountUsers(ctx)
		if err != nil {
			return err
		}
		if n > 0 {
			return ErrConflict
		}
		hash, err := auth.HashPassword(password)
		if err != nil {
			return err
		}
		out, err = q.CreateUser(ctx, gen.CreateUserParams{
			Email: normEmail(email), Name: name, PasswordHash: hash, Role: gen.UserRoleAdmin,
		})
		return err
	})
	return out, err
}

// decoyHash stands in for the password hash of an address that has no user.
// It is computed once at startup so that the unknown-email path costs the
// same one argon2id verification as the known-email path. Hashing it per
// request would make unknown addresses take roughly twice as long and leak
// which addresses are registered.
var decoyHash = mustHash("decoy-so-verify-still-runs")

func mustHash(pw string) string {
	h, err := auth.HashPassword(pw)
	if err != nil {
		panic("domain: cannot hash decoy password: " + err.Error())
	}
	return h
}

// Login always runs exactly one hash check so timing does not reveal whether
// the email exists. Any failure is ErrForbidden.
func (u Users) Login(ctx context.Context, email, password string) (gen.User, error) {
	user, err := u.S.Q.GetUserByEmail(ctx, normEmail(email))
	hash := user.PasswordHash
	if err != nil {
		hash = decoyHash
	}
	ok, _ := auth.VerifyPassword(hash, password)
	if err != nil || !ok {
		return gen.User{}, ErrForbidden
	}
	return user, nil
}

func (u Users) Register(ctx context.Context, token, email, name, password string, now time.Time) (gen.User, error) {
	var out gen.User
	err := u.S.WithTx(ctx, func(q *gen.Queries) error {
		inv, err := q.FindValidInvite(ctx, gen.FindValidInviteParams{Token: token, Now: now})
		if errors.Is(err, pgx.ErrNoRows) {
			return invalid("token", "invite is not valid")
		}
		if err != nil {
			return err
		}
		if inv.Email != nil && *inv.Email != normEmail(email) {
			return invalid("email", "invite is for a different address")
		}
		hash, err := auth.HashPassword(password)
		if err != nil {
			return err
		}
		out, err = q.CreateUser(ctx, gen.CreateUserParams{
			Email: normEmail(email), Name: name, PasswordHash: hash, Role: gen.UserRoleMember,
		})
		if err != nil {
			return err
		}
		_, err = q.ClaimInvite(ctx, gen.ClaimInviteParams{Token: token, Now: &now, UsedBy: &out.ID})
		return err
	})
	return out, err
}

func (u Users) ChangePassword(ctx context.Context, userID uuid.UUID, current, next string) error {
	user, err := u.S.Q.GetUser(ctx, userID)
	if err != nil {
		return ErrNotFound
	}
	if ok, _ := auth.VerifyPassword(user.PasswordHash, current); !ok {
		return ErrForbidden
	}
	if len(next) < 10 {
		return invalid("password", "must be at least 10 characters")
	}
	hash, err := auth.HashPassword(next)
	if err != nil {
		return err
	}
	return u.S.WithTx(ctx, func(q *gen.Queries) error {
		if err := q.UpdateUserPassword(ctx, gen.UpdateUserPasswordParams{ID: userID, PasswordHash: hash}); err != nil {
			return err
		}
		return q.DeleteUserSessions(ctx, userID)
	})
}

// Sessions implements auth.SessionLookup over the store.
type Sessions struct{ S *store.Store }

func (s Sessions) Create(ctx context.Context, userID uuid.UUID, now time.Time) (string, time.Time, error) {
	id, err := auth.NewSessionID()
	if err != nil {
		return "", time.Time{}, err
	}
	exp := now.Add(auth.SessionTTL)
	return id, exp, s.S.Q.CreateSession(ctx, gen.CreateSessionParams{ID: id, UserID: userID, ExpiresAt: exp})
}

func (s Sessions) Principal(ctx context.Context, id string) (auth.Principal, time.Time, error) {
	row, err := s.S.Q.GetSessionWithUser(ctx, id)
	if err != nil {
		return auth.Principal{}, time.Time{}, ErrForbidden
	}
	if time.Until(row.ExpiresAt) < auth.SessionTTL/2 {
		_ = s.S.Q.TouchSession(ctx, gen.TouchSessionParams{ID: id, ExpiresAt: time.Now().Add(auth.SessionTTL)})
	}
	return auth.Principal{Kind: auth.PrincipalKindUser, UserID: row.UserID, Role: string(row.Role)}, row.ExpiresAt, nil
}

func (s Sessions) Delete(ctx context.Context, id string) error { return s.S.Q.DeleteSession(ctx, id) }

type Invites struct{ S *store.Store }

func (i Invites) Create(ctx context.Context, createdBy uuid.UUID, email *string, now time.Time) (gen.Invite, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return gen.Invite{}, err
	}
	if email != nil {
		e := normEmail(*email)
		if e == "" {
			email = nil
		} else {
			email = &e
		}
	}
	return i.S.Q.CreateInvite(ctx, gen.CreateInviteParams{
		Token: base64.RawURLEncoding.EncodeToString(b), Email: email, CreatedBy: createdBy,
		CreatedAt: now, ExpiresAt: now.Add(InviteTTL),
	})
}

func (i Invites) List(ctx context.Context) ([]gen.ListInvitesRow, error) {
	return i.S.Q.ListInvites(ctx)
}

func (i Invites) Revoke(ctx context.Context, id uuid.UUID, now time.Time) (gen.Invite, error) {
	row, err := i.S.Q.RevokeInvite(ctx, gen.RevokeInviteParams{ID: id, Now: &now})
	if errors.Is(err, pgx.ErrNoRows) {
		return gen.Invite{}, ErrConflict
	}
	return row, err
}

func (i Invites) FindValid(ctx context.Context, token string, now time.Time) (gen.Invite, error) {
	row, err := i.S.Q.FindValidInvite(ctx, gen.FindValidInviteParams{Token: token, Now: now})
	if errors.Is(err, pgx.ErrNoRows) {
		return gen.Invite{}, ErrNotFound
	}
	return row, err
}
