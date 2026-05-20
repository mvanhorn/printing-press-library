// Copyright 2026 drdriftwood. Licensed under Apache-2.0. See LICENSE.

// `obsidian-pp-cli health` — composite vault-health score. Combines four
// independent sub-scores (each 0..1) into a single percentage so an
// agent can answer "is this vault healthy?" with one number, then drill
// in with the per-component breakdown.
//
// Sub-scores and their formulas:
//   connectivity = 1 - (orphans / notes)           lower orphan ratio is healthier
//   freshness    = 1 - (stale_90d / notes)         fewer notes >90d old is healthier
//   integrity    = 1 - (broken_links / links)      fewer unresolved wikilinks is healthier
//   consistency  = 1 - (frontmatter_drift / notes) common frontmatter shapes are healthier
//
// The composite is a simple unweighted average of the four sub-scores.
// V1 ships this opinionated weighting; --explain prints the formula and
// raw inputs so users can score their vault differently if they want.

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/productivity/obsidian/internal/store"
)

func newHealthCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var explain bool
	var staleDays int

	cmd := &cobra.Command{
		Use:   "health",
		Short: "Composite vault-health score (connectivity + freshness + integrity + consistency)",
		Long: `Score the vault on four axes (each 0..1, unweighted average):
  connectivity  = 1 - orphan ratio
  freshness     = 1 - share of notes not modified in N days (default 90)
  integrity     = 1 - share of broken wikilinks
  consistency   = 1 - frontmatter-drift ratio (notes whose frontmatter
                  schema diverges from the most common shape in their folder)

Returns a single 0..100 percentage plus the per-axis breakdown. Pass
--explain to print the raw inputs the score was derived from.`,
		Example: `  obsidian-pp-cli health
  obsidian-pp-cli health --explain
  obsidian-pp-cli health --json
  obsidian-pp-cli health --stale-days 30`,
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:typed-exit-codes": "0,5",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dbPath == "" {
				dbPath = defaultDBPath("obsidian-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()
			if err := db.EnsureObsidianSchema(); err != nil {
				return err
			}
			dbi := db.DB()

			notes := scalarInt(dbi, `SELECT COUNT(*) FROM notes`)
			if notes == 0 {
				return fmt.Errorf("mirror empty; run 'obsidian-pp-cli sync' first")
			}
			// Match the wikilink resolution used by `pm_orphans`,
			// `broken`, and `pm_stale` — exact path, basename with folder
			// prefix + `.md` stripped, OR frontmatter title — so all four
			// commands report coherent counts (a note linked only via
			// `[[Some Title]]` against its frontmatter `title:` is
			// resolved everywhere, not just in `broken`).
			orphans := scalarInt(dbi, `
				SELECT COUNT(*) FROM notes n
				WHERE NOT EXISTS (
					SELECT 1 FROM obsidian_links l
					WHERE l.target_path = n.path
					   OR l.target_path = replace(replace(n.path, rtrim(n.path, replace(n.path, '/', '')), ''), '.md', '')
					   OR l.target_path = n.title
				)`)
			// modified_at is stored as RFC3339 ("2026-05-21T12:00:00Z"), but
			// SQLite's datetime('now','-N days') returns "2026-05-21 12:00:00"
			// (space separator). Because ASCII 'T'(84) > ' '(32), a string
			// compare against the datetime() form would treat any note
			// modified on the exact cutoff date as newer-than-cutoff and miss
			// it, inflating the freshness sub-score. Compute the cutoff in Go
			// and bind it as the RFC3339 string the column actually holds —
			// same approach pm_stale.go takes.
			staleCutoff := time.Now().AddDate(0, 0, -staleDays).UTC().Format(time.RFC3339)
			staleN := scalarInt(dbi, `SELECT COUNT(*) FROM notes WHERE modified_at < ?`, staleCutoff)
			wikilinks := scalarInt(dbi, `SELECT COUNT(*) FROM obsidian_links WHERE link_type='wikilink'`)
			// Mirror the resolution rules used by `orphans` and `broken`
			// so the integrity sub-score doesn't get artificially deflated
			// by short-form wikilinks into nested notes (where the
			// target_path is just `foo` but the note lives at
			// `notes/foo.md`).
			brokenLinks := scalarInt(dbi, `
				SELECT COUNT(*) FROM obsidian_links l
				WHERE l.link_type='wikilink'
				  AND NOT EXISTS (
					SELECT 1 FROM notes n
					WHERE n.path = l.target_path
					   OR replace(n.path, '.md', '') = l.target_path
					   OR replace(replace(n.path, rtrim(n.path, replace(n.path, '/', '')), ''), '.md', '') = l.target_path
					   OR n.title = l.target_path
				  )`)
			// Frontmatter "drift" is approximated as the share of notes
			// WITH frontmatter that don't share the most-common (key-set)
			// schema. notesWithFM is the denominator so vaults where most
			// notes have no frontmatter aren't credited with a tall
			// consistency score for free. V1 computes this at the vault
			// level for simplicity; per-folder drift is what
			// `reconcile <dir>` reports.
			drift, notesWithFM := frontmatterDriftCount(dbi)

			connectivity := 1 - safeRatio(orphans, notes)
			freshness := 1 - safeRatio(staleN, notes)
			integrity := 1.0
			if wikilinks > 0 {
				integrity = 1 - safeRatio(brokenLinks, wikilinks)
			}
			consistency := 1.0
			if notesWithFM > 0 {
				consistency = 1 - safeRatio(drift, notesWithFM)
			}
			score := 100 * (connectivity + freshness + integrity + consistency) / 4

			out := map[string]any{
				"score":      round2(score),
				"components": map[string]float64{
					"connectivity": round4(connectivity),
					"freshness":    round4(freshness),
					"integrity":    round4(integrity),
					"consistency":  round4(consistency),
				},
			}
			if explain {
				out["explain"] = map[string]any{
					"formula": "score = 100 * (connectivity + freshness + integrity + consistency) / 4",
					"inputs": map[string]int{
						"notes":             notes,
						"orphans":           orphans,
						"stale_notes":       staleN,
						"wikilinks":         wikilinks,
						"broken_links":      brokenLinks,
						"drift_notes":       drift,
						"notes_with_frontmatter": notesWithFM,
						"stale_days":        staleDays,
					},
					"connectivity_formula": "1 - orphans/notes",
					"freshness_formula":    fmt.Sprintf("1 - stale_notes/notes (modified < %d days ago)", staleDays),
					"integrity_formula":    "1 - broken_links/wikilinks (or 1 if wikilinks=0)",
					"consistency_formula":  "1 - drift_notes/notes_with_frontmatter (or 1 if no notes carry frontmatter)",
				}
			}

			emitStalenessWarning(cmd, db)

			if flags.asJSON {
				b, _ := json.MarshalIndent(out, "", "  ")
				fmt.Fprintln(cmd.OutOrStdout(), string(b))
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Vault health: %.1f / 100\n\n", score)
			fmt.Fprintf(cmd.OutOrStdout(), "  connectivity: %.3f\n", connectivity)
			fmt.Fprintf(cmd.OutOrStdout(), "  freshness:    %.3f\n", freshness)
			fmt.Fprintf(cmd.OutOrStdout(), "  integrity:    %.3f\n", integrity)
			fmt.Fprintf(cmd.OutOrStdout(), "  consistency:  %.3f\n", consistency)
			if explain {
				fmt.Fprintf(cmd.OutOrStdout(),
					"\nInputs: notes=%d, orphans=%d, stale(%dd)=%d, wikilinks=%d, broken=%d, drift=%d (of %d w/ frontmatter)\n",
					notes, orphans, staleDays, staleN, wikilinks, brokenLinks, drift, notesWithFM)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/obsidian-pp-cli/data.db)")
	cmd.Flags().BoolVar(&explain, "explain", false, "Print the scoring formula and raw inputs")
	cmd.Flags().IntVar(&staleDays, "stale-days", 90, "Days without modification before a note counts as stale (for the freshness sub-score)")
	return cmd
}

func scalarInt(db *sql.DB, query string, args ...any) int {
	var n int
	_ = db.QueryRow(query, args...).Scan(&n)
	return n
}

func safeRatio(num, denom int) float64 {
	if denom <= 0 {
		return 0
	}
	return float64(num) / float64(denom)
}

func round2(v float64) float64 { return float64(int(v*100+0.5)) / 100 }
func round4(v float64) float64 { return float64(int(v*10000+0.5)) / 10000 }

// frontmatterDriftCount returns (drift, notesWithFrontmatter) — the
// number of notes whose frontmatter key-set differs from the
// most-common key-set across the vault, AND the count of notes that
// actually carry frontmatter. The caller divides drift by
// notesWithFrontmatter (not by total notes) so vaults where most notes
// have no frontmatter don't get an artificially-inflated consistency
// score. "No frontmatter" is a valid choice, not a drift.
func frontmatterDriftCount(db *sql.DB) (int, int) {
	rows, err := db.Query(`
		SELECT frontmatter_json FROM notes
		WHERE frontmatter_json IS NOT NULL AND frontmatter_json != '{}' AND frontmatter_json != ''
	`)
	if err != nil {
		return 0, 0
	}
	defer rows.Close()
	keySetCounts := map[string]int{}
	noteKeySets := []string{}
	for rows.Next() {
		var fm string
		if err := rows.Scan(&fm); err != nil {
			continue
		}
		var obj map[string]any
		if err := json.Unmarshal([]byte(fm), &obj); err != nil {
			continue
		}
		keys := make([]string, 0, len(obj))
		for k := range obj {
			keys = append(keys, k)
		}
		// Canonicalize key-set as comma-sorted string. Two notes drift
		// when their key-sets differ; key-order doesn't matter so sort.
		sortStrings(keys)
		sig := joinKeys(keys)
		keySetCounts[sig]++
		noteKeySets = append(noteKeySets, sig)
	}
	if err := rows.Err(); err != nil {
		return 0, 0
	}
	if len(noteKeySets) == 0 {
		return 0, 0
	}
	bestKey := ""
	bestN := -1
	for k, n := range keySetCounts {
		if n > bestN {
			bestN = n
			bestKey = k
		}
	}
	drift := 0
	for _, s := range noteKeySets {
		if s != bestKey {
			drift++
		}
	}
	return drift, len(noteKeySets)
}

// Local helpers (avoiding "sort"+"strings" imports here since this file
// has neither).
func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j-1] > s[j]; j-- {
			s[j-1], s[j] = s[j], s[j-1]
		}
	}
}

func joinKeys(s []string) string {
	out := ""
	for i, k := range s {
		if i > 0 {
			out += ","
		}
		out += k
	}
	return out
}
