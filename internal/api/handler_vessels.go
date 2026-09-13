package api

import (
	"net/http"

	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/ayosage/cellarkeep-server/internal/domain"
	"github.com/ayosage/cellarkeep-server/internal/store/gen"
)

func vesselJSON(v domain.VesselWithOccupant) Vessel {
	out := Vessel{
		Id: v.ID, Name: v.Name, Kind: VesselKind(v.Kind),
		CapacityL: v.CapacityL, Notes: v.Notes, CreatedAt: v.CreatedAt,
	}
	if v.Occupant != nil {
		o := v.Occupant
		out.Occupant = &Occupant{
			BatchId: o.BatchID, BatchName: o.BatchName, Beverage: Beverage(o.Beverage),
			VolumeL: float32(o.VolumeL), Since: dateOf(o.Since),
		}
	}
	return out
}

func (h *Handlers) ListVessels(w http.ResponseWriter, r *http.Request) {
	rows, err := h.vessels.List(r.Context())
	if err != nil {
		h.writeErr(w, err)
		return
	}
	out := make([]Vessel, 0, len(rows))
	for _, v := range rows {
		out = append(out, vesselJSON(v))
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handlers) CreateVessel(w http.ResponseWriter, r *http.Request) {
	var in VesselInput
	if err := decode(r, &in); err != nil {
		h.writeErr(w, err)
		return
	}
	v, err := h.vessels.Create(r.Context(), gen.CreateVesselParams{
		Name: in.Name, Kind: gen.VesselKind(in.Kind), CapacityL: in.CapacityL, Notes: in.Notes,
	})
	if err != nil {
		h.writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, vesselJSON(domain.VesselWithOccupant{Vessel: v}))
}

func (h *Handlers) UpdateVessel(w http.ResponseWriter, r *http.Request, id openapi_types.UUID) {
	var in VesselInput
	if err := decode(r, &in); err != nil {
		h.writeErr(w, err)
		return
	}
	v, err := h.vessels.Update(r.Context(), gen.UpdateVesselParams{
		ID: uuid.UUID(id), Name: in.Name, Kind: gen.VesselKind(in.Kind), CapacityL: in.CapacityL, Notes: in.Notes,
	})
	if err != nil {
		h.writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, vesselJSON(domain.VesselWithOccupant{Vessel: v}))
}

func (h *Handlers) DeleteVessel(w http.ResponseWriter, r *http.Request, id openapi_types.UUID) {
	if err := h.vessels.Delete(r.Context(), uuid.UUID(id)); err != nil {
		h.writeErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
