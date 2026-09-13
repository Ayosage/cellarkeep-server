package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"gopkg.in/yaml.v3"

	apispec "github.com/ayosage/cellarkeep-server/api"
	"github.com/ayosage/cellarkeep-server/internal/auth"
	"github.com/ayosage/cellarkeep-server/internal/domain"
)

// Pinger is satisfied by *pgxpool.Pool.
type Pinger interface {
	Ping(ctx context.Context) error
}

const basePath = "/api/v1"

// publicOps need no principal. deviceOps also accept a device token. adminOps
// need an admin session. Everything else needs any user session. Keys are
// operation ids from api/openapi.yaml; ids of routes later tasks add are
// listed here so the access level travels with the route, not the task. An
// operation the contract declares but none of these tables name gets the
// default, a user session; an operation the contract does not declare at all
// is refused outright.
var (
	publicOps = map[string]bool{
		"authSetup": true, "authLogin": true, "authRegister": true,
		"authStatus": true, "previewInvite": true,
	}
	deviceOps = map[string]bool{"listVessels": true, "postReadings": true, "listMetrics": true}
	adminOps  = map[string]bool{
		"listInvites": true, "createInvite": true, "revokeInvite": true,
		"listMembers": true, "listDevices": true, "createDevice": true, "revokeDevice": true,
	}
)

// routeTable turns the chi route pattern of a matched request back into the
// operation id the contract declares for it, so the gating tables above stay
// keyed by operation id and api/openapi.yaml remains the single source of
// truth for the route table.
type routeTable struct {
	byRoute map[string]string // "METHOD /path/{param}" without the base path
	known   map[string]bool   // every operation id the contract declares
}

func newRouteTable(spec *openapi3.T) routeTable {
	t := routeTable{byRoute: map[string]string{}, known: map[string]bool{}}
	if spec == nil || spec.Paths == nil {
		return t
	}
	for path, item := range spec.Paths.Map() {
		for method, op := range item.Operations() {
			if op.OperationID == "" {
				continue
			}
			id := lowerFirst(op.OperationID)
			t.byRoute[method+" "+path] = id
			t.known[id] = true
		}
	}
	return t
}

// resolve takes the request method and the chi route pattern exactly as the
// gate sees them at request time.
func (t routeTable) resolve(method, routePattern string) string {
	return t.byRoute[method+" "+strings.TrimPrefix(routePattern, basePath)]
}

// check walks every operation in the contract and confirms it resolves to its
// own id through the same lookup the gate uses. The generated router registers
// each route as basePath plus the contract path, which is what chi reports as
// the route pattern, so this exercises the real resolution, not a copy of it.
func (t routeTable) check(spec *openapi3.T) error {
	if spec == nil || spec.Paths == nil {
		return fmt.Errorf("openapi: no paths in the contract")
	}
	for path, item := range spec.Paths.Map() {
		for method, op := range item.Operations() {
			if op.OperationID == "" {
				return fmt.Errorf("openapi: %s %s declares no operationId", method, path)
			}
			want := lowerFirst(op.OperationID)
			if got := t.resolve(method, basePath+path); got != want {
				return fmt.Errorf("openapi: %s %s resolves to %q, want %q", method, path, got, want)
			}
			if !t.known[want] {
				return fmt.Errorf("openapi: %s %s operation %q is not in the operation set", method, path, want)
			}
		}
	}
	return nil
}

// lowerFirst undoes the capitalisation oapi-codegen applies when it embeds
// the spec: the generated copy carries the Go operation name (AuthSetup)
// where api/openapi.yaml declares the operation id (authSetup).
func lowerFirst(s string) string {
	r := []rune(s)
	if len(r) == 0 {
		return s
	}
	r[0] = unicode.ToLower(r[0])
	return string(r)
}

// contractJSON is the authored contract converted to JSON once per process.
var contractJSON = sync.OnceValues(func() ([]byte, error) {
	var doc any
	if err := yaml.Unmarshal(apispec.OpenAPI, &doc); err != nil {
		return nil, fmt.Errorf("openapi: parse contract: %w", err)
	}
	b, err := json.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("openapi: encode contract: %w", err)
	}
	return b, nil
})

// NewRouter panics when the contract cannot be loaded or a route in it cannot
// be resolved. Both mean the server would answer with the wrong access level,
// so it refuses to boot instead.
func NewRouter(d Deps) http.Handler {
	h := newHandlers(d)
	spec, err := GetSwagger()
	if err != nil {
		panic("api: cannot load the OpenAPI contract: " + err.Error())
	}
	table := newRouteTable(spec)
	if err := table.check(spec); err != nil {
		panic("api: " + err.Error())
	}

	r := chi.NewRouter()
	r.Use(middleware.RequestID, middleware.Recoverer, middleware.Timeout(30*time.Second))
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			req.Body = http.MaxBytesReader(w, req.Body, maxBodyBytes)
			next.ServeHTTP(w, req)
		})
	})
	r.Get("/healthz", func(w http.ResponseWriter, req *http.Request) {
		ctx, cancel := context.WithTimeout(req.Context(), 2*time.Second)
		defer cancel()
		if err := d.DB.Ping(ctx); err != nil {
			writeProblem(w, http.StatusServiceUnavailable, "database unreachable", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	r.Get(basePath+"/openapi.json", func(w http.ResponseWriter, _ *http.Request) {
		b, err := contractJSON()
		if err != nil {
			h.d.Log.Error("contract unavailable", "err", err)
			writeProblem(w, http.StatusInternalServerError, "spec unavailable", "")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(b)
	})

	authMw := auth.Middleware(
		auth.Config{ClientOrigin: d.Cfg.ClientOrigin, Insecure: d.Cfg.InsecureCookies()},
		h.sessions, domain.Devices{S: d.Store},
	)

	r.Group(func(r chi.Router) {
		r.Use(authMw)
		HandlerWithOptions(h, ChiServerOptions{
			BaseURL:     basePath,
			BaseRouter:  r,
			Middlewares: []MiddlewareFunc{gate(table, h.d.Log)},
			ErrorHandlerFunc: func(w http.ResponseWriter, _ *http.Request, err error) {
				writeProblem(w, http.StatusBadRequest, "invalid request", err.Error())
			},
		})
	})
	return r
}

// gate applies the auth requirement for the operation chi just matched. It
// runs inside the generated wrapper, which the router registers as the
// handler for each route, so the chi route pattern is already resolved. A
// pattern the contract does not declare is refused rather than defaulted,
// because the default would downgrade an admin route to any signed-in user.
func gate(t routeTable, log *slog.Logger) MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			pattern := chi.RouteContext(r.Context()).RoutePattern()
			op := t.resolve(r.Method, pattern)
			if op == "" || !t.known[op] {
				log.Error("unknown operation", "method", r.Method, "pattern", pattern)
				writeProblem(w, http.StatusForbidden, "unknown operation", "")
				return
			}
			switch {
			case publicOps[op]:
				next.ServeHTTP(w, r)
			case deviceOps[op]:
				if p, ok := auth.FromContext(r.Context()); ok && p.Kind == auth.PrincipalKindDevice {
					next.ServeHTTP(w, r)
					return
				}
				auth.RequireUser(next).ServeHTTP(w, r)
			case adminOps[op]:
				auth.RequireAdmin(next).ServeHTTP(w, r)
			default:
				auth.RequireUser(next).ServeHTTP(w, r)
			}
		})
	}
}
