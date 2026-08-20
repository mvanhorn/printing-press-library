// Copyright 2026 Maxime Delavergne and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/medium-reader/internal/store"
)

// authorStats is the per-author rollup computed from archived rows.
type authorStats struct {
	Author         string   `json:"author"`
	Archived       int      `json:"archived"`
	AvgClaps       float64  `json:"avg_claps"`
	AvgVoters      float64  `json:"avg_voters"`
	AvgResponses   float64  `json:"avg_responses"`
	AvgWordCount   float64  `json:"avg_word_count"`
	AvgReadingTime float64  `json:"avg_reading_time"`
	TopTags        []string `json:"top_tags"`
	Hint           string   `json:"hint,omitempty"`
	tagCounts      map[string]int
}

// pp:data-source local
// author-compare reads only from the local SQLite store (two writers'
// previously archived articles); it makes no live API calls. Populate via
// author-archive first.
func newNovelAuthorCompareCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "author-compare <a> <b>",
		Short: "Compare two writers on output cadence, topic mix, and engagement (claps, voters, responses, and reading time per article).",
		Example: strings.Trim(`
  medium-reader-pp-cli author-compare nickwignall the-medium-blog --agent
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				a, b := "<a>", "<b>"
				if len(args) > 0 {
					a = args[0]
				}
				if len(args) > 1 {
					b = args[1]
				}
				fmt.Fprintf(cmd.OutOrStdout(), "would compare archived stats for %s vs %s\n", a, b)
				return nil
			}
			if len(args) < 2 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("two usernames are required: <a> <b>"))
			}
			nameA, nameB := strings.TrimSpace(args[0]), strings.TrimSpace(args[1])

			if dbPath == "" {
				dbPath = defaultDBPath("medium-reader-pp-cli")
			}

			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				fmt.Fprintln(cmd.ErrOrStderr(), "no local mirror found; run 'medium-reader-pp-cli author-archive <username>' first")
				if flags.asJSON || flags.agent {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"a": nil, "b": nil}, flags)
				}
				return nil
			}

			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			rows, err := db.List("articles", 0)
			if err != nil {
				return fmt.Errorf("reading local store: %w", err)
			}

			statsA := computeAuthorStats(nameA, rows)
			statsB := computeAuthorStats(nameB, rows)

			view := map[string]any{"a": statsA, "b": statsB}
			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to the local SQLite store (default: standard data dir)")
	return cmd
}

// computeAuthorStats rolls up every archived row belonging to the named writer.
// When no rows match it returns a zero-count stat carrying a hint to run
// author-archive first — the command never live-fetches a whole catalog. The
// writer is matched by archiveKeyMatches, so a @handle / bare username joins the
// hex-keyed rows offline (see that helper).
func computeAuthorStats(username string, rows []json.RawMessage) authorStats {
	st := authorStats{Author: username, TopTags: make([]string, 0), tagCounts: map[string]int{}}
	var sumClaps, sumVoters, sumResponses, sumWords, sumReading float64
	for _, raw := range rows {
		var obj map[string]any
		if json.Unmarshal(raw, &obj) != nil {
			continue
		}
		if !archiveKeyMatches(obj, username) {
			continue
		}
		st.Archived++
		sumClaps += asFloat(obj["claps"])
		sumVoters += asFloat(obj["voters"])
		sumResponses += asFloat(obj["responses"])
		sumWords += asFloat(obj["word_count"])
		sumReading += asFloat(obj["reading_time"])
		for _, tag := range asStringSlice(obj["tags"]) {
			st.tagCounts[tag]++
		}
	}
	if st.Archived == 0 {
		st.Hint = fmt.Sprintf("no archived articles for %q; run 'medium-reader-pp-cli author-archive %s' first", username, username)
		return st
	}
	n := float64(st.Archived)
	st.AvgClaps = round2(sumClaps / n)
	st.AvgVoters = round2(sumVoters / n)
	st.AvgResponses = round2(sumResponses / n)
	st.AvgWordCount = round2(sumWords / n)
	st.AvgReadingTime = round2(sumReading / n)
	st.TopTags = topNTags(st.tagCounts, 5)
	return st
}

// normalizeWriterKey canonicalizes a writer identifier for offline matching: it
// strips a leading "@" and lowercases, so "@Quincylarson", "quincylarson", and a
// copied-with-uppercase hex id all reduce to one comparable key.
func normalizeWriterKey(s string) string {
	return strings.ToLower(strings.TrimPrefix(strings.TrimSpace(s), "@"))
}

// archiveKeyMatches reports whether an archived row belongs to the writer the
// user named on the command line. author-archive keys every row by the resolved
// hex archived_author AND the archived_handle the user typed, so a writer
// archived by @handle can be compared by @handle and one archived by hex id can
// be compared by hex id — all WITHOUT a network call, preserving author-compare's
// offline guarantee. It deliberately does NOT match the per-post "username": that
// is Medium's creator.username, which can be an opaque auto-generated string (so
// it would miss a handle compare) and, under includeDistributedResponses, may
// belong to a different writer (so it would mis-attribute foreign rows).
func archiveKeyMatches(obj map[string]any, arg string) bool {
	key := normalizeWriterKey(arg)
	if key == "" {
		return false
	}
	return normalizeWriterKey(asString(obj["archived_author"])) == key ||
		normalizeWriterKey(asString(obj["archived_handle"])) == key
}

// topNTags returns the n most frequent tags, breaking count ties alphabetically
// for deterministic output.
func topNTags(counts map[string]int, n int) []string {
	type tc struct {
		tag string
		n   int
	}
	all := make([]tc, 0, len(counts))
	for t, c := range counts {
		all = append(all, tc{t, c})
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].n != all[j].n {
			return all[i].n > all[j].n
		}
		return all[i].tag < all[j].tag
	})
	out := make([]string, 0, n)
	for i := 0; i < len(all) && i < n; i++ {
		out = append(out, all[i].tag)
	}
	return out
}

// asFloat coerces a decoded JSON value to float64; non-numeric values yield 0.
func asFloat(v any) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case int:
		return float64(t)
	default:
		return 0
	}
}

// asStringSlice coerces a decoded JSON array to a []string, dropping non-string
// elements.
func asStringSlice(v any) []string {
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, e := range arr {
		if s, ok := e.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// round2 rounds to two decimal places for stable, readable averages.
func round2(f float64) float64 {
	// math.Round rounds half away from zero for both signs (the truncating
	// int64(f*100+0.5) form rounded negatives toward zero). All engagement
	// metrics are non-negative today, so this only future-proofs derived metrics.
	return math.Round(f*100) / 100
}
