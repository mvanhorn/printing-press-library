package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/ai/surgegraph/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/ai/surgegraph/internal/store"

	"github.com/spf13/cobra"
)

// newVisibilityNovelCmd is the parent for the five transcendence-tier
// visibility commands: delta, prompts losers, citation-domains rank-shift,
// portfolio, and traffic-citations. The flat spec-derived
// `get-ai-visibility-*` commands remain available; these wrap them with
// local-store joins, deltas, and aggregations no single API call returns.
func newVisibilityNovelCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "visibility",
		Short: "AI Visibility transcendence commands (local-store deltas, joins, roll-ups)",
		Long: strings.TrimSpace(`
AI Visibility commands powered by the local SQLite snapshot store. Run
'surgegraph-pp-cli sync --project <id>' first to populate the store.

  visibility delta                       Week-over-week metric movers
  visibility prompts losers              Prompts whose citations or rank dropped
  visibility citation-domains rank-shift Citation-domain rank diff over a window
  visibility portfolio                   Cross-project roll-up (agency view)
  visibility traffic-citations           Traffic-page <-> citation join
`),
	}
	cmd.AddCommand(newVisibilityDeltaCmd(flags))
	cmd.AddCommand(newVisibilityPromptsCmd(flags))
	cmd.AddCommand(newVisibilityCitationDomainsCmd(flags))
	cmd.AddCommand(newVisibilityPortfolioCmd(flags))
	cmd.AddCommand(newVisibilityTrafficCitationsCmd(flags))
	return cmd
}

// ---------- visibility delta ----------

