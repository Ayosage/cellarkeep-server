package api

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/ayosage/cellarkeep-server/internal/auth"
)

func quietLog() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// TestContractRoutesResolve runs the same self-check NewRouter runs at boot,
// so a contract change that the gate cannot resolve fails here as well as at
// startup.
func TestContractRoutesResolve(t *testing.T) {
	spec, err := GetSwagger()
	if err != nil {
		t.Fatal(err)
	}
	if err := newRouteTable(spec).check(spec); err != nil {
		t.Fatal(err)
	}
}

// TestGateRejectsUnknownRoute covers the fail-closed path: a route mounted
// under the API prefix that the contract does not declare must be refused,
// not quietly downgraded to "any signed-in user".
func TestGateRejectsUnknownRoute(t *testing.T) {
	spec, err := GetSwagger()
	if err != nil {
		t.Fatal(err)
	}
	next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("handler ran for an operation the contract does not declare")
	})
	rctx := chi.NewRouteContext()
	rctx.RoutePatterns = []string{basePath + "/not-in-the-contract"}
	req := httptest.NewRequest(http.MethodGet, basePath+"/not-in-the-contract", nil).
		WithContext(context.WithValue(context.Background(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()
	gate(newRouteTable(spec), quietLog())(next).ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status %d, want 403", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Fatalf("content type %q", ct)
	}
}

// TestBlockedIPDoesNotGrowLimiter proves the per-email counter is skipped once
// the address is already blocked, so a spray cannot fill the limiter's map.
func TestBlockedIPDoesNotGrowLimiter(t *testing.T) {
	lim := auth.NewLimiter(1, time.Minute)
	if !lim.Allow("10.0.0.1") {
		t.Fatal("first attempt should be allowed")
	}
	before := lim.Len()
	h := &Handlers{d: Deps{Log: quietLog()}, logins: lim}
	for i := range 5 {
		body := fmt.Sprintf(`{"email":"user%d@example.com","password":"x"}`, i)
		req := httptest.NewRequest(http.MethodPost, basePath+"/auth/login", strings.NewReader(body))
		req.RemoteAddr = "10.0.0.1:34567"
		w := httptest.NewRecorder()
		h.AuthLogin(w, req)
		if w.Code != http.StatusTooManyRequests {
			t.Fatalf("attempt %d: status %d, want 429", i, w.Code)
		}
	}
	if got := lim.Len(); got != before {
		t.Fatalf("limiter keys grew from %d to %d while the address was blocked", before, got)
	}
}

// TestCheckCatchesAMissingRoute proves the self-check is not vacuous: drop one
// route from the table and it reports the operation it can no longer resolve.
func TestCheckCatchesAMissingRoute(t *testing.T) {
	spec, err := GetSwagger()
	if err != nil {
		t.Fatal(err)
	}
	table := newRouteTable(spec)
	delete(table.byRoute, http.MethodPost+" /auth/setup")
	err = table.check(spec)
	if err == nil {
		t.Fatal("check accepted a table missing POST /auth/setup")
	}
	if !strings.Contains(err.Error(), "/auth/setup") {
		t.Fatalf("error does not name the route: %v", err)
	}
}
