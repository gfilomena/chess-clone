package api

import (
	"os"
	"strings"
)

// AdminConfig legge ADMIN_EMAILS (comma-separated) e offre IsAdmin(email).
// Viene istanziata una sola volta al boot in NewRouter().
//
// Esempio .env:
//
//	ADMIN_EMAILS=tuo@email.com,altro@email.com
type AdminConfig struct {
	emails map[string]struct{}
}

func NewAdminConfig() *AdminConfig {
	cfg := &AdminConfig{emails: make(map[string]struct{})}
	raw := os.Getenv("ADMIN_EMAILS")
	for _, e := range strings.Split(raw, ",") {
		e = strings.ToLower(strings.TrimSpace(e))
		if e != "" {
			cfg.emails[e] = struct{}{}
		}
	}
	return cfg
}

// IsAdmin riporta true se email è nella lista degli admin (case-insensitive).
func (c *AdminConfig) IsAdmin(email string) bool {
	_, ok := c.emails[strings.ToLower(email)]
	return ok
}

// Empty riporta true se ADMIN_EMAILS non è stato configurato.
func (c *AdminConfig) Empty() bool {
	return len(c.emails) == 0
}
