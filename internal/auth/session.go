package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"time"

	"github.com/google/uuid"
)

const CookieName = "ck_session"
const SessionTTL = 30 * 24 * time.Hour

func NewSessionID() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func SetSessionCookie(w http.ResponseWriter, id string, expires time.Time, insecure bool) {
	http.SetCookie(w, &http.Cookie{
		Name: CookieName, Value: id, Path: "/", Expires: expires,
		HttpOnly: true, Secure: !insecure, SameSite: http.SameSiteLaxMode,
	})
}

func ClearSessionCookie(w http.ResponseWriter, insecure bool) {
	http.SetCookie(w, &http.Cookie{
		Name: CookieName, Value: "", Path: "/", MaxAge: -1,
		HttpOnly: true, Secure: !insecure, SameSite: http.SameSiteLaxMode,
	})
}

type PrincipalKind string

const (
	PrincipalKindUser   PrincipalKind = "user"
	PrincipalKindDevice PrincipalKind = "device"
)

type Principal struct {
	Kind     PrincipalKind
	UserID   uuid.UUID
	Role     string
	DeviceID uuid.UUID
}

type ctxKey struct{}

func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, ctxKey{}, p)
}

func FromContext(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(ctxKey{}).(Principal)
	return p, ok
}
