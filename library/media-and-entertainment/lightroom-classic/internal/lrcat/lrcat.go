// Copyright 2026 Micah Baldwin and contributors. Licensed under Apache-2.0. See LICENSE.
// Package lrcat provides strictly read-only access to Adobe Lightroom Classic
// catalogs (.lrcat SQLite databases). Every connection is opened with
// mode=ro plus PRAGMA query_only, so the catalog can never be modified.
package lrcat

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

// EnvCatalogPath overrides catalog discovery when set.
const EnvCatalogPath = "LIGHTROOM_CLASSIC_CATALOG"

// Catalog is a read-only handle on a Lightroom Classic catalog.
type Catalog struct {
	DB   *sql.DB
	Path string
}

// Resolve picks the catalog path: explicit flag value, then the
// LIGHTROOM_CLASSIC_CATALOG env var, then discovery of the most recently
// modified *.lrcat under ~/Pictures (two levels deep), skipping backups.
func Resolve(explicit string) (string, error) {
	if explicit != "" {
		return expand(explicit), nil
	}
	if v := os.Getenv(EnvCatalogPath); v != "" {
		return expand(v), nil
	}
	return discover()
}

func expand(p string) string {
	if strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, p[2:])
		}
	}
	return p
}

func discover() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolving home directory: %w", err)
	}
	roots := []string{
		filepath.Join(home, "Pictures", "Lightroom"),
		filepath.Join(home, "Pictures"),
	}
	var best string
	var bestMod int64
	for _, root := range roots {
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		for _, e := range entries {
			candidates := []string{}
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".lrcat") {
				candidates = append(candidates, filepath.Join(root, e.Name()))
			}
			if e.IsDir() && !strings.Contains(e.Name(), "Backup") {
				sub, err := os.ReadDir(filepath.Join(root, e.Name()))
				if err == nil {
					for _, s := range sub {
						if !s.IsDir() && strings.HasSuffix(s.Name(), ".lrcat") {
							candidates = append(candidates, filepath.Join(root, e.Name(), s.Name()))
						}
					}
				}
			}
			for _, c := range candidates {
				if strings.Contains(c, "Backup") {
					continue
				}
				info, err := os.Stat(c)
				if err != nil {
					continue
				}
				if info.ModTime().Unix() > bestMod {
					bestMod = info.ModTime().Unix()
					best = c
				}
			}
		}
	}
	if best == "" {
		return "", fmt.Errorf("no .lrcat catalog found under ~/Pictures; pass --catalog or set %s", EnvCatalogPath)
	}
	return best, nil
}

// Open opens the catalog strictly read-only. The connection uses SQLite
// mode=ro and PRAGMA query_only(1); any write attempt fails at the SQLite
// layer. WAL files left by a running Lightroom are honored (reads see the
// latest committed state).
func Open(ctx context.Context, path string) (*Catalog, error) {
	if path == "" {
		return nil, fmt.Errorf("catalog path is empty")
	}
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("catalog not found at %s; pass --catalog or set %s", path, EnvCatalogPath)
		}
		return nil, err
	}
	dsn := "file:" + url.PathEscape(path) +
		"?mode=ro&_pragma=query_only(1)&_pragma=busy_timeout(5000)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening catalog: %w", err)
	}
	// Single connection: the catalog is one file and queries are sequential.
	db.SetMaxOpenConns(1)
	var n int
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM sqlite_master WHERE name='Adobe_images'").Scan(&n); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("reading catalog: %w", err)
	}
	if n == 0 {
		_ = db.Close()
		return nil, fmt.Errorf("%s is not a Lightroom Classic catalog (no Adobe_images table)", path)
	}
	return &Catalog{DB: db, Path: path}, nil
}

// Close releases the underlying connection.
func (c *Catalog) Close() error {
	if c == nil || c.DB == nil {
		return nil
	}
	return c.DB.Close()
}
