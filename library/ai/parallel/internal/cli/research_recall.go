// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/ai/parallel/internal/store"
	"github.com/spf13/cobra"
)

type recallHit struct {
	Source  string `json:"source"`
	ID      string `json:"id"`
	Title   string `json:"title"`
	Snippet string `json:"snippet"`
}

func newNovelResearchRecallCmd(flags *rootFlags) *cobra.Command {
	var flagQuery string
	var flagLimit int
	var flagType string

	cmd := &cobra.Command{
		Use:   "recall [query]",
		Short: "FTS across local searches, extracts, and task summaries with typed hit IDs.",
		Long: `Search the local store for prior web searches, extracts, task runs,
and FindAll results. Use hits.source and hits.id to resume work with the
matching command (e.g. tasks runs-get, findall get-result).

This is the offline counterpart to live research — run sync first for coverage.`,
		Example: strings.Trim(`
  parallel-pp-cli research recall "Anthropic funding" --json --agent --select hits.source,hits.id,hits.title
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if err := validateDataSourceStrategy(flags, "local"); err != nil {
				return err
			}

			query := strings.TrimSpace(flagQuery)
			if query == "" && len(args) > 0 {
				query = strings.TrimSpace(strings.Join(args, " "))
			}
			if query == "" && !hasChangedLocalFlags(cmd) {
				return cmd.Help()
			}
			if query == "" {
				return usageErr(fmt.Errorf("query is required as a positional argument or --query"))
			}

			limit := flagLimit
			if limit <= 0 {
				limit = 20
			}
			typeFilter := strings.TrimSpace(flagType)
			if typeFilter != "" {
				switch typeFilter {
				case "websearch", "extract", "tasks", "findall":
				default:
					return usageErr(fmt.Errorf("invalid --type %q; valid: websearch, extract, tasks, findall", typeFilter))
				}
			}

			db, err := openStoreForRead(cmd.Context(), "parallel-pp-cli")
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			if db == nil {
				out := map[string]any{"query": query, "hits": []recallHit{}}
				return flags.printJSON(cmd, out)
			}
			defer db.Close()

			hintIfUnsynced(cmd, db, "")
			hintIfStale(cmd, db, "", flags.maxAge)

			rawHits, err := recallSearch(db, query, limit, typeFilter)
			if err != nil {
				return fmt.Errorf("recall search: %w", err)
			}

			hits := make([]recallHit, 0, len(rawHits))
			for _, raw := range rawHits {
				if hit, ok := recallHitFromJSON(raw); ok {
					hits = append(hits, hit)
				}
			}

			return flags.printJSON(cmd, map[string]any{
				"query": query,
				"hits":  hits,
			})
		},
	}
	cmd.Flags().StringVar(&flagQuery, "query", "", "Search string (alternative to positional)")
	cmd.Flags().IntVar(&flagLimit, "limit", 20, "Maximum hits to return")
	cmd.Flags().StringVar(&flagType, "type", "", "Filter by source type: websearch, extract, tasks, findall")
	return cmd
}

func recallSearch(db *store.Store, query string, limit int, typeFilter string) ([]json.RawMessage, error) {
	switch typeFilter {
	case "findall":
		return db.SearchFindall(query, limit)
	case "websearch", "extract", "tasks":
		return db.Search(query, limit, typeFilter)
	case "":
		seen := make(map[string]bool)
		var results []json.RawMessage
		appendUnique := func(batch []json.RawMessage) {
			for _, r := range batch {
				key := string(r)
				if seen[key] {
					continue
				}
				seen[key] = true
				results = append(results, r)
				if len(results) >= limit {
					return
				}
			}
		}
		for _, rt := range []string{"websearch", "extract", "tasks"} {
			partial, err := db.Search(query, limit, rt)
			if err != nil {
				return nil, err
			}
			appendUnique(partial)
			if len(results) >= limit {
				break
			}
		}
		if len(results) < limit {
			partial, err := db.SearchFindall(query, limit-len(results))
			if err != nil {
				return nil, err
			}
			appendUnique(partial)
		}
		return results, nil
	default:
		return nil, fmt.Errorf("unsupported type %q", typeFilter)
	}
}

func recallHitFromJSON(raw json.RawMessage) (recallHit, bool) {
	var obj map[string]any
	if json.Unmarshal(raw, &obj) != nil {
		return recallHit{}, false
	}
	hit := recallHit{
		Source:  firstString(obj, "resource_type", "source", "type"),
		ID:      firstRecallID(obj),
		Title:   firstString(obj, "title", "name", "objective", "query"),
		Snippet: recallSnippet(obj),
	}
	if hit.Source == "" {
		hit.Source = inferRecallSource(obj)
	}
	if hit.ID == "" && hit.Title == "" && hit.Snippet == "" {
		return recallHit{}, false
	}
	return hit, true
}

func firstRecallID(obj map[string]any) string {
	for _, k := range []string{"id", "run_id", "search_id", "extract_id", "findall_id", "candidate_id", "taskgroup_id"} {
		if v, ok := obj[k].(string); ok && strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func inferRecallSource(obj map[string]any) string {
	if _, ok := obj["findall_id"]; ok {
		return "findall"
	}
	if _, ok := obj["extract_id"]; ok {
		return "extract"
	}
	if _, ok := obj["run_id"]; ok {
		return "tasks"
	}
	if _, ok := obj["search_id"]; ok {
		return "websearch"
	}
	return "resources"
}

func recallSnippet(obj map[string]any) string {
	if s := firstString(obj, "snippet", "summary", "description", "output", "objective"); s != "" {
		return truncate(s, 200)
	}
	if s := firstString(obj, "url"); s != "" {
		return s
	}
	raw, _ := json.Marshal(obj)
	return truncate(string(raw), 200)
}
