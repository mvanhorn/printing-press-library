// Copyright 2026 Vikas and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored shared helpers for the Shodhganga novel commands. Not generated.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/shodhganga/internal/config"
	"github.com/mvanhorn/printing-press-library/library/productivity/shodhganga/internal/dspace"
	"github.com/mvanhorn/printing-press-library/library/productivity/shodhganga/internal/store"
)

// thesisResourceType is the store resource_type under which harvested theses are
// upserted; every store-backed novel command reads it.
const thesisResourceType = "thesis"

// newDSpaceClient builds the hand-authored Shodhganga HTML client using the same
// base URL and rate-limit settings the generated client would use.
func newDSpaceClient(flags *rootFlags) (*dspace.Client, error) {
	cfg, err := config.Load(flags.configPath)
	if err != nil {
		return nil, configErr(err)
	}
	return dspace.New(cfg.BaseURL, flags.rateLimit), nil
}

// openThesisStoreRead opens the local store for a read-only novel command,
// honoring the missing-mirror convention: when no DB exists yet it prints a hint
// naming the harvest command and returns ok=false so the caller emits an empty
// machine result and exits 0 (an empty local cache is not an error).
func openThesisStoreRead(cmd *cobra.Command, flags *rootFlags, dbPath string) (*store.Store, bool, error) {
	if dbPath == "" {
		dbPath = defaultDBPath("shodhganga-pp-cli")
	}
	if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
		fmt.Fprintf(cmd.ErrOrStderr(),
			"no local mirror at %s\nrun: shodhganga-pp-cli harvest <query> --db %s\n", dbPath, dbPath)
		if flags.asJSON || flags.agent {
			fmt.Fprintln(cmd.OutOrStdout(), "[]")
		}
		return nil, false, nil
	}
	// Read-only open: these commands (guide, similar, trends, university stats)
	// never write, so they must not take the SQLite write lock or run migrations —
	// otherwise a concurrent `harvest` would block them until it finishes.
	s, err := store.OpenReadOnlyContext(cmd.Context(), dbPath)
	if err != nil {
		return nil, false, fmt.Errorf("opening database: %w", err)
	}
	return s, true, nil
}

// openThesisStoreForWrite opens (creating if needed) the local store for the
// harvest command.
func openThesisStoreForWrite(cmd *cobra.Command, dbPath string) (*store.Store, error) {
	if dbPath == "" {
		dbPath = defaultDBPath("shodhganga-pp-cli")
	}
	s, err := store.OpenWithContext(cmd.Context(), dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}
	return s, nil
}

// loadTheses reads every stored thesis record into typed structs, skipping
// malformed rows.
func loadTheses(s *store.Store) ([]dspace.Thesis, error) {
	raws, err := s.List(thesisResourceType, 0)
	if err != nil {
		return nil, fmt.Errorf("listing theses: %w", err)
	}
	out := make([]dspace.Thesis, 0, len(raws))
	for _, raw := range raws {
		var t dspace.Thesis
		if err := json.Unmarshal(raw, &t); err != nil {
			continue
		}
		out = append(out, t)
	}
	return out, nil
}

// yearOf extracts a 4-digit leading year from a DC.date string ("2019",
// "2019-05-04T..."), or "" when none is present.
func yearOf(s string) string {
	s = strings.TrimSpace(s)
	if len(s) < 4 {
		return ""
	}
	y := s[:4]
	for _, r := range y {
		if r < '0' || r > '9' {
			return ""
		}
	}
	return y
}

// containsFold reports whether needle occurs in haystack, case-insensitively.
func containsFold(haystack, needle string) bool {
	return strings.Contains(strings.ToLower(haystack), strings.ToLower(strings.TrimSpace(needle)))
}
