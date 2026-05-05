package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/hackernews/internal/store"
)

// newLocalSearchCmd performs an FTS5 search across the local SQLite
// store of synced stories and items. Distinct from `search` (Algolia
// live) and `live-search` (also Algolia). This one never hits the
// network and surfaces the corpus the user has already touched —
// including items past Algolia's effective recency.
func newSearchLocalCmd(flags *rootFlags) *cobra.Command {
	var limit int
	var resourceType string

	cmd := &cobra.Command{
		Use:   "local <query>",
		Short: "Full-text search across the local SQLite store of synced stories and comments",
		Long: `Search locally synced HN data with SQLite FTS5.

Live API search hits Algolia, which works great for relevance but loses
some long-tail history and requires a network round-trip. local-search
runs offline against everything you've synced. Pair with sync to grow
the searchable corpus over time.`,
		Example: strings.Trim(`
  # Plain query
  hackernews-pp-cli search local "rust async"

  # Limit results, scope to one resource type
  hackernews-pp-cli search local "openai" --limit 5 --type stories

  # JSON output, pipe-friendly
  hackernews-pp-cli search local "kubernetes" --json
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			query := strings.TrimSpace(args[0])
			if query == "" {
				return usageErr(fmt.Errorf("query is required and must be non-empty"))
			}
			db, err := store.Open(defaultDBPath("hackernews-pp-cli"))
			if err != nil {
				return apiErr(err)
			}
			defer db.Close()

			if limit <= 0 {
				limit = 20
			}
			// FTS5 treats hyphens, colons, and other punctuation as
			// operators. Wrap each whitespace-separated token in double
			// quotes so a casual query like "open-source" or "kubernetes:1.28"
			// is parsed as a phrase rather than a syntax error.
			results, err := db.Search(escapeFTS5Query(query), limit)
			if err != nil {
				return apiErr(err)
			}
			// Always emit a slice, never nil — agents iterating the
			// JSON envelope expect `[]` for "no matches", not `null`.
			if results == nil {
				results = []json.RawMessage{}
			}

			// Optional resource-type filter: if specified, walk results
			// and keep only matching rows. Cheap because the result set is
			// already capped at limit.
			out := results
			if resourceType != "" {
				out = []json.RawMessage{}
				for _, raw := range results {
					obj := map[string]any{}
					_ = json.Unmarshal(raw, &obj)
					if rt, _ := obj["_resource_type"].(string); strings.EqualFold(rt, resourceType) {
						out = append(out, raw)
					}
				}
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				j, _ := json.MarshalIndent(out, "", "  ")
				return printOutput(cmd.OutOrStdout(), j, true)
			}
			if len(out) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no local matches — sync more to grow the corpus")
				return nil
			}
			rows := make([][]string, 0, len(out))
			for _, raw := range out {
				obj := map[string]any{}
				_ = json.Unmarshal(raw, &obj)
				id := stringOrEmpty(obj["id"])
				title := stringOrEmpty(obj["title"])
				if title == "" {
					title = truncateAtRune(stringOrEmpty(obj["text"]), 60)
				}
				rt := stringOrEmpty(obj["_resource_type"])
				rows = append(rows, []string{id, rt, truncateAtRune(title, 70)})
			}
			return flags.printTable(cmd, []string{"ID", "TYPE", "TITLE/TEXT"}, rows)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum results to return")
	cmd.Flags().StringVar(&resourceType, "type", "", "Filter results to one resource type (stories, ask, show, jobs, users)")
	return cmd
}

// escapeFTS5Query wraps each whitespace-separated token in double quotes so
// FTS5 parses them as phrases. Internal double quotes are doubled per the
// FTS5 phrase-quoting rule. Empty input returns empty.
func escapeFTS5Query(q string) string {
	q = strings.TrimSpace(q)
	if q == "" {
		return ""
	}
	tokens := strings.Fields(q)
	for i, t := range tokens {
		tokens[i] = "\"" + strings.ReplaceAll(t, "\"", "\"\"") + "\""
	}
	return strings.Join(tokens, " ")
}

func stringOrEmpty(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	if f, ok := v.(float64); ok {
		return fmt.Sprintf("%v", int64(f))
	}
	return fmt.Sprintf("%v", v)
}
