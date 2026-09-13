package auth

import (
	"testing"
	"time"
)

func TestLimiterBlocksAfterMax(t *testing.T) {
	l := NewLimiter(3, time.Minute)
	for i := 0; i < 3; i++ {
		if !l.Allow("k") {
			t.Fatalf("call %d should pass", i)
		}
	}
	if l.Allow("k") {
		t.Fatal("4th call should block")
	}
	if !l.Allow("other") {
		t.Fatal("other key unaffected")
	}
}