func newVisibilityDeltaCmd(flags *rootFlags) *cobra.Command {
	var projectID, brand, metricList, windowRaw string
	cmd := &cobra.Command{
		Use:   "delta",
		Short: "Show week-over-week (or windowed) movers across AI Visibility metrics",
		Long: strings.TrimSpace(`
Reads the two most-recent local snapshots for each requested metric and
emits row-wise deltas. --metric defaults to all four: overview, trend,
sentiment, traffic_summary. The folded-in 'sentiment-movers' case is
covered by --metric sentiment.
`),
		Example: strings.Trim(`
  surgegraph-pp-cli visibility delta --project proj_abc123 --window 7d
  surgegraph-pp-cli visibility delta --project proj_abc123 --metric sentiment --agent
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if projectID == "" {
				if dryRunOK(flags) {
					return nil
				}
				return errors.New("--project is required")
			}
			windowDur, err := cliutil.ParseDayDuration(windowRaw)
			if err != nil {
				return fmt.Errorf("--window: %w", err)
			}
			st, err := store.Open("")
			if err != nil {
				return err
			}
			defer st.Close()
			metrics := splitCSV(metricList)
			if len(metrics) == 0 {
				metrics = []string{"overview", "trend", "sentiment", "traffic_summary"}
			}
			ctx := cmd.Context()
			brandName := brand
			if brandName == "" {
				if p, err := resolveProject(ctx, st, projectID); err == nil && p != nil {
					brandName = pickBrandName(p)
				}
			}
			type row struct {
				Metric       string          `json:"metric"`
				Brand        string          `json:"brand"`
				NewerDate    string          `json:"newer_date"`
				OlderDate    string          `json:"older_date"`
				NewerPayload json.RawMessage `json:"newer_payload"`
				OlderPayload json.RawMessage `json:"older_payload,omitempty"`
				Note         string          `json:"note,omitempty"`
			}
			out := []row{}
			for _, m := range metrics {
				newer, older, err := st.LatestSnapshotPair(ctx, projectID, brandName, m)
				if err != nil {
					return fmt.Errorf("loading %s: %w", m, err)
				}
				r := row{Metric: m, Brand: brandName}
				if newer == nil {
					r.Note = fmt.Sprintf("no snapshots yet — run `surgegraph-pp-cli sync --project %s` first", projectID)
					out = append(out, r)
					continue
				}
				r.NewerDate = newer.SnapshotDate
				r.NewerPayload = newer.Payload
				if older == nil {
					r.Note = "only one snapshot in local store — run sync again tomorrow for delta"
				} else {
					// PATCH(greptile-5): respect --window. SnapshotDate is YYYY-MM-DD.
					// If the older snapshot falls outside the requested window,
					// treat as single-snapshot rather than presenting a stale pair
					// as a delta. The caller asked for "last N days"; an older
					// snapshot from months ago violates that contract.
					if d, perr := time.Parse("2006-01-02", older.SnapshotDate); perr == nil &&
						windowDur > 0 && time.Since(d) > windowDur {
						r.Note = fmt.Sprintf("older snapshot (%s) is outside --window %s — pair suppressed", older.SnapshotDate, windowDur)
					} else {
						r.OlderDate = older.SnapshotDate
						r.OlderPayload = older.Payload
					}
				}
				out = append(out, r)
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&projectID, "project", "", "Project ID")
	cmd.Flags().StringVar(&brand, "brand", "", "Brand name (defaults to project's primary)")
	cmd.Flags().StringVar(&metricList, "metric", "overview,trend,sentiment,traffic_summary", "Metric set (comma-separated)")
	cmd.Flags().StringVar(&windowRaw, "window", "7d", "Reserved: window for snapshot pair selection (accepts 7d, 168h, etc.)")
	return cmd
}

// ---------- visibility prompts ----------

func newVisibilityPromptsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "prompts",
		Short: "Prompt-level transcendence subcommands",
	}
	cmd.AddCommand(newVisibilityPromptsLosersCmd(flags))
	return cmd
}

func newVisibilityPromptsLosersCmd(flags *rootFlags) *cobra.Command {
	var projectID, sinceRaw string
	cmd := &cobra.Command{
		Use:   "losers",
		Short: "Prompts whose citation count or first-position rank dropped vs the prior window",
		Long: strings.TrimSpace(`
For each prompt, compute its average citation count and first-position
rank over the recent window (--since) vs the older snapshots. Negative
citation_delta or position_delta > 0 means the prompt lost ground.
`),
		Example: strings.Trim(`
  surgegraph-pp-cli visibility prompts losers --project proj_abc123 --since 30d
  surgegraph-pp-cli visibility prompts losers --project proj_abc123 --agent --select prompt_id,prompt,citation_delta
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if projectID == "" {
				if dryRunOK(flags) {
					return nil
				}
				return errors.New("--project is required")
			}
			sinceDur, err := cliutil.ParseDayDuration(sinceRaw)
			if err != nil {
				return fmt.Errorf("--since: %w", err)
			}
			st, err := store.Open("")
			if err != nil {
				return err
			}
			defer st.Close()
			cutoff := time.Now().UTC().Add(-sinceDur)
			rows, err := st.PromptDeltas(cmd.Context(), projectID, cutoff)
			if err != nil {
				return err
			}
			// Only emit "losers" — prompts where citation_delta < 0 or position got worse.
			type out struct {
				PromptID       string `json:"prompt_id"`
				Prompt         string `json:"prompt"`
				NewerCitations int    `json:"newer_citations"`
				OlderCitations int    `json:"older_citations"`
				CitationDelta  int    `json:"citation_delta"`
				PositionDelta  int    `json:"position_delta"`
				LatestDate     string `json:"latest_snapshot_at"`
			}
			result := []out{}
			for _, d := range rows {
				if d.CitationDelta < 0 || d.PositionDelta > 0 {
					result = append(result, out{
						PromptID:       d.PromptID,
						Prompt:         d.Prompt,
						NewerCitations: d.NewerCitations,
						OlderCitations: d.OlderCitations,
						CitationDelta:  d.CitationDelta,
						PositionDelta:  d.PositionDelta,
						LatestDate:     d.LatestSnapshotAt,
					})
				}
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&projectID, "project", "", "Project ID")
	cmd.Flags().StringVar(&sinceRaw, "since", "30d", "Window for newer-vs-older partition (e.g., 7d, 30d, 168h)")
	return cmd
}

// ---------- visibility citation-domains rank-shift ----------

func newVisibilityCitationDomainsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "citation-domains",
		Short: "Citation-domain transcendence subcommands",
	}
	cmd.AddCommand(newVisibilityCitationDomainsRankShiftCmd(flags))
	return cmd
}

