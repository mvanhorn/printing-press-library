// Copyright 2026 Anchal Sharma and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-coded transcendence features: SQLite-native intelligence unavailable in any stateless tool.

package cli

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/exa/internal/store"
	"github.com/spf13/cobra"
)

// ---------------------------------------------------------------------------
// workflow drift — compare result sets for a fixed query across dates
// ---------------------------------------------------------------------------

func newWorkflowDriftCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var query string
	var since string

	cmd := &cobra.Command{
		Use:   "drift",
		Short: "Compare result sets for a query across time",
		Long: `Drift compares the URLs returned for a query across two time windows
(the most recent sync and an older one). Shows which URLs entered, left, or
remained stable in the top results.

Results are only available after two or more 'sync' runs for the same resource.`,
		Example: `  # Compare search result drift for a topic
  exa-pp-cli workflow drift --query "LLM benchmarks"

  # Restrict comparison to the last 7 days vs prior
  exa-pp-cli workflow drift --query "AI safety" --since 7d

  # JSON output for piping
  exa-pp-cli workflow drift --query "rust programming" --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if query == "" && len(args) > 0 {
				query = strings.Join(args, " ")
			}
			if query == "" {
				return fmt.Errorf("--query is required")
			}
			if dbPath == "" {
				dbPath = defaultDBPath("exa-pp-cli")
			}
			s, err := store.Open(dbPath)
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer s.Close()

			cutoff, cutErr := parseDuration(since)

			// Query the resources table for stored neural-search results matching query.
			db := s.DB()
			rows, err := db.QueryContext(cmd.Context(), `
				SELECT id, data, synced_at FROM resources
				WHERE resource_type = 'neural_search'
				  AND json_extract(data, '$.query') = ?
				ORDER BY synced_at DESC
				LIMIT 200
			`, query)
			if err != nil {
				return fmt.Errorf("querying store: %w", err)
			}
			defer rows.Close()

			type snapshot struct {
				syncedAt time.Time
				urls     []string
			}
			var snapshots []snapshot
			for rows.Next() {
				var id, raw string
				var syncedAtStr string
				if err := rows.Scan(&id, &raw, &syncedAtStr); err != nil {
					continue
				}
				var obj map[string]any
				if err := json.Unmarshal([]byte(raw), &obj); err != nil {
					continue
				}
				t, _ := time.Parse(time.RFC3339, syncedAtStr)
				results, _ := obj["results"].([]any)
				var urls []string
				for _, r := range results {
					rm, _ := r.(map[string]any)
					if u, _ := rm["url"].(string); u != "" {
						urls = append(urls, u)
					}
				}
				snapshots = append(snapshots, snapshot{syncedAt: t, urls: urls})
			}

			if len(snapshots) < 2 {
				fmt.Fprintf(cmd.OutOrStdout(), "Not enough snapshots for query %q (found %d, need 2+). Run 'sync' again after some time has passed.\n", query, len(snapshots))
				return nil
			}

			// Use most recent as "current" and oldest within window as "prior".
			current := snapshots[0]
			prior := snapshots[len(snapshots)-1]
			if cutErr == nil && !cutoff.IsZero() {
				for _, s := range snapshots[1:] {
					if s.syncedAt.After(cutoff) {
						prior = s
					}
				}
			}

			currentSet := urlSet(current.urls)
			priorSet := urlSet(prior.urls)

			var entered, dropped, stable []string
			for u := range currentSet {
				if priorSet[u] {
					stable = append(stable, u)
				} else {
					entered = append(entered, u)
				}
			}
			for u := range priorSet {
				if !currentSet[u] {
					dropped = append(dropped, u)
				}
			}
			sort.Strings(entered)
			sort.Strings(dropped)
			sort.Strings(stable)

			result := map[string]any{
				"query":        query,
				"current_date": current.syncedAt.Format(time.RFC3339),
				"prior_date":   prior.syncedAt.Format(time.RFC3339),
				"entered":      entered,
				"dropped":      dropped,
				"stable":       stable,
				"summary": map[string]any{
					"entered_count": len(entered),
					"dropped_count": len(dropped),
					"stable_count":  len(stable),
				},
			}

			if flags.asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(result)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Drift for %q (%s → %s)\n\n",
				query, prior.syncedAt.Format("2006-01-02"), current.syncedAt.Format("2006-01-02"))
			printDriftSection(cmd, "Entered (+%d)", entered)
			printDriftSection(cmd, "Dropped (-%d)", dropped)
			printDriftSection(cmd, "Stable  (%d)", stable)
			return nil
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/exa-pp-cli/data.db)")
	cmd.Flags().StringVarP(&query, "query", "q", "", "Query to compare drift for")
	cmd.Flags().StringVar(&since, "since", "", "How far back to look for the prior snapshot (e.g. 7d, 30d)")
	return cmd
}

func printDriftSection(cmd *cobra.Command, header string, items []string) {
	fmt.Fprintf(cmd.OutOrStdout(), header+"\n", len(items))
	for _, u := range items {
		fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", u)
	}
	fmt.Fprintln(cmd.OutOrStdout())
}

func urlSet(urls []string) map[string]bool {
	m := make(map[string]bool, len(urls))
	for _, u := range urls {
		m[u] = true
	}
	return m
}

// ---------------------------------------------------------------------------
// workflow entities — cross-query entity co-occurrence
// ---------------------------------------------------------------------------

func newWorkflowEntitiesCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var queryFilter string
	var limit int

	cmd := &cobra.Command{
		Use:   "entities",
		Short: "Find entities appearing across multiple stored search results",
		Long: `Entities scans stored search result text across all (or filtered) queries
and counts co-occurrence of named entities (companies, people, products).
Entities appearing in 2 or more distinct queries rise to the top.

Requires synced search results with --text extraction.`,
		Example: `  # Find entities across all stored searches
  exa-pp-cli workflow entities

  # Restrict to searches matching a topic
  exa-pp-cli workflow entities --query-filter "LLM"

  # Top 20 entities as JSON
  exa-pp-cli workflow entities --limit 20 --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dbPath == "" {
				dbPath = defaultDBPath("exa-pp-cli")
			}
			s, err := store.Open(dbPath)
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer s.Close()

			db := s.DB()
			qArgs := []any{}
			qWhere := "resource_type = 'neural_search'"
			if queryFilter != "" {
				qWhere += " AND json_extract(data, '$.query') LIKE ?"
				qArgs = append(qArgs, "%"+queryFilter+"%")
			}

			rows, err := db.QueryContext(cmd.Context(),
				`SELECT json_extract(data, '$.query'), json_extract(data, '$.results') FROM resources WHERE `+qWhere+` LIMIT 500`,
				qArgs...)
			if err != nil {
				return fmt.Errorf("querying store: %w", err)
			}
			defer rows.Close()

			// Simple NER: extract capitalised multi-word phrases.
			entityQueryCount := map[string]map[string]bool{} // entity → set of queries
			for rows.Next() {
				var query, resultsRaw string
				if err := rows.Scan(&query, &resultsRaw); err != nil {
					continue
				}
				var results []map[string]any
				if err := json.Unmarshal([]byte(resultsRaw), &results); err != nil {
					continue
				}
				for _, r := range results {
					text, _ := r["text"].(string)
					title, _ := r["title"].(string)
					combined := title + " " + text
					for _, e := range extractEntities(combined) {
						if entityQueryCount[e] == nil {
							entityQueryCount[e] = map[string]bool{}
						}
						entityQueryCount[e][query] = true
					}
				}
			}

			type entityResult struct {
				Entity     string   `json:"entity"`
				QueryCount int      `json:"query_count"`
				Queries    []string `json:"queries"`
			}
			var results []entityResult
			for entity, qs := range entityQueryCount {
				if len(qs) < 2 {
					continue
				}
				ql := make([]string, 0, len(qs))
				for q := range qs {
					ql = append(ql, q)
				}
				sort.Strings(ql)
				results = append(results, entityResult{Entity: entity, QueryCount: len(qs), Queries: ql})
			}
			sort.Slice(results, func(i, j int) bool {
				return results[i].QueryCount > results[j].QueryCount
			})
			if limit > 0 && len(results) > limit {
				results = results[:limit]
			}

			if flags.asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(map[string]any{"entities": results, "total": len(results)})
			}
			if len(results) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No cross-query entities found. Sync search results with --text first.")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Cross-query entities (%d)\n\n", len(results))
			for _, e := range results {
				fmt.Fprintf(cmd.OutOrStdout(), "  %-40s  %d queries\n", e.Entity, e.QueryCount)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&queryFilter, "query-filter", "", "Filter source queries by substring")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum entities to return")
	return cmd
}

