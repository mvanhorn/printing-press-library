// Copyright 2026 justinwfu. Licensed under Apache-2.0.

// path --theme — multi-piece curated walk across the federated corpus.
// FTS5 search builds the candidate pool; diversity ordering produces the
// final walk so consecutive steps don't repeat source/region/medium when
// the corpus allows.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/art-goat/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/art-goat/internal/store"

	"github.com/spf13/cobra"
)

func newPathCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var theme string
	var steps int

	cmd := &cobra.Command{
		Use:   "path",
		Short: "Walk a theme across the federated corpus",
		Long: `Compose a multi-piece curated walk over a theme. Runs the theme as
an FTS5 search across title, creator, medium, period, and description,
then orders the result for diversity — consecutive steps avoid sharing
the same source, region, and medium when the corpus allows.

Print only. The walk is not persisted to the journal; use 'sit <id>'
after picking a step if you want to commit to one.`,
		Example: `  art-goat-pp-cli path --theme "impermanence"
  art-goat-pp-cli path --theme "solitude" --steps 7
  art-goat-pp-cli path --theme "the moon" --json --select theme,steps`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliutil.IsVerifyEnv() && !flags.asJSON {
				return emitPathVerifyEnvelope(cmd, flags)
			}
			if strings.TrimSpace(theme) == "" {
				return usageErr(fmt.Errorf("--theme is required (e.g. --theme \"impermanence\")"))
			}
			if steps <= 0 {
				return usageErr(fmt.Errorf("--steps must be a positive integer (got %d)", steps))
			}
			if steps > 25 {
				return usageErr(fmt.Errorf("--steps capped at 25 (got %d) to keep walks readable", steps))
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

			query := buildThemeFTSQuery(theme)
			// Over-fetch so diversityOrder has room to skip near-duplicates.
			// 5x the requested step count caps practical pool size at 125
			// for the max steps=25, which the FTS5 ORDER BY rank query
			// handles in milliseconds against a single-digit-thousand row
			// corpus.
			pool, err := db.SearchWorks(cmd.Context(), query, steps*5)
			if err != nil {
				return err
			}

			walk := diversityOrderedWalk(pool, steps)

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), pathEnvelope(theme, steps, walk), flags)
			}
			renderPath(cmd, theme, walk)
			return nil
		},
	}

	cmd.Flags().StringVar(&theme, "theme", "", "Theme to walk (FTS5 query across title/creator/medium/period/description)")
	cmd.Flags().IntVar(&steps, "steps", 5, "Number of steps in the walk (1-25)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/art-goat-pp-cli/data.db)")
	return cmd
}

// buildThemeFTSQuery turns a user theme into an FTS5 query string. The
// theme is tokenized on whitespace; each token is wrapped in quotes and
// joined with FTS5's implicit AND so multi-word themes like "the moon"
// match works mentioning both tokens. Embedded double quotes are stripped
// to keep FTS5 parsing happy; single tokens shorter than 2 chars are
// dropped to keep noise out of the rank.
//
// AND-joining (vs OR) is intentional: a `path --theme "solitude"` walk
// over OR-joined tokens would surface every work that mentions any
// theme word, which dilutes the practice. AND-joining ranks tighter
// matches first and lets users widen by typing more words.
func buildThemeFTSQuery(theme string) string {
	tokens := strings.Fields(strings.ToLower(strings.TrimSpace(theme)))
	terms := make([]string, 0, len(tokens))
	for _, t := range tokens {
		t = strings.TrimFunc(t, func(r rune) bool {
			return r == '"' || r == '*' || r == '(' || r == ')'
		})
		if len(t) < 2 {
			continue
		}
		terms = append(terms, `"`+t+`"`)
	}
	if len(terms) == 0 {
		// Fallback: return the raw theme; SearchWorks treats empty/
		// whitespace as "recent works", which is the right behavior
		// when the user typed a single noise character.
		return strings.TrimSpace(theme)
	}
	return strings.Join(terms, " AND ")
}

