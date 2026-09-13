package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
)

type fakeSessions struct{ id string }

func (f fakeSessions) Principal(_ context.Context, id string) (Principal, time.Time, error) {
	if id == f.id {
		return Principal{Kind: PrincipalKindUser, UserID: uuid.New(), Role: "admin"}, time.Now().Add(time.Hour), nil
	}
	return Principal{}, time.Time{}, errors.New("no session")
}

type fakeDevices struct{ token string }

func (f fakeDevices) PrincipalForToken(_ context.Context, tok string) (Principal, error) {
	if tok == f.token {
		return Principal{Kind: PrincipalKindDevice, DeviceID: uuid.New()}, nil
	}
	return Principal{}, errors.New("bad token")
}

func handler(t *testing.T) http.Handler {
	mw := Middleware(Config{ClientOrigin: "http://localhost:3000", Insecure: true}, fakeSessions{"good"}, fakeDevices{"devtok"})
	return mw(RequireUser(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})))
}

func TestNoCredentialsIs401(t *testing.T) {
	w := httptest.NewRecorder()
	handler(t).ServeHTTP(w, httptest.NewRequest("GET", "/x", nil))
	if w.Code != 401 {
		t.Fatalf("got %d", w.Code)
	}
}

func TestSessionPostNeedsOrigin(t *testing.T) {
	r := httptest.NewRequest("POST", "/x", nil)
	r.AddCookie(&http.Cookie{Name: CookieName, Value: "good"})
	w := httptest.NewRecorder()
	handler(t).ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatalf("missing origin: got %d", w.Code)
	}
	r.Header.Set("Origin", "http://localhost:3000")
	w = httptest.NewRecorder()
	handler(t).ServeHTTP(w, r)
	if w.Code != 204 {
		t.Fatalf("with origin: got %d", w.Code)
	}
}

func TestDeviceCannotUseUserRoute(t *testing.T) {
	r := httptest.NewRequest("GET", "/x", nil)
	r.Header.Set("Authorization", "Bearer devtok")
	w := httptest.NewRecorder()
	handler(t).ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatalf("got %d", w.Code)
	}
}
