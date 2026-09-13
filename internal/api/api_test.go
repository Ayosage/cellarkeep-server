package api_test

import (
	"net/http"
	"strings"
	"testing"
)

func TestSetupLoginAndVessels(t *testing.T) {
	ts := newTestServer(t)
	if res := ts.do(t, "GET", "/vessels", nil); res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("anonymous list: %d", res.StatusCode)
	}
	setupAdmin(t, ts)
	if res := ts.do(t, "POST", "/auth/setup", map[string]string{
		"email": "x@x.com", "name": "X", "password": "passwordpassword",
	}); res.StatusCode != http.StatusConflict {
		t.Fatalf("second setup: %d", res.StatusCode)
	}
	res := ts.do(t, "POST", "/vessels", map[string]any{"name": "Bucket #1", "kind": "bucket", "capacityL": 23})
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("create vessel: %d", res.StatusCode)
	}
	var v map[string]any
	decodeInto(t, res, &v)
	if v["id"] == nil || v["capacityL"] != 23.0 {
		t.Fatalf("created vessel %+v", v)
	}
	res = ts.do(t, "GET", "/vessels", nil)
	var list []map[string]any
	decodeInto(t, res, &list)
	if len(list) != 1 || list[0]["name"] != "Bucket #1" || list[0]["occupant"] != nil {
		t.Fatalf("list %+v", list)
	}
	// An unknown device token is refused rather than falling back to the session.
	if res := ts.do(t, "GET", "/vessels", nil, asDevice("not-a-device")); res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("bogus device token: %d", res.StatusCode)
	}
	ts.do(t, "POST", "/auth/logout", nil)
	if res := ts.do(t, "GET", "/auth/me", nil); res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("after logout: %d", res.StatusCode)
	}
	res = ts.do(t, "POST", "/auth/login", map[string]string{
		"email": "brandon@example.com", "password": "correct-horse-battery",
	})
	if res.StatusCode != http.StatusOK {
		t.Fatalf("login: %d", res.StatusCode)
	}
	if res := ts.do(t, "GET", "/auth/me", nil); res.StatusCode != http.StatusOK {
		t.Fatalf("me after login: %d", res.StatusCode)
	}
}

func TestOriginRequiredOnWrites(t *testing.T) {
	ts := newTestServer(t)
	setupAdmin(t, ts)
	req, err := http.NewRequestWithContext(t.Context(), "POST", ts.srv.URL+"/api/v1/vessels",
		strings.NewReader(`{"name":"A","kind":"tank","capacityL":100}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(ts.cookie)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("no origin: %d", res.StatusCode)
	}
}

func TestInviteFlow(t *testing.T) {
	ts := newTestServer(t)
	setupAdmin(t, ts)
	res := ts.do(t, "POST", "/invites", nil)
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("create invite: %d", res.StatusCode)
	}
	var inv map[string]any
	decodeInto(t, res, &inv)
	token, ok := inv["token"].(string)
	if !ok || token == "" {
		t.Fatalf("invite %+v", inv)
	}
	if inv["createdByName"] != "Brandon" {
		t.Fatalf("invite creator %+v", inv)
	}
	member := newTestServerSharingDB(t, ts)
	res = member.do(t, "GET", "/invites/preview/"+token, nil)
	var prev map[string]any
	decodeInto(t, res, &prev)
	if prev["valid"] != true {
		t.Fatalf("preview should be valid: %+v", prev)
	}
	res = member.do(t, "POST", "/auth/register", map[string]string{
		"inviteToken": token, "email": "m@x.com", "name": "M", "password": "passwordpassword",
	})
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("register: %d", res.StatusCode)
	}
	if res := member.do(t, "GET", "/invites", nil); res.StatusCode != http.StatusForbidden {
		t.Fatalf("member listing invites: %d", res.StatusCode)
	}
	if res := ts.do(t, "GET", "/invites", nil); res.StatusCode != http.StatusOK {
		t.Fatalf("admin listing invites: %d", res.StatusCode)
	}
	// The token is spent, so a second preview reports it invalid.
	res = member.do(t, "GET", "/invites/preview/"+token, nil)
	decodeInto(t, res, &prev)
	if prev["valid"] != false {
		t.Fatalf("preview after use: %+v", prev)
	}
}