// diversityOrderedWalk picks up to `steps` works from the pool, ordered
// so that no two consecutive steps share the same source, region, or
// medium when the corpus allows. The first step is always the highest-
// ranked match (pool[0]); subsequent steps are chosen greedily by
// scoring remaining candidates against the cumulative fingerprint.
//
// When the pool is smaller than `steps`, the walk is truncated to the
// pool size — better to return a shorter, real walk than to repeat or
// pad with weak matches.
func diversityOrderedWalk(pool []store.Work, steps int) []store.Work {
	if len(pool) == 0 || steps <= 0 {
		return nil
	}
	if steps > len(pool) {
		steps = len(pool)
	}
	walk := make([]store.Work, 0, steps)
	used := make(map[string]bool, len(pool))
	// Cumulative fingerprint of source/region/medium chosen so far.
	sources := map[string]bool{}
	regions := map[string]bool{}
	mediums := map[string]bool{}

	// First step: take the highest-ranked match (FTS5 rank).
	first := pool[0]
	walk = append(walk, first)
	used[first.ID] = true
	sources[first.Source] = true
	regions[strings.ToLower(first.CultureRegion)] = true
	mediums[strings.ToLower(first.Medium)] = true

	for len(walk) < steps {
		bestIdx := -1
		bestScore := -1
		// Iterate pool in rank order; ties are broken in favor of
		// earlier (higher-ranked) candidates so the walk respects FTS
		// relevance as a secondary signal.
		for i, w := range pool {
			if used[w.ID] {
				continue
			}
			score := diversityStepScore(w, sources, regions, mediums)
			if score > bestScore {
				bestScore = score
				bestIdx = i
			}
			if score == 3 {
				break // unbeatable
			}
		}
		if bestIdx < 0 {
			break // pool exhausted
		}
		next := pool[bestIdx]
		walk = append(walk, next)
		used[next.ID] = true
		sources[next.Source] = true
		regions[strings.ToLower(next.CultureRegion)] = true
		mediums[strings.ToLower(next.Medium)] = true
	}
	return walk
}

func diversityStepScore(w store.Work, sources, regions, mediums map[string]bool) int {
	score := 0
	if !sources[w.Source] {
		score++
	}
	if !regions[strings.ToLower(w.CultureRegion)] {
		score++
	}
	if !mediums[strings.ToLower(w.Medium)] {
		score++
	}
	return score
}

func pathEnvelope(theme string, requested int, walk []store.Work) map[string]any {
	steps := make([]map[string]any, 0, len(walk))
	for _, w := range walk {
		steps = append(steps, workToEnvelope(w))
	}
	// distinct sources/regions/mediums in the walk — gives agents a
	// quick read on how broad the resulting walk is without re-deriving.
	srcs, regs, meds := walkSummary(walk)
	return map[string]any{
		"theme":            theme,
		"requested_steps":  requested,
		"actual_steps":     len(walk),
		"distinct_sources": srcs,
		"distinct_regions": regs,
		"distinct_mediums": meds,
		"steps":            steps,
	}
}

func walkSummary(walk []store.Work) (sources, regions, mediums []string) {
	src := map[string]bool{}
	reg := map[string]bool{}
	med := map[string]bool{}
	for _, w := range walk {
		if w.Source != "" {
			src[w.Source] = true
		}
		if w.CultureRegion != "" {
			reg[w.CultureRegion] = true
		}
		if w.Medium != "" {
			med[w.Medium] = true
		}
	}
	for s := range src {
		sources = append(sources, s)
	}
	for r := range reg {
		regions = append(regions, r)
	}
	for m := range med {
		mediums = append(mediums, m)
	}
	sort.Strings(sources)
	sort.Strings(regions)
	sort.Strings(mediums)
	return sources, regions, mediums
}

func renderPath(cmd *cobra.Command, theme string, walk []store.Work) {
	out := cmd.OutOrStdout()
	fmt.Fprintln(out, "")
	fmt.Fprintf(out, "Walk: %s (%d steps)\n", theme, len(walk))
	if len(walk) == 0 {
		fmt.Fprintln(out, "")
		fmt.Fprintln(out, "No matches in the local corpus. Try `sync` first, or broaden the theme.")
		fmt.Fprintln(out, "")
		return
	}
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "  #\tCREATOR\tTITLE\tID\tMEDIUM\tREGION\tDATE")
	for i, w := range walk {
		fmt.Fprintf(tw, "  %d.\t%s\t%s\t%s\t%s\t%s\t%s\n",
			i+1,
			truncate(coalesce(w.Creator, "(unknown)"), 24),
			truncate(coalesce(w.Title, "(untitled)"), 40),
			w.ID,
			truncate(coalesce(w.Medium, ""), 16),
			truncate(coalesce(w.CultureRegion, ""), 14),
			coalesce(w.DateText, ""),
		)
	}
	_ = tw.Flush()
	fmt.Fprintln(out, "")
	fmt.Fprintf(out, "Start any step: art-goat-pp-cli sit %s\n", walk[0].ID)
	fmt.Fprintln(out, "")
}

func emitPathVerifyEnvelope(cmd *cobra.Command, flags *rootFlags) error {
	envelope := map[string]any{
		"command":                 "path",
		"verify_noop":             true,
		"success":                 true,
		"__pp_verify_synthetic__": true,
		"reason":                  "verify_short_circuit",
		"note":                    "path reads the local store; PRINTING_PRESS_VERIFY=1 short-circuits the rendering. Pass --json to get the data envelope.",
	}
	if flags.asJSON {
		return printJSONFiltered(cmd.OutOrStdout(), envelope, flags)
	}
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(envelope)
}
