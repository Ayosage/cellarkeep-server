// Package api serves the CellarKeep HTTP contract. The route table, request
// and response types live in api/openapi.yaml and are generated into
// server.gen.go; the handlers here implement the generated ServerInterface.
package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/ayosage/cellarkeep-server/internal/auth"
	"github.com/ayosage/cellarkeep-server/internal/config"
	"github.com/ayosage/cellarkeep-server/internal/domain"
	"github.com/ayosage/cellarkeep-server/internal/store"
)

// maxBodyBytes caps every request body. A payload past it is a 413, not a
// malformed-JSON 422.
const maxBodyBytes = 1 << 20

// errBodyTooLarge reports a request body that hit maxBodyBytes.
var errBodyTooLarge = errors.New("request body is larger than 1 MB")

type Deps struct {
	DB    Pinger
	Store *store.Store
	Cfg   config.Config
	Log   *slog.Logger
}

// Handlers implements the generated ServerInterface.
type Handlers struct {
	d        Deps
	users    domain.Users
	sessions domain.Sessions
	invites  domain.Invites
	vessels  domain.Vessels
	inv      domain.Inventory
	// logins is per router, and the server builds one router, so in practice
	// it is one fixed window for the process.
	logins *auth.Limiter
}

func newHandlers(d Deps) *Handlers {
	if d.Log == nil {
		d.Log = slog.Default()
	}
	return &Handlers{
		d:        d,
		users:    domain.Users{S: d.Store},
		sessions: domain.Sessions{S: d.Store},
		invites:  domain.Invites{S: d.Store},
		vessels:  domain.Vessels{S: d.Store},
		inv:      domain.Inventory{S: d.Store},
		logins:   auth.NewLimiter(10, 15*time.Minute),
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// decode reads a JSON request body and refuses anything the contract does not
// declare, so a typo in a client payload fails loudly instead of being dropped.
func decode(r *http.Request, v any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return errBodyTooLarge
		}
		return &domain.ValidationError{Field: "body", Msg: err.Error()}
	}
	return nil
}

func (h *Handlers) writeErr(w http.ResponseWriter, err error) {
	var ve *domain.ValidationError
	switch {
	case errors.Is(err, errBodyTooLarge):
		writeProblem(w, http.StatusRequestEntityTooLarge, "request too large", err.Error())
	case errors.As(err, &ve):
		writeProblem(w, http.StatusUnprocessableEntity, "validation failed", ve.Error())
	case errors.Is(err, domain.ErrNotFound):
		writeProblem(w, http.StatusNotFound, "not found", "")
	case errors.Is(err, domain.ErrConflict):
		writeProblem(w, http.StatusConflict, "conflict", err.Error())
	case errors.Is(err, domain.ErrForbidden):
		writeProblem(w, http.StatusForbidden, "forbidden", "")
	default:
		h.d.Log.Error("unhandled", "err", err)
		writeProblem(w, http.StatusInternalServerError, "internal error", "")
	}
}

func dateOf(d domain.Day) openapi_types.Date { return openapi_types.Date{Time: d.Time()} }
