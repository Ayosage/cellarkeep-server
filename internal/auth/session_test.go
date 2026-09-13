package auth

import (
	"net/http/httptest"
	"testing"
	"time"
)

func TestSessionCookieFlags(t *testing.T) {
	w := httptest.NewRecorder()
	SetSessionCookie(w, "abc", time.Now().Add(time.Hour), false)
	c := w.Result().Cookies()[0]
	if c.Name != CookieName || !c.HttpOnly || !c.Secure || c.Path != "/" {
		t.Fatalf("bad cookie %+v", c)
	}
	id, err := NewSessionID()
	if err != nil || len(id) < 40 {
		t.Fatalf("id %q %v", id, err)
	}
}
