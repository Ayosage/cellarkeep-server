package domain

import (
	"context"

	"github.com/google/uuid"

	"github.com/ayosage/cellarkeep-server/internal/store"
	"github.com/ayosage/cellarkeep-server/internal/store/gen"
)

// Occupant is the batch currently living in a vessel.
type Occupant struct {
	BatchID   uuid.UUID
	BatchName string
	Beverage  string
	VolumeL   float64
	Since     Day
}

type VesselWithOccupant struct {
	gen.Vessel
	Occupant *Occupant
}

type Vessels struct{ S *store.Store }

func (v Vessels) List(ctx context.Context) ([]VesselWithOccupant, error) {
	rows, err := v.S.Q.ListVesselsWithOccupant(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]VesselWithOccupant, 0, len(rows))
	for _, r := range rows {
		item := VesselWithOccupant{Vessel: gen.Vessel{
			ID: r.ID, Name: r.Name, Kind: r.Kind, CapacityL: r.CapacityL, Notes: r.Notes, CreatedAt: r.CreatedAt,
		}}
		if r.BatchID != nil {
			// The occupancy started when the batch was racked into this
			// vessel, or at the batch start date if it never moved.
			since := DayOf(*r.BatchStartDate)
			if r.RackedIn != nil {
				since = DayOf(*r.RackedIn)
			}
			item.Occupant = &Occupant{
				BatchID:   *r.BatchID,
				BatchName: *r.BatchName,
				Beverage:  string(r.BatchBeverage.BeverageType),
				VolumeL:   float64(*r.BatchVolumeL),
				Since:     since,
			}
		}
		out = append(out, item)
	}
	return out, nil
}

func (v Vessels) Create(ctx context.Context, p gen.CreateVesselParams) (gen.Vessel, error) {
	return v.S.Q.CreateVessel(ctx, p)
}

func (v Vessels) Update(ctx context.Context, p gen.UpdateVesselParams) (gen.Vessel, error) {
	row, err := v.S.Q.UpdateVessel(ctx, p)
	if err != nil {
		return gen.Vessel{}, ErrNotFound
	}
	return row, nil
}

// Delete refuses to remove a vessel that still holds an active batch.
func (v Vessels) Delete(ctx context.Context, id uuid.UUID) error {
	n, err := v.S.Q.CountActiveBatchesInVessel(ctx, &id)
	if err != nil {
		return err
	}
	if n > 0 {
		return ErrConflict
	}
	return v.S.Q.DeleteVessel(ctx, id)
}