func newVisibilityCitationDomainsRankShiftCmd(flags *rootFlags) *cobra.Command {
	var projectID, windowRaw string
	cmd := &cobra.Command{
		Use:   "rank-shift",
		Short: "Diff citation-domain rank between the newest and oldest snapshot in window",
		Example: strings.Trim(`
  surgegraph-pp-cli visibility citation-domains rank-shift --project proj_abc123 --window 30d
  surgegraph-pp-cli visibility citation-domains rank-shift --project proj_abc123 --agent
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if projectID == "" {
				if dryRunOK(flags) {
					return nil
				}
				return errors.New("--project is required")
			}
			windowDur, err := cliutil.ParseDayDuration(windowRaw)
			if err != nil {
				return fmt.Errorf("--window: %w", err)
			}
			st, err := store.Open("")
			if err != nil {
				return err
			}
			defer st.Close()
			cutoff := time.Now().UTC().Add(-windowDur)
			rows, err := st.CitationRanksSince(cmd.Context(), projectID, cutoff)
			if err != nil {
				return err
			}
			// Group by domain; rank within first snapshot and last snapshot.
			byDate := map[string]map[string]int{} // date -> domain -> count
			for _, r := range rows {
				if _, ok := byDate[r.SnapshotDate]; !ok {
					byDate[r.SnapshotDate] = map[string]int{}
				}
				byDate[r.SnapshotDate][r.Domain] = r.CitationCount
			}
			if len(byDate) < 2 {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"note": "need at least 2 distinct snapshot dates in the window — run sync on different days",
				}, flags)
			}
			dates := sortedKeys(byDate)
			oldest := dates[0]
			newest := dates[len(dates)-1]
			oldRank := rankMap(byDate[oldest])
			newRank := rankMap(byDate[newest])
			type entry struct {
				Domain     string `json:"domain"`
				NewerRank  int    `json:"newer_rank"`
				OlderRank  int    `json:"older_rank"`
				RankDelta  int    `json:"rank_delta"` // negative = improved (smaller rank)
				NewerCount int    `json:"newer_count"`
				OlderCount int    `json:"older_count"`
			}
			result := []entry{}
			seen := map[string]struct{}{}
			for d := range newRank {
				seen[d] = struct{}{}
			}
			for d := range oldRank {
				seen[d] = struct{}{}
			}
			for d := range seen {
				e := entry{Domain: d}
				if r, ok := newRank[d]; ok {
					e.NewerRank = r
					e.NewerCount = byDate[newest][d]
				}
				if r, ok := oldRank[d]; ok {
					e.OlderRank = r
					e.OlderCount = byDate[oldest][d]
				}
				e.RankDelta = e.NewerRank - e.OlderRank
				result = append(result, e)
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"oldest_date": oldest,
				"newest_date": newest,
				"rows":        result,
			}, flags)
		},
	}
	cmd.Flags().StringVar(&projectID, "project", "", "Project ID")
	cmd.Flags().StringVar(&windowRaw, "window", "30d", "Window for the rank diff (accepts 30d, 720h, etc.)")
	return cmd
}

// ---------- visibility portfolio ----------

func newVisibilityPortfolioCmd(flags *rootFlags) *cobra.Command {
	var windowRaw string
	cmd := &cobra.Command{
		Use:   "portfolio",
		Short: "Cross-project AI Visibility roll-up (agency view)",
		Long: strings.TrimSpace(`
Fans the visibility-delta engine across every project in the local
store. Useful when one organization owns multiple brands or one user
manages multiple clients; the SurgeGraph UI surfaces one project at a
time.
`),
		Example: strings.Trim(`
  surgegraph-pp-cli visibility portfolio --window 7d
  surgegraph-pp-cli visibility portfolio --agent
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			windowDur, err := cliutil.ParseDayDuration(windowRaw)
			if err != nil {
				return fmt.Errorf("--window: %w", err)
			}
			st, err := store.Open("")
			if err != nil {
				return err
			}
			defer st.Close()
			projects, err := st.ListProjects(cmd.Context())
			if err != nil {
				return err
			}
			type row struct {
				ProjectID string `json:"project_id"`
				Project   string `json:"project"`
				Brand     string `json:"brand"`
				Metric    string `json:"metric"`
				NewerDate string `json:"newer_date"`
				OlderDate string `json:"older_date,omitempty"`
				HasOlder  bool   `json:"has_older"`
			}
			result := []row{}
			for _, p := range projects {
				brand := pickBrandName(&p)
				for _, m := range []string{"overview", "trend", "sentiment", "traffic_summary"} {
					n, o, err := st.LatestSnapshotPair(cmd.Context(), p.ID, brand, m)
					if err != nil || n == nil {
						continue
					}
					r := row{ProjectID: p.ID, Project: p.Name, Brand: brand, Metric: m, NewerDate: n.SnapshotDate}
					// PATCH(greptile-5): respect --window. Same logic as visibility delta —
					// suppress the older snapshot when it falls outside the requested window
					// so the portfolio roll-up doesn't claim a months-old "delta".
					if o != nil {
						if d, perr := time.Parse("2006-01-02", o.SnapshotDate); perr == nil &&
							windowDur > 0 && time.Since(d) > windowDur {
							// drop older — outside window
						} else {
							r.OlderDate = o.SnapshotDate
							r.HasOlder = true
						}
					}
					result = append(result, r)
				}
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&windowRaw, "window", "7d", "Window (reserved; accepts 7d, 168h, etc.)")
	return cmd
}

// ---------- visibility traffic-citations ----------

func newVisibilityTrafficCitationsCmd(flags *rootFlags) *cobra.Command {
	var projectID string
	var limit int
	cmd := &cobra.Command{
		Use:   "traffic-citations",
		Short: "Join top traffic pages with the citation URLs that resolve to them",
		Example: strings.Trim(`
  surgegraph-pp-cli visibility traffic-citations --project proj_abc123 --agent
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if projectID == "" {
				if dryRunOK(flags) {
					return nil
				}
				return errors.New("--project is required")
			}
			st, err := store.Open("")
			if err != nil {
				return err
			}
			defer st.Close()
			pages, err := st.TopTrafficPages(cmd.Context(), projectID, limit)
			if err != nil {
				return err
			}
			// For each top page, scan citation_snapshots.raw_json for the URL.
			type cite struct {
				Domain string `json:"domain"`
				Date   string `json:"snapshot_date"`
			}
			type row struct {
				URL         string `json:"page_url"`
				AIVisits    int    `json:"ai_visits"`
				HumanVisits int    `json:"human_visits"`
				Citations   []cite `json:"citations"`
			}
			cites, err := st.CitationRanksSince(cmd.Context(), projectID, time.Now().UTC().AddDate(0, 0, -30))
			if err != nil {
				return err
			}
			// PATCH(greptile-9): parse the citation_snapshots raw JSON and
			// exact-match URL fields instead of substring-matching the entire
			// blob. The previous strings.Contains approach attributed citations
			// to the wrong page whenever a short page URL (e.g. example.com/blog)
			// appeared as a prefix of an unrelated citation URL
			// (example.com/blog/post-a) or as a substring inside an image src,
			// canonical tag, or author link embedded in the raw JSON.
			result := []row{}
			for _, p := range pages {
				r := row{URL: p.URL, AIVisits: p.AIVisits, HumanVisits: p.HumanVisits}
				target := normalizeCitationURL(p.URL)
				if target == "" {
					result = append(result, r)
					continue
				}
				for _, c := range cites {
					if citationJSONContainsURL(c.RawJSON, target) {
						r.Citations = append(r.Citations, cite{Domain: c.Domain, Date: c.SnapshotDate})
					}
				}
				result = append(result, r)
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&projectID, "project", "", "Project ID")
	cmd.Flags().IntVar(&limit, "limit", 50, "Max top-traffic pages to consider")
	return cmd
}

// ---------- shared local helpers ----------

func splitCSV(s string) []string {
	out := []string{}
	for _, p := range strings.Split(s, ",") {
		t := strings.TrimSpace(p)
		if t != "" {
			out = append(out, t)
		}
	}
	return out
}

func resolveProject(ctx context.Context, st *store.Store, id string) (*store.Project, error) {
	projects, err := st.ListProjects(ctx)
	if err != nil {
		return nil, err
	}
	for _, p := range projects {
		if p.ID == id {
			return &p, nil
		}
	}
	return nil, fmt.Errorf("project %q not in local store; run `surgegraph-pp-cli sync --project %s` first", id, id)
}

func pickBrandName(p *store.Project) string {
	if p.BrandName != "" {
		return p.BrandName
	}
	return p.Name
}

func sortedKeys(m map[string]map[string]int) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	// Simple insertion sort; len(dates) is small.
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j-1] > out[j]; j-- {
			out[j-1], out[j] = out[j], out[j-1]
		}
	}
	return out
}

// normalizeCitationURL lowercases and trims a URL so that exact comparison
// is resilient to inconsequential casing or trailing-slash differences.
// Returns "" for empty input so callers can short-circuit.
func normalizeCitationURL(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	s = strings.TrimRight(s, "/")
	return strings.ToLower(s)
}

// citationJSONContainsURL recursively walks a citation_snapshots.raw_json
// payload and returns true only when some string value exactly matches the
// (already normalized) target URL. This avoids the false positives that a
// raw substring search produced when the page URL was a prefix of a longer
// citation URL or appeared inside an image src / canonical tag / author
// link embedded in the raw JSON blob.
func citationJSONContainsURL(raw json.RawMessage, target string) bool {
	if len(raw) == 0 || target == "" {
		return false
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return false
	}
	return citationJSONWalk(v, target)
}

func citationJSONWalk(v any, target string) bool {
	switch x := v.(type) {
	case string:
		return normalizeCitationURL(x) == target
	case map[string]any:
		for _, vv := range x {
			if citationJSONWalk(vv, target) {
				return true
			}
		}
	case []any:
		for _, item := range x {
			if citationJSONWalk(item, target) {
				return true
			}
		}
	}
	return false
}

func rankMap(counts map[string]int) map[string]int {
	type kv struct {
		k string
		v int
	}
	pairs := make([]kv, 0, len(counts))
	for k, v := range counts {
		pairs = append(pairs, kv{k, v})
	}
	// Descending sort by count.
	for i := 1; i < len(pairs); i++ {
		for j := i; j > 0 && pairs[j-1].v < pairs[j].v; j-- {
			pairs[j-1], pairs[j] = pairs[j], pairs[j-1]
		}
	}
	out := make(map[string]int, len(pairs))
	for i, p := range pairs {
		out[p.k] = i + 1 // 1-indexed rank
	}
	return out
}
