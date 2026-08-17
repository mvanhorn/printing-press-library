// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command. Implemented body; generate --force preserves this file.
// pp:data-source local

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/cloud/browserbase/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/cloud/browserbase/internal/store"
)

type webHistoryEntry struct {
	Type       string          `json:"type"`
	Key        string          `json:"key"`
	At         string          `json:"at"`
	Status     int             `json:"status_code,omitempty"`
	Summary    string          `json:"summary,omitempty"`
	CachedBody json.RawMessage `json:"cached_body,omitempty"`
}

type webHistoryView struct {
	Since string            `json:"since"`
	Type  string            `json:"type,omitempty"`
	Items []webHistoryEntry `json:"items"`
	Total int               `json:"total"`
}

func newNovelWebHistoryCmd(flags *rootFlags) *cobra.Command {
	var flagSince string
	var flagType string
	var flagReemit string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "history",
		Short: "Review past fetch and search calls with cached results, and re-emit a cached response without re-hitting the API.",
		Long: `Use this command to review past fetch and search activity and re-run cached results.
Do NOT use it to fetch a fresh list of URLs; use 'fetch batch' instead.`,
		Example:     "  browserbase-pp-cli web history --since 7d --type fetch --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "web history")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			since := 7 * 24 * time.Hour
			if flagSince != "" {
				parsed, err := cliutil.ParseDurationLoose(flagSince)
				if err != nil {
					_ = cmd.Usage()
					return usageErr(fmt.Errorf("--since %q is invalid: %w (use e.g. 7d, 24h, 1w)", flagSince, err))
				}
				since = parsed
			}
			if flagType != "" && flagType != "fetch" && flagType != "search" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--type %q is invalid; use fetch or search", flagType))
			}

			if dbPath == "" {
				dbPath = defaultDBPath("browserbase-pp-cli")
			}
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: browserbase-pp-cli sync --resources fetch,websearch --db %s\n", dbPath, dbPath)
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), webHistoryView{Since: flagSince, Type: flagType, Items: []webHistoryEntry{}}, flags)
				}
				return nil
			}
			db, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			if !hintIfUnsynced(cmd, db, "fetch") {
				hintIfStale(cmd, db, "fetch", flags.maxAge)
			}

			// Re-emit mode: print the cached body for one key without re-hitting the API.
			if flagReemit != "" {
				raw, err := db.Get("fetch", flagReemit)
				if err != nil || len(raw) == 0 {
					raw, err = db.Get("websearch", flagReemit)
				}
				if err != nil || len(raw) == 0 {
					if !wantsHumanTable(cmd.OutOrStdout(), flags) {
						return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"error": "not found in local cache", "key": flagReemit}, flags)
					}
					fmt.Fprintf(cmd.OutOrStdout(), "no cached entry for %q\n", flagReemit)
					return nil
				}
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), json.RawMessage(raw), flags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(raw))
				return nil
			}

			cutoff := time.Now().UTC().Add(-since)
			resources := []string{"fetch", "websearch"}
			if flagType == "fetch" {
				resources = []string{"fetch"}
			}
			if flagType == "search" {
				resources = []string{"websearch"}
			}

			items := make([]webHistoryEntry, 0)
			for _, res := range resources {
				var rows *sql.Rows
				var err error
				if res == "fetch" {
					// The typed fetch table carries synced_at + status_code; the
					// payload has no createdAt/url, so synced_at is the best
					// timestamp and content is the cached body.
					rows, err = db.DB().QueryContext(ctx, `
						SELECT id, synced_at, status_code, ''
						FROM fetch`)
				} else {
					rows, err = db.DB().QueryContext(ctx, `
						SELECT id, json_extract(data, '$.createdAt'), json_extract(data, '$.statusCode'),
						       json_extract(data, '$.url'), json_extract(data, '$.query')
						FROM resources
						WHERE resource_type = ?`, res)
				}
				if err != nil {
					return fmt.Errorf("querying %s history: %w", res, err)
				}
				type rawRow struct {
					id        string
					createdAt string
					status    string
					url       string
					query     string
				}
				rawRows := make([]rawRow, 0)
				for rows.Next() {
					var r rawRow
					var created, status, url, query sql.NullString
					// The fetch branch returns 4 columns (id, synced_at,
					// status_code, ''); the generic branch returns 5. Scan the
					// same shape either way by using a nullable url slot.
					if res == "fetch" {
						if err := rows.Scan(&r.id, &created, &status, &url); err != nil {
							_ = rows.Close()
							return fmt.Errorf("scanning %s row: %w", res, err)
						}
					} else {
						if err := rows.Scan(&r.id, &created, &status, &url, &query); err != nil {
							_ = rows.Close()
							return fmt.Errorf("scanning %s row: %w", res, err)
						}
					}
					r.createdAt = created.String
					r.status = status.String
					r.url = url.String
					r.query = query.String
					rawRows = append(rawRows, r)
				}
				if err := rows.Err(); err != nil {
					_ = rows.Close()
					return fmt.Errorf("iterating %s rows: %w", res, err)
				}
				if err := rows.Close(); err != nil {
					return fmt.Errorf("closing rows: %w", err)
				}

				for _, r := range rawRows {
					if r.createdAt == "" {
						continue
					}
					t := cliutil.ParseStoredTime(r.createdAt)
					if t.IsZero() || t.Before(cutoff) {
						continue
					}
					entry := webHistoryEntry{
						Type:    strings.TrimPrefix(res, "web"),
						Key:     r.id,
						At:      t.UTC().Format(time.RFC3339),
						Summary: r.url,
					}
					if entry.Summary == "" {
						entry.Summary = r.query
					}
					if entry.Summary == "" {
						entry.Summary = "fetch " + r.id
					}
					fmt.Sscanf(r.status, "%d", &entry.Status)
					// Load the cached body for re-emit capability.
					if raw, err := db.Get(res, r.id); err == nil && len(raw) > 0 {
						entry.CachedBody = raw
					}
					items = append(items, entry)
				}
			}

			sort.Slice(items, func(i, j int) bool { return items[i].At > items[j].At })

			view := webHistoryView{Since: flagSince, Type: flagType, Items: items, Total: len(items)}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			if len(items) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No web activity in the last %s.\n", since)
				return nil
			}
			for _, e := range items {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\t%d\n", e.At, e.Type, e.Summary, e.Status)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%d entries\n", len(items))
			return nil
		},
	}
	cmd.Flags().StringVar(&flagSince, "since", "7d", "Look back window (e.g. 7d, 24h, 1w)")
	cmd.Flags().StringVar(&flagType, "type", "", "Filter to fetch or search (default: both)")
	cmd.Flags().StringVar(&flagReemit, "reemit", "", "Re-emit the cached body for this entry key without re-hitting the API")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (defaults to the CLI data dir)")
	return cmd
}
