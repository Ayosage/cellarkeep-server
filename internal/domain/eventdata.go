package domain

import "fmt"

type EventType string

const (
	EventMeasurement EventType = "measurement"
	EventAddition    EventType = "addition"
	EventRacking     EventType = "racking"
	EventStabilize   EventType = "stabilize"
	EventSweeten     EventType = "sweeten"
	EventBottling    EventType = "bottling"
	EventNote        EventType = "note"
)

type fieldRule struct {
	kind     string // "string" | "number" | "int" | "day"
	required bool
	min, max *float64
}

func f(v float64) *float64 { return &v }

var eventRules = map[EventType]map[string]fieldRule{
	EventMeasurement: {"label": {kind: "string"}},
	EventAddition: {
		"label": {kind: "string"}, "ingredientId": {kind: "string", required: true},
		"qty": {kind: "number", required: true, min: f(0)}, "unit": {kind: "string", required: true},
	},
	EventRacking: {"label": {kind: "string"}},
	EventStabilize: {
		"label": {kind: "string"}, "kmetaG": {kind: "number", min: f(0)}, "sorbateG": {kind: "number", min: f(0)},
	},
	EventSweeten: {
		"label": {kind: "string"}, "targetFg": {kind: "number", min: f(0.9), max: f(1.2)},
		"ingredientId": {kind: "string"}, "qty": {kind: "number", min: f(0)},
	},
	EventBottling: {
		"label": {kind: "string"}, "count": {kind: "int", required: true, min: f(1)},
		"sizeMl": {kind: "int", required: true, min: f(1)}, "drinkFrom": {kind: "day"},
		"drinkTo": {kind: "day"}, "location": {kind: "string"},
	},
	EventNote: {"label": {kind: "string"}},
}

// ValidateEventData checks a payload against the rules for its event type and
// returns a copy with unknown keys removed. A min of 0 means strictly positive.
func ValidateEventData(t EventType, data map[string]any) (map[string]any, error) {
	rules, ok := eventRules[t]
	if !ok {
		return nil, invalid("type", fmt.Sprintf("unknown event type %q", t))
	}
	out := map[string]any{}
	for key, rule := range rules {
		v, present := data[key]
		if !present || v == nil {
			if rule.required {
				return nil, invalid(key, "is required")
			}
			continue
		}
		switch rule.kind {
		case "string":
			s, ok := v.(string)
			if !ok {
				return nil, invalid(key, "must be a string")
			}
			out[key] = s
		case "day":
			s, ok := v.(string)
			if !ok {
				return nil, invalid(key, "must be a YYYY-MM-DD string")
			}
			if _, err := ParseDay(s); err != nil {
				return nil, invalid(key, err.Error())
			}
			out[key] = s
		case "number", "int":
			n, ok := v.(float64)
			if !ok {
				return nil, invalid(key, "must be a number")
			}
			if rule.kind == "int" && n != float64(int64(n)) {
				return nil, invalid(key, "must be an integer")
			}
			if rule.min != nil && (n < *rule.min || (*rule.min == 0 && n == 0)) {
				return nil, invalid(key, "must be positive")
			}
			if rule.max != nil && n > *rule.max {
				return nil, invalid(key, fmt.Sprintf("must be at most %v", *rule.max))
			}
			out[key] = n
		}
	}
	return out, nil
}

type BottlingData struct {
	Count, SizeML      int
	DrinkFrom, DrinkTo *Day
	Location           *string
}

func ParseBottling(data map[string]any) (BottlingData, error) {
	clean, err := ValidateEventData(EventBottling, data)
	if err != nil {
		return BottlingData{}, err
	}
	b := BottlingData{Count: int(clean["count"].(float64)), SizeML: int(clean["sizeMl"].(float64))}
	if s, ok := clean["drinkFrom"].(string); ok {
		d := Day(s)
		b.DrinkFrom = &d
	}
	if s, ok := clean["drinkTo"].(string); ok {
		d := Day(s)
		b.DrinkTo = &d
	}
	if s, ok := clean["location"].(string); ok {
		b.Location = &s
	}
	return b, nil
}
