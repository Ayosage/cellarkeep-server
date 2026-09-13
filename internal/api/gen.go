package api

//go:generate go tool oapi-codegen -generate types,chi-server,spec -package api -o server.gen.go ../../api/openapi.yaml
