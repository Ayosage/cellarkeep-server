package auth

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	ClientOrigin string
	Insecure     bool
}

type SessionLookup interface {
	Principal(ctx context.Context, sessionID string) (Principal, time.Time, error)
}

type DeviceLookup interface {
	PrincipalForToken(ctx context.Context, token string) (Principal, error)
}

func problem(w http.ResponseWriter, status int, title string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(`{"type":"about:blank","title":"` + title + `","status":` + strconv.Itoa(status) + `}`))
}

func unsafe(m string) bool {
	return m == http.MethodPost || m == http.MethodPut || m == http.MethodPatch || m == http.MethodDelete
}

// Middleware resolves the caller. It never rejects on its own except for the
// origin rule; RequireUser / RequireAdmin / RequireDevice do the gating.
func Middleware(cfg Config, sessions SessionLookup, devices DeviceLookup) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
				p, err := devices.PrincipalForToken(ctx, strings.TrimPrefix(h, "Bearer "))
				if err != nil {
					slog.Warn("device token rejected", "ip", r.RemoteAddr)
					problem(w, http.StatusUnauthorized, "invalid device token")
					return
				}
				next.ServeHTTP(w, r.WithContext(WithPrincipal(ctx, p)))
				return
			}
			if c, err := r.Cookie(CookieName); err == nil {
				p, exp, err := sessions.Principal(ctx, c.Value)
				if err == nil {
					if unsafe(r.Method) && r.Header.Get("Origin") != cfg.ClientOrigin {
						slog.Warn("origin rejected", "origin", r.Header.Get("Origin"), "ip", r.RemoteAddr)
						problem(w, http.StatusForbidden, "origin not allowed")
						return
					}
					if time.Until(exp) < SessionTTL/2 {
						SetSessionCookie(w, c.Value, time.Now().Add(SessionTTL), cfg.Insecure)
					}
					next.ServeHTTP(w, r.WithContext(WithPrincipal(ctx, p)))
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

func RequireUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, ok := FromContext(r.Context())
		if !ok {
			problem(w, http.StatusUnauthorized, "sign in required")
			return
		}
		if p.Kind != PrincipalKindUser {
			problem(w, http.StatusForbidden, "device tokens cannot use this route")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func RequireAdmin(next http.Handler) http.Handler {
	return RequireUser(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, _ := FromContext(r.Context())
		if p.Role != "admin" {
			problem(w, http.StatusForbidden, "admin only")
			return
		}
		next.ServeHTTP(w, r)
	}))
}

func RequireDevice(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, ok := FromContext(r.Context())
		if !ok || p.Kind != PrincipalKindDevice {
			problem(w, http.StatusForbidden, "device token required")
			return
		}
		next.ServeHTTP(w, r)
	})
}
