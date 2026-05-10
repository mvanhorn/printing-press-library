package store

import (
	"os"
	"path/filepath"
	"strings"
)

// PATCH: shared path resolver used by CLI and MCP so custom stores do not drift.
// DefaultPath returns the canonical SQLite store path for a generated CLI.
// POSTMAN_EXPLORE_DB is an explicit override for non-Cobra entry points such
// as MCP tools, where there is no root --db flag to inherit.
func DefaultPath(name string) string {
	if v := strings.TrimSpace(os.Getenv("POSTMAN_EXPLORE_DB")); v != "" {
		return v
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", name, "data.db")
}

// ResolvePath returns explicit when non-empty, otherwise the default path.
func ResolvePath(name, explicit string) string {
	if strings.TrimSpace(explicit) != "" {
		return explicit
	}
	return DefaultPath(name)
}
