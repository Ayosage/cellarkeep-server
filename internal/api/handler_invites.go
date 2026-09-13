package api

import (
	"net/http"
	"time"

	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/ayosage/cellarkeep-server/internal/auth"
	"github.com/ayosage/cellarkeep-server/internal/store/gen"
)

func inviteJSON(i gen.Invite, createdByName string, usedByName *string) Invite {
	return Invite{
		Id: i.ID, Token: i.Token, Email: i.Email, CreatedBy: i.CreatedBy,
		CreatedByName: createdByName, CreatedAt: i.CreatedAt, ExpiresAt: i.ExpiresAt,
		UsedAt: i.UsedAt, UsedByName: usedByName, RevokedAt: i.RevokedAt,
	}
}

func (h *Handlers) ListInvites(w http.ResponseWriter, r *http.Request) {
	rows, err := h.invites.List(r.Context())
	if err != nil {
		h.writeErr(w, err)
		return
	}
	out := make([]Invite, 0, len(rows))
	for _, row := range rows {
		out = append(out, inviteJSON(gen.Invite{
			ID: row.ID, Token: row.Token, Email: row.Email, CreatedBy: row.CreatedBy,
			CreatedAt: row.CreatedAt, ExpiresAt: row.ExpiresAt, UsedAt: row.UsedAt,
			UsedBy: row.UsedBy, RevokedAt: row.RevokedAt,
		}, row.CreatedByName, row.UsedByName))
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handlers) CreateInvite(w http.ResponseWriter, r *http.Request) {
	var in InviteCreate
	// The body is optional: an invite with no address is a plain link.
	if r.ContentLength > 0 {
		if err := decode(r, &in); err != nil {
			h.writeErr(w, err)
			return
		}
	}
	p, _ := auth.FromContext(r.Context())
	inv, err := h.invites.Create(r.Context(), p.UserID, in.Email, time.Now())
	if err != nil {
		h.writeErr(w, err)
		return
	}
	u, err := h.d.Store.Q.GetUser(r.Context(), p.UserID)
	if err != nil {
		h.writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, inviteJSON(inv, u.Name, nil))
}

func (h *Handlers) RevokeInvite(w http.ResponseWriter, r *http.Request, id openapi_types.UUID) {
	inv, err := h.invites.Revoke(r.Context(), uuid.UUID(id), time.Now())
	if err != nil {
		h.writeErr(w, err)
		return
	}
	u, err := h.d.Store.Q.GetUser(r.Context(), inv.CreatedBy)
	if err != nil {
		h.writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, inviteJSON(inv, u.Name, nil))
}

func (h *Handlers) PreviewInvite(w http.ResponseWriter, r *http.Request, token string) {
	inv, err := h.invites.FindValid(r.Context(), token, time.Now())
	if err != nil {
		writeJSON(w, http.StatusOK, InvitePreview{Valid: false})
		return
	}
	writeJSON(w, http.StatusOK, InvitePreview{Valid: true, Email: inv.Email})
}
