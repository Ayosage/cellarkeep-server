package domain

import "testing"

func TestABV(t *testing.T) {
	std, corr := ABV(1.102, 1.012)
	if std != 11.8 {
		t.Fatalf("standard %v", std)
	}
	if corr != 13.0 {
		t.Fatalf("corrected %v", corr)
	}
}
