package api

import (
	"encoding/json"
	"net/http"
)

// writeProblem writes an RFC 9457 problem+json response using the Problem
// schema from the contract.
func writeProblem(w http.ResponseWriter, status int, title, detail string) {
	p := Problem{Type: "about:blank", Title: title, Status: status}
	if detail != "" {
		p.Detail = &detail
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(p)
}
