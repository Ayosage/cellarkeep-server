package domain

import (
	"testing"

	"github.com/ayosage/cellarkeep-server/internal/auth"
)

// The decoy hash stands in for a missing user so Login runs exactly one
// password verification either way. A hash that does not parse would make
// VerifyPassword return early with an error and reopen the timing gap.
func TestDecoyHashParses(t *testing.T) {
	ok, err := auth.VerifyPassword(decoyHash, "x")
	if err != nil {
		t.Fatalf("decoy hash does not parse: %v", err)
	}
	if ok {
		t.Fatal("decoy hash must not match an arbitrary password")
	}
}
