package auth

import "testing"

func TestHashAndVerify(t *testing.T) {
	h, err := HashPassword("correct-horse-battery")
	if err != nil {
		t.Fatal(err)
	}
	if ok, _ := VerifyPassword(h, "correct-horse-battery"); !ok {
		t.Fatal("want match")
	}
	if ok, _ := VerifyPassword(h, "wrong"); ok {
		t.Fatal("want mismatch")
	}
	if _, err := VerifyPassword("garbage", "x"); err == nil {
		t.Fatal("want parse error")
	}
}
