package domain

import (
	"math"
	"sort"

	"github.com/google/uuid"
)

type Anchor string

const (
	AnchorStart    Anchor = "start"
	AnchorPrevious Anchor = "previous"
)

type StepInput struct {
	SortIndex  int
	Type       string
	Anchor     Anchor
	OffsetDays int
	Label      string
	Data       map[string]any
}

type StampedEvent struct {
	StepInput
	ScheduledDate Day
}

// StampSchedule resolves each step's date. START+n counts from the batch start;
// PREV+n counts from the previously stamped step, so a late step drags the rest.
func StampSchedule(steps []StepInput, start Day) []StampedEvent {
	ordered := append([]StepInput(nil), steps...)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].SortIndex < ordered[j].SortIndex })
	chain := start
	out := make([]StampedEvent, 0, len(ordered))
	for _, s := range ordered {
		var d Day
		if s.Anchor == AnchorStart {
			d = start.Add(s.OffsetDays)
		} else {
			d = chain.Add(s.OffsetDays)
		}
		chain = d
		out = append(out, StampedEvent{StepInput: s, ScheduledDate: d})
	}
	return out
}

func round3(n float64) float64 { return math.Round(n*1000) / 1000 }

func ScaleQty(qtyPerBase, baseVolumeL, actualVolumeL float64) float64 {
	if baseVolumeL <= 0 {
		return 0
	}
	return round3(qtyPerBase * actualVolumeL / baseVolumeL)
}

type ShiftableEvent struct {
	ID            uuid.UUID
	SortIndex     int
	Status        string
	Anchor        *Anchor
	OffsetDays    *int
	ScheduledDate *Day
	CompletedDate *Day
}

type DateChange struct {
	ID            uuid.UUID
	ScheduledDate Day
}

// ShiftAfterCompletion reflows downstream planned PREV-anchored events from the
// actual completion date. Freeform events (nil anchor) are never moved and do
// not affect the chain. A skipped event is never moved and does not affect the
// chain either. A done event does not move, but its actual date (CompletedDate,
// falling back to ScheduledDate) becomes the new chain so later PREV-anchored
// events reflow from what actually happened, not from the plan. A planned
// START-anchored event keeps its date but restarts the chain from it. A planned
// PREV-anchored event's date becomes chain + offset, reported as a change when
// it differs from its current ScheduledDate.
func ShiftAfterCompletion(events []ShiftableEvent, completedSortIndex int, completedDate Day) []DateChange {
	var downstream []ShiftableEvent
	for _, e := range events {
		if e.SortIndex > completedSortIndex {
			downstream = append(downstream, e)
		}
	}
	sort.SliceStable(downstream, func(i, j int) bool { return downstream[i].SortIndex < downstream[j].SortIndex })
	chain := completedDate
	changes := []DateChange{}
	for _, e := range downstream {
		if e.Anchor == nil {
			continue
		}
		switch e.Status {
		case "done":
			switch {
			case e.CompletedDate != nil:
				chain = *e.CompletedDate
			case e.ScheduledDate != nil:
				chain = *e.ScheduledDate
			}
		case "skipped":
			// chain unchanged
		case "planned":
			if *e.Anchor == AnchorStart {
				if e.ScheduledDate != nil {
					chain = *e.ScheduledDate
				}
				continue
			}
			off := 0
			if e.OffsetDays != nil {
				off = *e.OffsetDays
			}
			next := chain.Add(off)
			chain = next
			if e.ScheduledDate == nil || *e.ScheduledDate != next {
				changes = append(changes, DateChange{ID: e.ID, ScheduledDate: next})
			}
		}
	}
	return changes
}
