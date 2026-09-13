package domain

import "testing"

func TestRegistryHasElevenMetrics(t *testing.T) {
	if n := len(Metrics()); n != 11 {
		t.Fatalf("got %d", n)
	}
}

func TestMetricValidateRange(t *testing.T) {
	m, ok := LookupMetric("ph")
	if !ok {
		t.Fatal("ph missing")
	}
	if err := m.Validate(3.4); err != nil {
		t.Fatal(err)
	}
	if err := m.Validate(7); err == nil {
		t.Fatal("want range error")
	}
	if _, ok := LookupMetric("colour"); ok {
		t.Fatal("unknown metric must not resolve")
	}
}
