// Copyright 2026 Nate Olson and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: point-in-time model snapshotting and diffing.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/printgoat/internal/client"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/printgoat/internal/store"
	"github.com/spf13/cobra"
)

// historyFacts is the subset of a model's remote state that history diff
// tracks across runs. Kept intentionally small and stable so old snapshot
// rows (written by an older binary) still unmarshal cleanly.
type historyFacts struct {
	Name       string  `json:"name"`
	FilesCount int     `json:"files_count"`
	Likes      int     `json:"likes"`
	Downloads  int     `json:"downloads"`
	Rating     float64 `json:"rating"`
	License    string  `json:"license"`
}

func factsFromDetail(d *modelDetail) historyFacts {
	return historyFacts{
		Name:       d.Name,
		FilesCount: len(d.Files),
		Likes:      d.Likes,
		Downloads:  d.Downloads,
		Rating:     d.Rating,
		License:    d.License,
	}
}

type historyDiffEntry struct {
	Field string `json:"field"`
	Old   any    `json:"old"`
	New   any    `json:"new"`
}

func diffFacts(prev, cur historyFacts) []historyDiffEntry {
	var diffs []historyDiffEntry
	add := func(field string, old, new any) {
		diffs = append(diffs, historyDiffEntry{Field: field, Old: old, New: new})
	}
	if prev.Name != cur.Name {
		add("name", prev.Name, cur.Name)
	}
	if prev.FilesCount != cur.FilesCount {
		add("files_count", prev.FilesCount, cur.FilesCount)
	}
	if prev.Likes != cur.Likes {
		add("likes", prev.Likes, cur.Likes)
	}
	if prev.Downloads != cur.Downloads {
		add("downloads", prev.Downloads, cur.Downloads)
	}
	if prev.Rating != cur.Rating {
		add("rating", prev.Rating, cur.Rating)
	}
	if prev.License != cur.License {
		add("license", prev.License, cur.License)
	}
	return diffs
}

// diffOneModel fetches source, id's current state, compares it against the
// most recent prior snapshot (if any), records the fresh fetch as the new
// most-recent snapshot, and returns a JSON-shaped result describing what
// happened. A fetch failure is returned as an error (caller decides how to
// report); everything else (no prior snapshot, model gone, diff found or
// not) is folded into the returned map so a batch --all run keeps going
// past individual model problems.
func diffOneModel(ctx context.Context, c *client.Client, db *sql.DB, source, id string) (map[string]any, error) {
	detail, err := fetchModelDetail(ctx, c, source, id)
	if err != nil {
		return nil, err
	}
	result := map[string]any{"source": source, "model_id": id, "model_key": modelKey(source, id)}
	if !detail.Found {
		result["error"] = "model not found (delisted or deleted)"
		return result, nil
	}
	cur := factsFromDetail(detail)

	var prevJSON string
	scanErr := db.QueryRowContext(ctx,
		`SELECT snapshot_json FROM printgoat_model_snapshots WHERE source = ? AND model_id = ? ORDER BY snapshotted_at DESC, id DESC LIMIT 1`,
		source, id,
	).Scan(&prevJSON)
	switch {
	case scanErr == sql.ErrNoRows:
		result["baseline"] = true
		result["message"] = "no prior snapshot; recorded baseline"
	case scanErr != nil:
		return nil, fmt.Errorf("reading prior snapshot: %w", scanErr)
	default:
		var prev historyFacts
		if uerr := json.Unmarshal([]byte(prevJSON), &prev); uerr == nil {
			diffs := diffFacts(prev, cur)
			result["baseline"] = false
			result["changed"] = len(diffs) > 0
			result["differences"] = diffs
		} else {
			result["baseline"] = false
			result["error"] = "could not parse prior snapshot"
		}
	}

	curJSON, merr := marshalJSONNoEscape(cur)
	if merr != nil {
		return nil, fmt.Errorf("marshaling snapshot: %w", merr)
	}
	if _, ierr := db.ExecContext(ctx,
		`INSERT INTO printgoat_model_snapshots (source, model_id, snapshot_json, snapshotted_at) VALUES (?, ?, ?, ?)`,
		source, id, string(curJSON), time.Now().UTC().Format(time.RFC3339),
	); ierr != nil {
		return nil, fmt.Errorf("recording snapshot: %w", ierr)
	}
	result["current"] = cur
	return result, nil
}

// pp:data-source computed
func newNovelHistoryDiffCmd(flags *rootFlags) *cobra.Command {
	var flagAll bool

	cmd := &cobra.Command{
		Use:         "diff [model-key]",
		Short:       "Know exactly what changed on a model since you last looked — new files, price changes, license changes.",
		Example:     "  printgoat-pp-cli history diff printables:3161 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if !flagAll && len(args) == 0 {
				return usageErr(fmt.Errorf("history diff requires a <model-key> argument, or --all\nUsage: %s <model-key>|--all", cmd.CommandPath()))
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			dbPath := defaultDBPath("printgoat-pp-cli")
			s, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer s.Close()
			if err := store.EnsurePrintgoatNovelSchema(s.DB()); err != nil {
				return fmt.Errorf("preparing local schema: %w", err)
			}

			if flagAll {
				rows, qerr := s.DB().QueryContext(ctx, `SELECT DISTINCT source, model_id FROM printgoat_model_snapshots`)
				if qerr != nil {
					return fmt.Errorf("listing tracked models: %w", qerr)
				}
				var pairs [][2]string
				for rows.Next() {
					var source, id string
					if serr := rows.Scan(&source, &id); serr != nil {
						_ = rows.Close()
						return fmt.Errorf("scanning tracked models: %w", serr)
					}
					pairs = append(pairs, [2]string{source, id})
				}
				closeErr := rows.Close()
				if err := rows.Err(); err != nil {
					return fmt.Errorf("listing tracked models: %w", err)
				}
				if closeErr != nil {
					return fmt.Errorf("listing tracked models: %w", closeErr)
				}

				results := make([]map[string]any, 0, len(pairs))
				for _, p := range pairs {
					result, derr := diffOneModel(ctx, c, s.DB(), p[0], p[1])
					if derr != nil {
						result = map[string]any{"source": p[0], "model_id": p[1], "model_key": modelKey(p[0], p[1]), "error": derr.Error()}
					}
					results = append(results, result)
				}
				out := map[string]any{"mode": "all", "checked": len(results), "results": results}
				if len(results) == 0 {
					out["message"] = "no models have a prior snapshot yet; run 'history diff <model-key>' first"
				}
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}

			source, id, perr := parseModelRef(args[0])
			if perr != nil {
				return usageErr(perr)
			}
			result, derr := diffOneModel(ctx, c, s.DB(), source, id)
			if derr != nil {
				return classifyAPIError(derr, flags)
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().BoolVar(&flagAll, "all", false, "Diff every model that already has at least one prior snapshot, instead of a single model key")
	return cmd
}
