package domain

import (
	"reflect"
	"testing"

	"github.com/google/uuid"
)

func steps() []StepInput {
	return []StepInput{
		{SortIndex: 0, Type: "measurement", Anchor: AnchorStart, OffsetDays: 0, Label: "Pitch + OG"},
		{SortIndex: 1, Type: "addition", Anchor: AnchorStart, OffsetDays: 1, Label: "TOSNA 1/4"},
		{SortIndex: 2, Type: "racking", Anchor: AnchorStart, OffsetDays: 14, Label: "Rack"},
		{SortIndex: 3, Type: "stabilize", Anchor: AnchorPrevious, OffsetDays: 16, Label: "Stabilize"},
		{SortIndex: 4, Type: "bottling", Anchor: AnchorPrevious, OffsetDays: 49, Label: "Bottle"},
	}
}

func TestStampScheduleResolvesAnchors(t *testing.T) {
	out := StampSchedule(steps(), Day("2026-08-28"))
	var got []Day
	for _, e := range out {
		got = append(got, e.ScheduledDate)
	}
	want := []Day{"2026-08-28", "2026-08-29", "2026-09-11", "2026-09-27", "2026-11-15"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func ids(n int) []uuid.UUID {
	out := make([]uuid.UUID, n)
	for i := range out {
		out[i] = uuid.MustParse("00000000-0000-0000-0000-00000000000" + string(rune('0'+i)))
	}
	return out
}

func TestShiftAfterCompletionMovesPreviousChain(t *testing.T) {
	stamped := StampSchedule(steps(), Day("2026-08-28"))
	id := ids(5)
	var evs []ShiftableEvent
	for i, e := range stamped {
		a, o, d := e.Anchor, e.OffsetDays, e.ScheduledDate
		evs = append(evs, ShiftableEvent{ID: id[i], SortIndex: e.SortIndex, Status: "planned", Anchor: &a, OffsetDays: &o, ScheduledDate: &d})
	}
	got := ShiftAfterCompletion(evs, 2, Day("2026-09-14"))
	want := []DateChange{{ID: id[3], ScheduledDate: "2026-09-30"}, {ID: id[4], ScheduledDate: "2026-11-18"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestShiftSkipsDoneSkippedAndFreeform(t *testing.T) {
	id := ids(3)
	prev := AnchorPrevious
	five, two := 5, 2
	d1, d3 := Day("2026-09-01"), Day("2026-09-03")
	evs := []ShiftableEvent{
		{ID: id[0], SortIndex: 3, Status: "skipped", Anchor: &prev, OffsetDays: &five, ScheduledDate: &d1},
		{ID: id[1], SortIndex: 4, Status: "planned"},
		{ID: id[2], SortIndex: 5, Status: "planned", Anchor: &prev, OffsetDays: &two, ScheduledDate: &d3},
	}
	got := ShiftAfterCompletion(evs, 2, Day("2026-09-10"))
	want := []DateChange{{ID: id[2], ScheduledDate: "2026-09-12"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestShiftAfterCompletionUsesDoneEventActualDate(t *testing.T) {
	id := ids(2)
	prev := AnchorPrevious
	two := 2
	doneDate := Day("2026-09-20")
	plannedDate := Day("2026-09-05")
	evs := []ShiftableEvent{
		{ID: id[0], SortIndex: 3, Status: "done", Anchor: &prev, CompletedDate: &doneDate},
		{ID: id[1], SortIndex: 4, Status: "planned", Anchor: &prev, OffsetDays: &two, ScheduledDate: &plannedDate},
	}
	got := ShiftAfterCompletion(evs, 2, Day("2026-09-10"))
	want := []DateChange{{ID: id[1], ScheduledDate: "2026-09-22"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestScaleQty(t *testing.T) {
	if got := ScaleQty(5.4, 19, 11); got != 3.126 {
		t.Fatalf("got %v", got)
	}
	if got := ScaleQty(5, 0, 10); got != 0 {
		t.Fatalf("got %v want 0 for non-positive base volume", got)
	}
}

func TestDayAddAndParse(t *testing.T) {
	if Day("2026-02-28").Add(1) != "2026-03-01" {
		t.Fatal("leap handling")
	}
	if _, err := ParseDay("2026-13-01"); err == nil {
		t.Fatal("want error")
	}
}
