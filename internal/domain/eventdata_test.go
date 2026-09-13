package domain

import "testing"

func TestAdditionRequiresIngredientAndPositiveQty(t *testing.T) {
	_, err := ValidateEventData(EventAddition, map[string]any{"ingredientId": "x", "qty": -1.0, "unit": "g"})
	if err == nil {
		t.Fatal("want error")
	}
	out, err := ValidateEventData(EventAddition, map[string]any{"ingredientId": "x", "qty": 2.5, "unit": "g", "junk": 1})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := out["junk"]; ok {
		t.Fatal("unknown keys must be stripped")
	}
}

func TestBottlingRequiresCountAndSize(t *testing.T) {
	_, err := ValidateEventData(EventBottling, map[string]any{"count": 24.0})
	if err == nil {
		t.Fatal("want sizeMl error")
	}
	out, err := ValidateEventData(EventBottling, map[string]any{"count": 24.0, "sizeMl": 750.0, "drinkFrom": "2027-01-01"})
	if err != nil {
		t.Fatal(err)
	}
	b, err := ParseBottling(out)
	if err != nil || b.Count != 24 || b.SizeML != 750 || b.DrinkFrom == nil || *b.DrinkFrom != "2027-01-01" {
		t.Fatalf("got %+v %v", b, err)
	}
}

func TestMeasurementPayloadHoldsOnlyLabel(t *testing.T) {
	out, err := ValidateEventData(EventMeasurement, map[string]any{"label": "Day 3", "gravity": 1.05})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := out["gravity"]; ok {
		t.Fatal("values live in readings, not the event payload")
	}
}
