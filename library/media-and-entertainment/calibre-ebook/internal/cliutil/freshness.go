package cliutil

import (
	"fmt"
	"os"
	"time"
)

func EnsureFresh(dbPath string, maxAge time.Duration) error {
	if dbPath == "" {
		return fmt.Errorf("no database path")
	}
	if maxAge <= 0 {
		maxAge = 6 * time.Hour
	}
	info, err := os.Stat(dbPath)
	if err != nil {
		return fmt.Errorf("cannot stat database: %w", err)
	}
	if time.Since(info.ModTime()) > maxAge {
		return fmt.Errorf("database is stale (last modified %s ago)", time.Since(info.ModTime()).Round(time.Minute))
	}
	return nil
}
