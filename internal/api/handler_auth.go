package api

import (
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/ayosage/cellarkeep-server/internal/auth"
	"github.com/ayosage/cellarkeep-server/internal/domain"
	"github.com/ayosage/cellarkeep-server/internal/store/gen"
)

// loginLimiter is process wide on purpose: one cellar, one process, and the
// window has to outlive a single request.
var loginLimiter = auth.NewLimiter(10, 15*time.Minute)

// clientIP drops the ephemeral port so the limiter counts a host, not a
// connection.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func userJSON(u gen.User) User {
	return User{Id: u.ID, Email: u.Email, Name: u.Name, Role: UserRole(u.Role), CreatedAt: u.CreatedAt}
}

func (h *Handlers) startSession(w http.ResponseWriter, r *http.Request, u gen.User) error {
	id, exp, err := h.sessions.Create(r.Context(), u.ID, time.Now())
	if err != nil {
		return err
	}
	auth.SetSessionCookie(w, id, exp, h.d.Cfg.InsecureCookies())
	return nil
}

func (h *Handlers) AuthStatus(w http.ResponseWriter, r *http.Request) {
	n, err := h.d.Store.Q.CountUsers(r.Context())
	if err != nil {
		h.writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"needsSetup": n == 0})
}

func (h *Handlers) AuthSetup(w http.ResponseWriter, r *http.Request) {
	var in SetupRequest
	if err := decode(r, &in); err != nil {
		h.writeErr(w, err)
		return
	}
	u, err := h.users.Setup(r.Context(), in.Email, in.Name, in.Password)
	if err != nil {
		h.writeErr(w, err)
		return
	}
	if err := h.startSession(w, r, u); err != nil {
		h.writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, userJSON(u))
}

func (h *Handlers) AuthLogin(w http.ResponseWriter, r *http.Request) {
	var in Credentials
	if err := decode(r, &in); err != nil {
		h.writeErr(w, err)
		return
	}
	ip := clientIP(r)
	// Both counters are consumed on every attempt, so a spray across many
	// addresses still trips the per-account limit.
	ipOK := loginLimiter.Allow(ip)
	emailOK := loginLimiter.Allow("email:" + strings.ToLower(strings.TrimSpace(in.Email)))
	if !ipOK || !emailOK {
		h.d.Log.Warn("login rate limited", "ip", ip)
		writeProblem(w, http.StatusTooManyRequests, "too many attempts", "try again in 15 minutes")
		return
	}
	u, err := h.users.Login(r.Context(), in.Email, in.Password)
	if err != nil {
		h.d.Log.Warn("login failed", "ip", ip)
		writeProblem(w, http.StatusForbidden, "invalid credentials", "")
		return
	}
	if err := h.startSession(w, r, u); err != nil {
		h.writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, userJSON(u))
}

func (h *Handlers) AuthLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(auth.CookieName); err == nil {
		if err := h.sessions.Delete(r.Context(), c.Value); err != nil {
			h.d.Log.Error("session delete", "err", err)
		}
	}
	auth.ClearSessionCookie(w, h.d.Cfg.InsecureCookies())
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handlers) AuthMe(w http.ResponseWriter, r *http.Request) {
	p, _ := auth.FromContext(r.Context())
	u, err := h.d.Store.Q.GetUser(r.Context(), p.UserID)
	if err != nil {
		h.writeErr(w, domain.ErrNotFound)
		return
	}
	writeJSON(w, http.StatusOK, userJSON(u))
}

func (h *Handlers) AuthRegister(w http.ResponseWriter, r *http.Request) {
	var in RegisterRequest
	if err := decode(r, &in); err != nil {
		h.writeErr(w, err)
		return
	}
	u, err := h.users.Register(r.Context(), in.InviteToken, in.Email, in.Name, in.Password, time.Now())
	if err != nil {
		h.writeErr(w, err)
		return
	}
	if err := h.startSession(w, r, u); err != nil {
		h.writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, userJSON(u))
}

func (h *Handlers) AuthChangePassword(w http.ResponseWriter, r *http.Request) {
	var in ChangePasswordRequest
	if err := decode(r, &in); err != nil {
		h.writeErr(w, err)
		return
	}
	p, _ := auth.FromContext(r.Context())
	if err := h.users.ChangePassword(r.Context(), p.UserID, in.CurrentPassword, in.NewPassword); err != nil {
		h.writeErr(w, err)
		return
	}
	// ChangePassword drops every session, this one included.
	auth.ClearSessionCookie(w, h.d.Cfg.InsecureCookies())
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handlers) ListMembers(w http.ResponseWriter, r *http.Request) {
	rows, err := h.d.Store.Q.ListUsers(r.Context())
	if err != nil {
		h.writeErr(w, err)
		return
	}
	out := make([]User, 0, len(rows))
	for _, u := range rows {
		out = append(out, User{Id: u.ID, Email: u.Email, Name: u.Name, Role: UserRole(u.Role), CreatedAt: u.CreatedAt})
	}
	writeJSON(w, http.StatusOK, out)
}
