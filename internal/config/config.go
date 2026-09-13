// Package config loads server settings from the environment. Missing or
// invalid values fail at boot with the variable named in the error.
package config

import (
	"errors"
	"fmt"
	"strings"
)

type Config struct {
	DatabaseURL   string
	SessionSecret string
	ClientOrigin  string
	Addr          string
}

func Load(getenv func(string) string) (Config, error) {
	c := Config{
		DatabaseURL:   getenv("DATABASE_URL"),
		SessionSecret: getenv("SESSION_SECRET"),
		ClientOrigin:  strings.TrimRight(getenv("CLIENT_ORIGIN"), "/"),
		Addr:          getenv("ADDR"),
	}
	var errs []error
	if c.DatabaseURL == "" {
		errs = append(errs, errors.New("DATABASE_URL is required"))
	}
	if len(c.SessionSecret) < 32 {
		errs = append(errs, errors.New("SESSION_SECRET must be at least 32 characters"))
	}
	if c.ClientOrigin == "" {
		errs = append(errs, errors.New("CLIENT_ORIGIN is required"))
	}
	if c.Addr == "" {
		c.Addr = ":8080"
	}
	if len(errs) > 0 {
		return Config{}, fmt.Errorf("config: %w", errors.Join(errs...))
	}
	return c, nil
}

// InsecureCookies is true only for local http development.
func (c Config) InsecureCookies() bool {
	return strings.HasPrefix(c.ClientOrigin, "http://localhost")
}
