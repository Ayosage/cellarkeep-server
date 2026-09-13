package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ayosage/cellarkeep-server/internal/api"
	"github.com/ayosage/cellarkeep-server/internal/config"
	"github.com/ayosage/cellarkeep-server/internal/store"
	"github.com/ayosage/cellarkeep-server/internal/store/testdb"
)

const origin = "http://localhost:3000"

// testServer drives one HTTP server over the shared test pool. The API runs
// on the pool rather than inside a test transaction, so each server truncates
// the tables it touches on the way in. Run database packages with -p 1.
type testServer struct {
	srv    *httptest.Server
	cookie *http.Cookie
	bearer string
}

// reqOpt adjusts one request before it is sent.
type reqOpt func(*http.Request)

// asDevice sends a bearer token instead of the stored session cookie.
func asDevice(token string) reqOpt {
	return func(r *http.Request) {
		r.Header.Set("Authorization", "Bearer "+token)
	}
}

// truncate empties every table the API writes to. The API runs on the pool,
// so its rows are committed; each test clears them on the way in and on the
// way out, leaving the shared database as it found it for the packages that
// run after this one.
func truncate(ctx context.Context, t *testing.T, s *store.Store) {
	t.Helper()
	if _, err := s.Pool.Exec(ctx, `truncate users, vessels, ingredients, recipes, batches cascade`); err != nil {
		t.Fatal(err)
	}
}

func newTestServer(t *testing.T) *testServer {
	t.Helper()
	s := testdb.Open(t)
	truncate(t.Context(), t, s)
	t.Cleanup(func() { truncate(context.Background(), t, s) })
	cfg := config.Config{
		DatabaseURL:   "unused",
		SessionSecret: strings.Repeat("x", 32),
		ClientOrigin:  origin,
		Addr:          ":0",
	}
	h := api.NewRouter(api.Deps{DB: s.Pool, Store: s, Cfg: cfg, Log: slog.Default()})
	ts := &testServer{srv: httptest.NewServer(h)}
	t.Cleanup(ts.srv.Close)
	return ts
}

// newTestServerSharingDB returns a second client against the same server with
// no session of its own, so a test can act as another user.
func newTestServerSharingDB(_ *testing.T, base *testServer) *testServer {
	return &testServer{srv: base.srv}
}

func (ts *testServer) do(t *testing.T, method, path string, body any, opts ...reqOpt) *http.Response {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatal(err)
		}
	}
	req, err := http.NewRequestWithContext(t.Context(), method, ts.srv.URL+"/api/v1"+path, &buf)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", origin)
	if ts.cookie != nil {
		req.AddCookie(ts.cookie)
	}
	if ts.bearer != "" {
		req.Header.Set("Authorization", "Bearer "+ts.bearer)
	}
	for _, o := range opts {
		o(req)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = res.Body.Close() })
	for _, c := range res.Cookies() {
		if c.Name != "ck_session" {
			continue
		}
		if c.MaxAge < 0 {
			ts.cookie = nil
		} else {
			ts.cookie = c
		}
	}
	return res
}

func decodeInto(t *testing.T, res *http.Response, v any) {
	t.Helper()
	if err := json.NewDecoder(res.Body).Decode(v); err != nil {
		t.Fatal(err)
	}
}

func setupAdmin(t *testing.T, ts *testServer) map[string]any {
	t.Helper()
	res := ts.do(t, "POST", "/auth/setup", map[string]string{
		"email": "brandon@example.com", "name": "Brandon", "password": "correct-horse-battery",
	})
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("setup: %d", res.StatusCode)
	}
	var u map[string]any
	decodeInto(t, res, &u)
	return u
}
