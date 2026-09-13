// Package apispec embeds the authored OpenAPI contract. The server serves
// this file rather than the generator's embedded copy, which rewrites every
// operation id to its Go name.
package apispec

import _ "embed"

//go:embed openapi.yaml
var OpenAPI []byte