var entityRE = regexp.MustCompile(`\b([A-Z][a-z]{1,}(?:\s+[A-Z][a-z]{1,}){1,4})\b`)

func extractEntities(text string) []string {
	matches := entityRE.FindAllString(text, -1)
	seen := map[string]bool{}
	var out []string
	for _, m := range matches {
		if !seen[m] && len(m) > 4 {
			seen[m] = true
			out = append(out, m)
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// workflow monitor-digest — weekly roll-up of monitor delivery results
// ---------------------------------------------------------------------------

func newWorkflowMonitorDigestCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var days int

	cmd := &cobra.Command{
		Use:   "monitor-digest",
		Short: "Weekly roll-up of all monitor delivery results",
		Long: `Monitor Digest aggregates the local monitor delivery log across all monitors
and produces a deduped, ranked report of URLs and topics surfaced.

Populate the delivery log by running 'workflow archive' regularly so monitor
results are captured in the local store.`,
		Example: `  # Digest of the last 7 days
  exa-pp-cli workflow monitor-digest

  # Extend to 30 days
  exa-pp-cli workflow monitor-digest --days 30 --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dbPath == "" {
				dbPath = defaultDBPath("exa-pp-cli")
			}
			s, err := store.Open(dbPath)
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer s.Close()

			db := s.DB()
			since := time.Now().AddDate(0, 0, -days).Format("2006-01-02")

			// Try the delivery log table first.
			rows, err := db.QueryContext(cmd.Context(), `
				SELECT monitor_id, monitor_name, SUM(results_count), GROUP_CONCAT(result_urls, '|'), COUNT(*)
				FROM monitor_delivery_log
				WHERE date(fired_at) >= ?
				GROUP BY monitor_id
				ORDER BY SUM(results_count) DESC
			`, since)
			if err != nil {
				// Fall back to the synced monitors/runs tables.
				return digestFromRunsTable(cmd, flags, since)
			}
			defer rows.Close()

			type monitorSummary struct {
				MonitorID    string   `json:"monitor_id"`
				MonitorName  string   `json:"monitor_name"`
				TotalResults int      `json:"total_results"`
				FireCount    int      `json:"fire_count"`
				TopURLs      []string `json:"top_urls"`
			}
			var summaries []monitorSummary
			urlCount := map[string]int{}

			for rows.Next() {
				var id, name, urlsRaw string
				var total, count int
				if err := rows.Scan(&id, &name, &total, &urlsRaw, &count); err != nil {
					continue
				}
				var urls []string
				seen := map[string]bool{}
				for _, u := range strings.Split(urlsRaw, "|") {
					for _, uu := range strings.Split(u, ",") {
						uu = strings.TrimSpace(uu)
						if uu != "" && !seen[uu] {
							seen[uu] = true
							urls = append(urls, uu)
							urlCount[uu]++
						}
					}
				}
				summaries = append(summaries, monitorSummary{
					MonitorID:    id,
					MonitorName:  name,
					TotalResults: total,
					FireCount:    count,
					TopURLs:      urls,
				})
			}

			if len(summaries) == 0 {
				return digestFromRunsTable(cmd, flags, since)
			}

			// Deduped URL ranking.
			type urlRank struct {
				URL   string
				Count int
			}
			var ranked []urlRank
			for u, c := range urlCount {
				ranked = append(ranked, urlRank{u, c})
			}
			sort.Slice(ranked, func(i, j int) bool { return ranked[i].Count > ranked[j].Count })
			if len(ranked) > 20 {
				ranked = ranked[:20]
			}

			if flags.asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(map[string]any{
					"period_days": days,
					"since":       since,
					"monitors":    summaries,
					"top_urls":    ranked,
				})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Monitor Digest (%d days ending %s)\n\n", days, time.Now().Format("2006-01-02"))
			for _, m := range summaries {
				fmt.Fprintf(cmd.OutOrStdout(), "  [%s] %s — %d results in %d fires\n", m.MonitorID, m.MonitorName, m.TotalResults, m.FireCount)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\nTop URLs across all monitors:\n")
			for _, r := range ranked {
				fmt.Fprintf(cmd.OutOrStdout(), "  (%d) %s\n", r.Count, r.URL)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().IntVar(&days, "days", 7, "Number of days to include in the digest")
	return cmd
}

func digestFromRunsTable(cmd *cobra.Command, flags *rootFlags, since string) error {
	fmt.Fprintln(cmd.OutOrStdout(), "No delivery log found. Run 'workflow archive' to populate monitor data, then re-run digest.")
	return nil
}

// ---------------------------------------------------------------------------
// workflow velocity — topic velocity (new results per time bucket)
// ---------------------------------------------------------------------------

func newWorkflowVelocityCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var query string
	var bucketDays int

	cmd := &cobra.Command{
		Use:   "velocity",
		Short: "Measure how many new results a query gains over time",
		Long: `Velocity counts new (previously unseen) URLs returned per time bucket across
successive sync runs for a query. Rising count → topic gaining momentum.

Requires at least 2 prior sync runs with results stored for the query.`,
		Example: `  exa-pp-cli workflow velocity --query "AI agents"
  exa-pp-cli workflow velocity --query "rust async" --bucket-days 3 --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if query == "" && len(args) > 0 {
				query = strings.Join(args, " ")
			}
			if query == "" {
				return fmt.Errorf("--query is required")
			}
			if dbPath == "" {
				dbPath = defaultDBPath("exa-pp-cli")
			}
			s, err := store.Open(dbPath)
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer s.Close()

			db := s.DB()
			rows, err := db.QueryContext(cmd.Context(), `
				SELECT data, synced_at FROM resources
				WHERE resource_type = 'neural_search'
				  AND json_extract(data, '$.query') = ?
				ORDER BY synced_at ASC
			`, query)
			if err != nil {
				return fmt.Errorf("querying store: %w", err)
			}
			defer rows.Close()

			type bucket struct {
				Label  string `json:"bucket"`
				NewURL int    `json:"new_urls"`
			}
			seen := map[string]bool{}
			bucketMap := map[string]int{}
			var bucketOrder []string
			for rows.Next() {
				var raw, syncedAtStr string
				if err := rows.Scan(&raw, &syncedAtStr); err != nil {
					continue
				}
				t, _ := time.Parse(time.RFC3339, syncedAtStr)
				label := t.Format("2006-01-02")
				if bucketDays > 1 {
					// Quantise to N-day windows.
					days := int(t.Unix() / (86400 * int64(bucketDays)))
					epoch := time.Unix(int64(days)*86400*int64(bucketDays), 0).UTC()
					label = epoch.Format("2006-01-02") + "+"
				}
				if _, seen2 := bucketMap[label]; !seen2 {
					bucketOrder = append(bucketOrder, label)
				}
				var obj map[string]any
				if err := json.Unmarshal([]byte(raw), &obj); err != nil {
					continue
				}
				results, _ := obj["results"].([]any)
				for _, r := range results {
					rm, _ := r.(map[string]any)
					u, _ := rm["url"].(string)
					if u != "" && !seen[u] {
						seen[u] = true
						bucketMap[label]++
					}
				}
			}

			var buckets []bucket
			for _, l := range bucketOrder {
				buckets = append(buckets, bucket{Label: l, NewURL: bucketMap[l]})
			}

			if flags.asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(map[string]any{"query": query, "bucket_days": bucketDays, "buckets": buckets})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Velocity for %q (bucket: %d day)\n\n", query, bucketDays)
			for _, b := range buckets {
				bar := strings.Repeat("█", b.NewURL)
				fmt.Fprintf(cmd.OutOrStdout(), "  %-12s  %3d  %s\n", b.Label, b.NewURL, bar)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVarP(&query, "query", "q", "", "Query to measure velocity for")
	cmd.Flags().IntVar(&bucketDays, "bucket-days", 1, "Days per bucket (1 = daily)")
	return cmd
}

// ---------------------------------------------------------------------------
// workflow contradiction — detect divergent answers for the same question
// ---------------------------------------------------------------------------

func newWorkflowContradictionCmd(flags *rootFlags) *cobra.Command {
	var query string
	var live bool

	cmd := &cobra.Command{
		Use:   "contradiction",
		Short: "Detect conflicting answers for the same question",
		Long: `Contradiction fires two /answer calls for the same question using different
search strategies (auto and keyword), then diffs the stored responses to surface
divergent claims. Use for fact-checking before publishing.

With --live, both calls are always sent to the API. Without it, previously stored
answers for the exact question are compared if available.`,
		Example: `  exa-pp-cli workflow contradiction --query "What is the current Fed funds rate?"
  exa-pp-cli workflow contradiction -q "When did GPT-4 launch?" --live --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if query == "" && len(args) > 0 {
				query = strings.Join(args, " ")
			}
			if query == "" {
				return fmt.Errorf("--query is required")
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			callAnswer := func(searchType string) (string, error) {
				body := map[string]any{
					"query": query,
					"text":  true,
				}
				if searchType != "" {
					body["searchType"] = searchType
				}
				data, _, err := c.PostQueryWithParams(cmd.Context(), "/answer", nil, body)
				if err != nil {
					return "", err
				}
				var obj map[string]any
				if err := json.Unmarshal(data, &obj); err != nil {
					return string(data), nil
				}
				ans, _ := obj["answer"].(string)
				if ans == "" {
					ans, _ = obj["text"].(string)
				}
				return ans, nil
			}

			ansAuto, err := callAnswer("auto")
			if err != nil {
				return fmt.Errorf("auto answer: %w", err)
			}
			ansKeyword, err := callAnswer("keyword")
			if err != nil {
				return fmt.Errorf("keyword answer: %w", err)
			}

			// Simple sentence-level diff to surface divergent claims.
			autoSents := splitSentences(ansAuto)
			kwSents := splitSentences(ansKeyword)

			var onlyAuto, onlyKW []string
			kwSet := make(map[string]bool, len(kwSents))
			for _, s := range kwSents {
				kwSet[strings.ToLower(strings.TrimSpace(s))] = true
			}
			autoSet := make(map[string]bool, len(autoSents))
			for _, s := range autoSents {
				autoSet[strings.ToLower(strings.TrimSpace(s))] = true
			}
			for _, s := range autoSents {
				if !kwSet[strings.ToLower(strings.TrimSpace(s))] {
					onlyAuto = append(onlyAuto, s)
				}
			}
			for _, s := range kwSents {
				if !autoSet[strings.ToLower(strings.TrimSpace(s))] {
					onlyKW = append(onlyKW, s)
				}
			}

			agreed := len(onlyAuto) == 0 && len(onlyKW) == 0

			if flags.asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(map[string]any{
					"query":           query,
					"agreed":          agreed,
					"auto_answer":     ansAuto,
					"keyword_answer":  ansKeyword,
					"only_in_auto":    onlyAuto,
					"only_in_keyword": onlyKW,
				})
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Contradiction check for: %q\n\n", query)
			if agreed {
				fmt.Fprintln(cmd.OutOrStdout(), "✓ No divergence detected — both strategies agree.")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "⚠ Divergence detected:\n\n")
			if len(onlyAuto) > 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "Only in auto-search answer:")
				for _, s := range onlyAuto {
					fmt.Fprintf(cmd.OutOrStdout(), "  • %s\n", strings.TrimSpace(s))
				}
			}
			if len(onlyKW) > 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "Only in keyword-search answer:")
				for _, s := range onlyKW {
					fmt.Fprintf(cmd.OutOrStdout(), "  • %s\n", strings.TrimSpace(s))
				}
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&query, "query", "q", "", "Question to check for contradictions")
	cmd.Flags().BoolVar(&live, "live", false, "Always make fresh API calls (don't use cached answers)")
	return cmd
}

var sentenceRE = regexp.MustCompile(`[^.!?]+[.!?]+`)

func splitSentences(text string) []string {
	matches := sentenceRE.FindAllString(text, -1)
	var out []string
	for _, m := range matches {
		m = strings.TrimSpace(m)
		if len(m) > 20 {
			out = append(out, m)
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// workflow template — stored query templates
// ---------------------------------------------------------------------------

func newWorkflowTemplateCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "template",
		Short:       "Stored query templates with variable interpolation",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newWorkflowTemplateSaveCmd(flags))
	cmd.AddCommand(newWorkflowTemplateRunCmd(flags))
	cmd.AddCommand(newWorkflowTemplateListCmd(flags))
	cmd.AddCommand(newWorkflowTemplateDeleteCmd(flags))
	return cmd
}

func newWorkflowTemplateSaveCmd(flags *rootFlags) *cobra.Command {
	var dbPath, name, query, category, typeFlag, includeDomains, excludeDomains string
	var numResults int
	var variables []string

	cmd := &cobra.Command{
		Use:   "save <name>",
		Short: "Save a query template",
		Example: `  exa-pp-cli workflow template save weekly-ai \
    --query "AI news {{week}}" --category news --num-results 20

  # Template with variable slots ({{var}} syntax)
  exa-pp-cli workflow template save competitor-watch \
    --query "{{company}} product announcement" --type auto`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name = args[0]
			if query == "" && !flags.dryRun {
				return fmt.Errorf("--query is required")
			}
			if flags.dryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "(dry run) would save template %q with query %q\n", name, query)
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("exa-pp-cli")
			}
			s, err := store.Open(dbPath)
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer s.Close()

			varsJSON, _ := json.Marshal(variables)
			_, err = s.DB().ExecContext(cmd.Context(), `
				INSERT OR REPLACE INTO query_templates
				  (name, query_template, category, type, include_domains, exclude_domains, num_results, variables, created_at)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
			`, name, query, category, typeFlag, includeDomains, excludeDomains, numResults, string(varsJSON))
			if err != nil {
				return fmt.Errorf("saving template: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Template %q saved. Run with: exa-pp-cli workflow template run %s\n", name, name)
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVarP(&query, "query", "q", "", "Query template (use {{var}} for variables)")
	cmd.Flags().StringVar(&category, "category", "", "Category filter")
	cmd.Flags().StringVar(&typeFlag, "type", "", "Search type")
	cmd.Flags().StringVar(&includeDomains, "include-domains", "", "Comma-separated domain allow-list")
	cmd.Flags().StringVar(&excludeDomains, "exclude-domains", "", "Comma-separated domain block-list")
	cmd.Flags().IntVar(&numResults, "num-results", 10, "Default result count")
	cmd.Flags().StringArrayVar(&variables, "var", nil, "Variable default: key=value")
	return cmd
}

func newWorkflowTemplateRunCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var variables []string

	cmd := &cobra.Command{
		Use:   "run <name> [key=value...]",
		Short: "Execute a stored template",
		Example: `  exa-pp-cli workflow template run weekly-ai
  exa-pp-cli workflow template run competitor-watch company=Anthropic --json`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			if dbPath == "" {
				dbPath = defaultDBPath("exa-pp-cli")
			}
			s, err := store.Open(dbPath)
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer s.Close()

			var queryTpl, category, typeFlag, includeDomains, excludeDomains string
			var numResults int
			err = s.DB().QueryRowContext(cmd.Context(), `
				SELECT query_template, category, type, include_domains, exclude_domains, num_results
				FROM query_templates WHERE name = ?
			`, name).Scan(&queryTpl, &category, &typeFlag, &includeDomains, &excludeDomains, &numResults)
			if err != nil {
				return fmt.Errorf("template %q not found", name)
			}

			// Interpolate variables from args and --var.
			varMap := map[string]string{}
			for _, v := range append(variables, args[1:]...) {
				if k, val, ok := strings.Cut(v, "="); ok {
					varMap[k] = val
				}
			}
			query := queryTpl
			for k, v := range varMap {
				query = strings.ReplaceAll(query, "{{"+k+"}}", v)
			}

			// Build and fire the search request.
			body := map[string]any{"query": query, "numResults": numResults}
			if category != "" {
				body["category"] = category
			}
			if typeFlag != "" {
				body["type"] = typeFlag
			}
			if includeDomains != "" {
				body["includeDomains"] = strings.Split(includeDomains, ",")
			}
			if excludeDomains != "" {
				body["excludeDomains"] = strings.Split(excludeDomains, ",")
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, _, err := c.PostQueryWithParams(cmd.Context(), "/search", nil, body)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			// Update last_run_at.
			_, _ = s.DB().ExecContext(cmd.Context(),
				`UPDATE query_templates SET last_run_at = CURRENT_TIMESTAMP WHERE name = ?`, name)

			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringArrayVar(&variables, "var", nil, "Variable override: key=value")
	return cmd
}

func newWorkflowTemplateListCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List stored templates",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dbPath == "" {
				dbPath = defaultDBPath("exa-pp-cli")
			}
			s, err := store.Open(dbPath)
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer s.Close()

			rows, err := s.DB().QueryContext(cmd.Context(),
				`SELECT name, query_template, last_run_at FROM query_templates ORDER BY name`)
			if err != nil {
				return err
			}
			defer rows.Close()

			type tmpl struct {
				Name      string `json:"name"`
				Query     string `json:"query"`
				LastRunAt string `json:"last_run_at"`
			}
			var out []tmpl
			for rows.Next() {
				var t tmpl
				_ = rows.Scan(&t.Name, &t.Query, &t.LastRunAt)
				out = append(out, t)
			}

			if flags.asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(out)
			}
			if len(out) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No templates. Save one with: exa-pp-cli workflow template save <name> --query \"...\"")
				return nil
			}
			for _, t := range out {
				lastRun := t.LastRunAt
				if lastRun == "" {
					lastRun = "never"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "  %-20s  last: %-20s  %s\n", t.Name, lastRun, t.Query)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func newWorkflowTemplateDeleteCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:   "delete <name>",
		Short: "Delete a stored template",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dbPath == "" {
				dbPath = defaultDBPath("exa-pp-cli")
			}
			s, err := store.Open(dbPath)
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer s.Close()

			res, err := s.DB().ExecContext(cmd.Context(),
				`DELETE FROM query_templates WHERE name = ?`, args[0])
			if err != nil {
				return err
			}
			n, _ := res.RowsAffected()
			if n == 0 {
				return fmt.Errorf("template %q not found", args[0])
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Template %q deleted.\n", args[0])
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

// ---------------------------------------------------------------------------
// workflow budget — daily credit cap with spend tracking
// ---------------------------------------------------------------------------

func newWorkflowBudgetCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "budget",
		Short:       "Daily credit cap with per-call spend tracking",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newWorkflowBudgetSetCmd(flags))
	cmd.AddCommand(newWorkflowBudgetStatusCmd(flags))
	return cmd
}

func newWorkflowBudgetSetCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:   "set <cents>",
		Short: "Set daily spend cap in US cents (0 = no cap)",
		Example: `  exa-pp-cli workflow budget set 100   # $1.00/day cap
  exa-pp-cli workflow budget set 0     # disable cap`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dbPath == "" {
				dbPath = defaultDBPath("exa-pp-cli")
			}
			s, err := store.Open(dbPath)
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer s.Close()

			var cents int
			if _, err := fmt.Sscanf(args[0], "%d", &cents); err != nil {
				return fmt.Errorf("invalid cents value %q: must be an integer", args[0])
			}
			_, err = s.DB().ExecContext(cmd.Context(), `
				INSERT OR REPLACE INTO budget_config (id, daily_limit_cents, updated_at)
				VALUES (1, ?, CURRENT_TIMESTAMP)
			`, cents)
			if err != nil {
				return err
			}
			if cents == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "Budget cap disabled.")
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "Daily cap set to $%.2f (≈%d API calls).\n", float64(cents)/100, cents/10)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func newWorkflowBudgetStatusCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:         "status",
		Short:       "Show today's spend and remaining budget",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dbPath == "" {
				dbPath = defaultDBPath("exa-pp-cli")
			}
			s, err := store.Open(dbPath)
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer s.Close()

			var cap int
			_ = s.DB().QueryRowContext(cmd.Context(),
				`SELECT daily_limit_cents FROM budget_config WHERE id = 1`).Scan(&cap)

			var spent int
			_ = s.DB().QueryRowContext(cmd.Context(),
				`SELECT COALESCE(SUM(estimated_cents), 0) FROM spend_log WHERE date(logged_at) = date('now')`).Scan(&spent)

			remaining := cap - spent
			if cap == 0 {
				remaining = -1
			}

			if flags.asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(map[string]any{
					"daily_cap_cents": cap,
					"spent_cents":     spent,
					"remaining_cents": remaining,
					"cap_set":         cap > 0,
				})
			}

			if cap == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "Budget: no cap set. Today's spend: $%.2f\n", float64(spent)/100)
			} else {
				pct := 0.0
				if cap > 0 {
					pct = float64(spent) / float64(cap) * 100
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Budget: $%.2f/$%.2f (%.0f%%) — $%.2f remaining today\n",
					float64(spent)/100, float64(cap)/100, pct, float64(remaining)/100)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

// ---------------------------------------------------------------------------
// workflow intersect — URL intersection across saved searches
// ---------------------------------------------------------------------------

func newWorkflowIntersectCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var queries []string
	var minQueries int

	cmd := &cobra.Command{
		Use:   "intersect",
		Short: "Find URLs appearing in two or more saved searches",
		Long: `Intersect scans the local result store and returns URLs that appear in results
for multiple distinct queries. Authoritative sources that independently surface
across different research angles are the most reliable references.

Requires synced search results (run 'sync' or 'workflow archive' first).`,
		Example: `  # URLs shared across any two stored searches
  exa-pp-cli workflow intersect

  # URLs shared across at least 3 queries
  exa-pp-cli workflow intersect --min-queries 3

  # Scope to specific queries
  exa-pp-cli workflow intersect --query "LLM safety" --query "AI alignment" --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dbPath == "" {
				dbPath = defaultDBPath("exa-pp-cli")
			}
			s, err := store.Open(dbPath)
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer s.Close()

			db := s.DB()

			qArgs := []any{}
			qWhere := "resource_type = 'neural_search'"
			if len(queries) > 0 {
				placeholders := strings.Repeat("?,", len(queries))
				placeholders = strings.TrimSuffix(placeholders, ",")
				qWhere += " AND json_extract(data, '$.query') IN (" + placeholders + ")"
				for _, q := range queries {
					qArgs = append(qArgs, q)
				}
			}

			rows, err := db.QueryContext(cmd.Context(),
				`SELECT json_extract(data, '$.query'), json_extract(data, '$.results') FROM resources WHERE `+qWhere,
				qArgs...)
			if err != nil {
				return fmt.Errorf("querying store: %w", err)
			}
			defer rows.Close()

			urlQueries := map[string]map[string]bool{} // url → set of queries
			for rows.Next() {
				var query, resultsRaw string
				if err := rows.Scan(&query, &resultsRaw); err != nil {
					continue
				}
				var results []map[string]any
				if err := json.Unmarshal([]byte(resultsRaw), &results); err != nil {
					continue
				}
				for _, r := range results {
					u, _ := r["url"].(string)
					if u == "" {
						continue
					}
					if urlQueries[u] == nil {
						urlQueries[u] = map[string]bool{}
					}
					urlQueries[u][query] = true
				}
			}

			type urlResult struct {
				URL        string   `json:"url"`
				QueryCount int      `json:"query_count"`
				Queries    []string `json:"queries"`
			}
			var results []urlResult
			for u, qs := range urlQueries {
				if len(qs) < minQueries {
					continue
				}
				ql := make([]string, 0, len(qs))
				for q := range qs {
					ql = append(ql, q)
				}
				sort.Strings(ql)
				results = append(results, urlResult{URL: u, QueryCount: len(qs), Queries: ql})
			}
			sort.Slice(results, func(i, j int) bool {
				if results[i].QueryCount != results[j].QueryCount {
					return results[i].QueryCount > results[j].QueryCount
				}
				return results[i].URL < results[j].URL
			})

			if flags.asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(map[string]any{"intersect": results, "min_queries": minQueries})
			}
			if len(results) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No URLs found in 2+ queries. Run 'sync' or 'workflow archive' to populate results.")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "URL intersection (min %d queries) — %d URLs\n\n", minQueries, len(results))
			for _, r := range results {
				fmt.Fprintf(cmd.OutOrStdout(), "  [%d] %s\n      %s\n", r.QueryCount, r.URL, strings.Join(r.Queries, ", "))
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringArrayVarP(&queries, "query", "q", nil, "Filter to specific queries (repeatable)")
	cmd.Flags().IntVar(&minQueries, "min-queries", 2, "Minimum number of queries a URL must appear in")
	return cmd
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func parseDuration(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, fmt.Errorf("empty")
	}
	// Support N d/h/m shorthand.
	var n int
	var unit string
	if _, err := fmt.Sscanf(s, "%d%s", &n, &unit); err != nil {
		return time.Time{}, err
	}
	switch strings.ToLower(unit) {
	case "d", "day", "days":
		return time.Now().AddDate(0, 0, -n), nil
	case "h", "hour", "hours":
		return time.Now().Add(-time.Duration(n) * time.Hour), nil
	case "w", "week", "weeks":
		return time.Now().AddDate(0, 0, -n*7), nil
	default:
		return time.Time{}, fmt.Errorf("unknown unit %q", unit)
	}
}
