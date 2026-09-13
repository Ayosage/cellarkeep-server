package api

import (
	"context"
	"net/http"
	"strings"
	"time"
	"unicode"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

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
// listed here so the access level travels with the route, not the task.
var (
	publicOps = map[string]bool{
		"authSetup": true, "authLogin": true, "authRegister": true,
		"authStatus": true, "previewInvite": true,
	}
	deviceOps = map[string]bool{"listVessels": true, "postReadings": true}
	adminOps  = map[string]bool{
		"listInvites": true, "createInvite": true, "revokeInvite": true,
		"listMembers": true, "listDevices": true, "createDevice": true, "revokeDevice": true,
	}
)

func NewRouter(d Deps) http.Handler {
	h := newHandlers(d)
	spec, specErr := GetSwagger()
	r := chi.NewRouter()
	r.Use(middleware.RequestID, middleware.Recoverer, middleware.Timeout(30*time.Second))
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			req.Body = http.MaxBytesReader(w, req.Body, 1<<20)
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
		if specErr != nil {
			writeProblem(w, http.StatusInternalServerError, "spec unavailable", specErr.Error())
			return
		}
		writeJSON(w, http.StatusOK, spec)
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
			Middlewares: []MiddlewareFunc{gate(operationIDs(spec))},
		})
	})
	return r
}

// operationIDs maps "METHOD /path/{param}" to the operation id the contract
// declares for it, so the gating table below stays keyed by operation id and
// api/openapi.yaml remains the single source of truth for the route table.
func operationIDs(spec *openapi3.T) map[string]string {
	out := map[string]string{}
	if spec == nil || spec.Paths == nil {
		return out
	}
	for path, item := range spec.Paths.Map() {
		for method, op := range item.Operations() {
			if op.OperationID != "" {
				out[method+" "+path] = lowerFirst(op.OperationID)
			}
		}
	}
	return out
}

// lowerFirst undoes the capitalisation oapi-codegen applies when it embeds
// the spec: the generated copy carries the Go operation name (AuthSetup)
// where api/openapi.yaml declares the operation id (authSetup). Keying the
// tables above by the contract's spelling keeps the YAML readable as the
// source of truth.
func lowerFirst(s string) string {
	r := []rune(s)
	if len(r) == 0 {
		return s
	}
	r[0] = unicode.ToLower(r[0])
	return string(r)
}

// gate applies the auth requirement for the operation chi just matched. It
// runs inside the generated wrapper, which the router registers as the
// handler for each route, so the chi route pattern is already resolved.
// An operation id it cannot resolve falls through to requiring a user
// session, which fails closed.
func gate(ops map[string]string) MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			pattern := strings.TrimPrefix(chi.RouteContext(r.Context()).RoutePattern(), basePath)
			op := ops[r.Method+" "+pattern]
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
