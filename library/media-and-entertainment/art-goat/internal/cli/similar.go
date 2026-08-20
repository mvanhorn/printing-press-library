// Copyright 2026 justinwfu and contributors. Licensed under Apache-2.0.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/art-goat/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/art-goat/internal/store"

	"github.com/spf13/cobra"
)

func newSimilarCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var limit int

	cmd := &cobra.Command{
		Use:   "similar <work-id>",
		Short: "Find works similar to a given piece",
		Long: `Find works in the local corpus that resemble a seed work. Builds an
FTS5 query from the seed's medium, period, culture region, and creator
(OR-joined), then ranks across every source. If FTS5 returns nothing,
falls back to a structured match on medium, region, or canonical creator
so a thin corpus still produces relatives.

The seed work itself is excluded from results.`,
		Example: `  art-goat-pp-cli similar aic:24645
  art-goat-pp-cli similar met:436532 --limit 5
  art-goat-pp-cli similar harvard:204498 --json`,
		Args: cobra.ExactArgs(1),
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliutil.IsVerifyEnv() && !flags.asJSON {
				return emitSimilarVerifyEnvelope(cmd, flags)
			}
			if dbPath == "" {
				dbPath = defaultDBPath("art-goat-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()
			if err := db.EnsureArtGoatTables(cmd.Context()); err != nil {
				return err
			}

			seedID := args[0]
			seed, err := db.GetWork(cmd.Context(), seedID)
			if err != nil {
				return err
			}
			if seed == nil {
				return fmt.Errorf("work %q not found — sync first or check the id (e.g. aic:24645)", seedID)
			}

			query := buildSimilarFTSQuery(seed)
			var hits []store.Work
			if query != "" {
				rawHits, err := db.SearchWorks(cmd.Context(), query, limit+1)
				if err != nil {
					return err
				}
				hits = filterOutSeed(rawHits, seed.ID, limit)
			}

			// Fallback: structured similarity when FTS yielded zero matches.
			if len(hits) == 0 {
				fallback, err := db.WorksByStructuredSimilarity(
					cmd.Context(), seed.Medium, seed.CultureRegion, seed.CreatorCanonical, seed.ID, limit,
				)
				if err != nil {
					return err
				}
				hits = fallback
			}

			if flags.asJSON {
				envelopes := make([]map[string]any, 0, len(hits))
				for _, w := range hits {
					envelopes = append(envelopes, workToEnvelope(w))
				}
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"seed":    seed.ID,
					"query":   query,
					"count":   len(envelopes),
					"results": envelopes,
				}, flags)
			}

			renderSimilar(cmd, seed, hits)
			return nil
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum number of similar works to return")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/art-goat-pp-cli/data.db)")
	return cmd
}

// buildSimilarFTSQuery composes an FTS5 query from a seed work's
// medium, period, culture region, and creator. Empty fields are
// omitted. Terms are quoted to keep multi-word phrases intact and
// OR-joined so a recall-friendly match across any dimension wins.
func buildSimilarFTSQuery(seed *store.Work) string {
	terms := []string{}
	for _, raw := range []string{seed.Medium, seed.Period, seed.CultureRegion, seed.Creator} {
		t := strings.TrimSpace(raw)
		if t == "" {
			continue
		}
		// Strip embedded double quotes so FTS5 doesn't mis-parse the term.
		t = strings.ReplaceAll(t, `"`, "")
		if t == "" {
			continue
		}
		terms = append(terms, `"`+t+`"`)
	}
	return strings.Join(terms, " OR ")
}

func filterOutSeed(hits []store.Work, seedID string, limit int) []store.Work {
	out := make([]store.Work, 0, len(hits))
	for _, w := range hits {
		if w.ID == seedID {
			continue
		}
		out = append(out, w)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}

func renderSimilar(cmd *cobra.Command, seed *store.Work, hits []store.Work) {
	out := cmd.OutOrStdout()
	fmt.Fprintln(out, "")
	fmt.Fprintf(out, "Similar to: %s — %s\n", coalesce(seed.Title, "(untitled)"), coalesce(seed.Creator, "(unknown)"))
	fmt.Fprintf(out, "Seed:       %s\n", seed.ID)
	fmt.Fprintln(out, "")
	if len(hits) == 0 {
		fmt.Fprintln(out, "No similar works found in the local corpus. Try `sync` or widen your sources.")
		return
	}
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tSOURCE\tTITLE\tCREATOR\tDATE")
	for _, w := range hits {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
			w.ID, w.Source,
			truncate(coalesce(w.Title, "(untitled)"), 50),
			truncate(coalesce(w.Creator, "(unknown)"), 30),
			coalesce(w.DateText, ""),
		)
	}
	_ = tw.Flush()
	fmt.Fprintln(out, "")
}

// workToEnvelope renders a store.Work into the canonical JSON-envelope
// shape used by the cross-source novel commands (similar, compare,
// artist). Mirrors today.go's envelope keys so agent consumers can
// dispatch on the same field names regardless of which command emitted
// them.
func workToEnvelope(w store.Work) map[string]any {
	return map[string]any{
		"id":             w.ID,
		"source":         w.Source,
		"source_id":      w.SourceID,
		"title":          w.Title,
		"creator":        w.Creator,
		"date":           w.DateText,
		"medium":         w.Medium,
		"classification": w.Classification,
		"period":         w.Period,
		"region":         w.CultureRegion,
		"description":    w.Description,
		"image_url":      w.ImageURL,
		"thumbnail_url":  w.ThumbnailURL,
		"license":        w.License,
		"source_url":     w.SourceURL,
	}
}

func emitSimilarVerifyEnvelope(cmd *cobra.Command, flags *rootFlags) error {
	envelope := map[string]any{
		"command":                 "similar",
		"verify_noop":             true,
		"success":                 true,
		"__pp_verify_synthetic__": true,
		"reason":                  "verify_short_circuit",
		"note":                    "similar reads the local store; PRINTING_PRESS_VERIFY=1 short-circuits the table rendering. Pass --json to get the data envelope.",
	}
	if flags.asJSON {
		return printJSONFiltered(cmd.OutOrStdout(), envelope, flags)
	}
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(envelope)
}
