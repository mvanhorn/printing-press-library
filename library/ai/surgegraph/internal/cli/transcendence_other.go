package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/ai/surgegraph/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/ai/surgegraph/internal/store"

	"github.com/spf13/cobra"
)

// ---------- docs stale ----------

func newDocsTranscendCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "docs",
		Short: "Document transcendence subcommands (stale projection)",
	}
	cmd.AddCommand(newDocsStaleCmd(flags))
	return cmd
}

func newDocsStaleCmd(flags *rootFlags) *cobra.Command {
	var projectID string
	var olderThan int
	cmd := &cobra.Command{
		Use:   "stale",
		Short: "List writer/optimized docs not updated in N days, ranked by AI traffic",
		Example: strings.Trim(`
  surgegraph-pp-cli docs stale --project proj_abc123 --older-than 90 --agent
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
			rows, err := st.StaleDocs(cmd.Context(), projectID, olderThan)
			if err != nil {
				return err
			}
			type row struct {
				ID           string    `json:"document_id"`
				Project      string    `json:"project_id"`
				Kind         string    `json:"kind"`
				Title        string    `json:"title"`
				LastUpdated  time.Time `json:"last_updated"`
				AITraffic30d int       `json:"ai_traffic_30d"`
			}
			out := make([]row, 0, len(rows))
			for _, r := range rows {
				out = append(out, row{ID: r.ID, Project: r.ProjectID, Kind: r.Kind, Title: r.Title, LastUpdated: r.UpdatedAt, AITraffic30d: r.AITraffic30d})
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&projectID, "project", "", "Project ID")
	cmd.Flags().IntVar(&olderThan, "older-than", 90, "Stale threshold in days")
	return cmd
}

// ---------- knowledge impact ----------

func newKnowledgeTranscendCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "knowledge",
		Short: "Knowledge-library transcendence subcommands (impact)",
	}
	cmd.AddCommand(newKnowledgeImpactCmd(flags))
	return cmd
}

func newKnowledgeImpactCmd(flags *rootFlags) *cobra.Command {
	var projectID string
	cmd := &cobra.Command{
		Use:   "impact",
		Short: "Rank knowledge libraries by how often their URLs appear in tracked AI citations",
		Example: strings.Trim(`
  surgegraph-pp-cli knowledge impact --project proj_abc123 --agent
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
			rows, err := st.KnowledgeImpact(cmd.Context(), projectID)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
		},
	}
	cmd.Flags().StringVar(&projectID, "project", "", "Project ID")
	return cmd
}

// ---------- account burn ----------

func newAccountTranscendCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "account",
		Short: "Account transcendence subcommands (burn-down forecast)",
	}
	cmd.AddCommand(newAccountBurnCmd(flags))
	return cmd
}

func newAccountBurnCmd(flags *rootFlags) *cobra.Command {
	var windowRaw string
	cmd := &cobra.Command{
		Use:   "burn",
		Short: "Project credit exhaustion from local usage history (preflight for bulk operations)",
		Example: strings.Trim(`
  surgegraph-pp-cli account burn --window 30d --agent
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
			cutoff := time.Now().UTC().Add(-windowDur)
			rows, err := st.UsageHistory(cmd.Context(), cutoff)
			if err != nil {
				return err
			}
			if len(rows) < 2 {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"note": "need at least 2 usage snapshots — run `surgegraph-pp-cli sync --include usage` on different days",
					"rows": rows,
				}, flags)
			}
			first := rows[0]
			last := rows[len(rows)-1]
			daysObserved := dayDiff(first.Date, last.Date)
			if daysObserved <= 0 {
				daysObserved = 1
			}
			usedFirst := first.CreditsUsed.Int64
			usedLast := last.CreditsUsed.Int64
			perDay := float64(usedLast-usedFirst) / float64(daysObserved)
			var depleteIn float64
			if perDay > 0 && last.CreditsRemaining.Valid {
				depleteIn = float64(last.CreditsRemaining.Int64) / perDay
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"window":           windowDur.String(),
				"days_observed":    daysObserved,
				"credits_per_day":  perDay,
				"current_used":     usedLast,
				"current_remain":   last.CreditsRemaining.Int64,
				"depletes_in_days": depleteIn,
				"first_snapshot":   first.Date,
				"last_snapshot":    last.Date,
			}, flags)
		},
	}
	cmd.Flags().StringVar(&windowRaw, "window", "30d", "History window (accepts 30d, 720h, etc.)")
	return cmd
}

func dayDiff(start, end string) int {
	a, errA := time.Parse("2006-01-02", start)
	b, errB := time.Parse("2006-01-02", end)
	if errA != nil || errB != nil {
		return 0
	}
	return int(b.Sub(a).Hours() / 24)
}

// ---------- context bundle ----------

func newContextBundleCmd(flags *rootFlags) *cobra.Command {
	var projectID, include string
	cmd := &cobra.Command{
		Use:   "context",
		Short: "Bundle SurgeGraph project state into one JSON for an agent's context window",
	}
	cmd.AddCommand(newContextBundleLeaf(flags, &projectID, &include))
	return cmd
}

func newContextBundleLeaf(flags *rootFlags, projectIDPtr, includePtr *string) *cobra.Command {
	var projectID, include string
	cmd := &cobra.Command{
		Use:   "bundle",
		Short: "Emit a JSON bundle of project state (prompts, citations, docs, topics)",
		Example: strings.Trim(`
  surgegraph-pp-cli context bundle --project proj_abc123 --include prompts,citations,docs,topics --agent
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
			scopes := normalizeIncludes(include)
			if len(scopes) == 0 {
				scopes = map[string]bool{"prompts": true, "citations": true, "docs": true, "topics": true}
			}
			bundle := map[string]any{
				"project_id":   projectID,
				"generated_at": time.Now().UTC().Format(time.RFC3339),
			}
			ctx := cmd.Context()
			if scopes["prompts"] {
				bundle["prompts"] = mustQueryRaw(ctx, st, "SELECT id, project_id, text, raw_json FROM prompts WHERE project_id = ? LIMIT 50", projectID)
			}
			if scopes["citations"] {
				bundle["citations"] = mustQueryRaw(ctx, st, "SELECT domain, citation_count, snapshot_date FROM citation_snapshots WHERE project_id = ? ORDER BY snapshot_date DESC, citation_count DESC LIMIT 50", projectID)
			}
			if scopes["docs"] {
				bundle["docs"] = mustQueryRaw(ctx, st, "SELECT id, kind, title, updated_at FROM documents WHERE project_id = ? LIMIT 50", projectID)
			}
			if scopes["topics"] {
				bundle["topic_researches"] = mustQueryRaw(ctx, st, "SELECT id, seed_topic, status FROM topic_researches WHERE project_id = ? LIMIT 25", projectID)
				bundle["domain_researches"] = mustQueryRaw(ctx, st, "SELECT id, domain, status FROM domain_researches WHERE project_id = ? LIMIT 25", projectID)
			}
			return printJSONFiltered(cmd.OutOrStdout(), bundle, flags)
		},
	}
	cmd.Flags().StringVar(&projectID, "project", "", "Project ID")
	cmd.Flags().StringVar(&include, "include", "prompts,citations,docs,topics", "Comma-separated scopes")
	_ = projectIDPtr
	_ = includePtr
	return cmd
}

func mustQueryRaw(ctx interface{}, st *store.Store, query string, args ...any) []map[string]any {
	rows, err := st.DB.Query(query, args...)
	if err != nil {
		return nil
	}
	defer rows.Close()
	cols, _ := rows.Columns()
	out := []map[string]any{}
	for rows.Next() {
		dst := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range dst {
			ptrs[i] = &dst[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			continue
		}
		row := map[string]any{}
		for i, c := range cols {
			row[c] = dst[i]
		}
		out = append(out, row)
	}
	return out
}

// ---------- search ----------

func newSearchCmd(flags *rootFlags) *cobra.Command {
	var kindList string
	var limit int
	cmd := &cobra.Command{
		Use:   "search [query]",
		Short: "Full-text search across local cache (prompts + citations + docs + topics)",
		Long: strings.TrimSpace(`
SQLite FTS5 across cached prompts, citation URLs/snippets, document
titles, and topic-map nodes. Run 'sync' first to populate the index.
The shipped 'sql' and 'search' framework tools also remain available
for raw SQL queries against the same store.
`),
		Example: strings.Trim(`
  surgegraph-pp-cli search "AI search optimization" --kind prompts,citations,docs,topics --agent
  surgegraph-pp-cli search "buyer intent" --limit 10 --agent --select kind,id,title,snippet
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			query := strings.Join(args, " ")
			st, err := store.Open("")
			if err != nil {
				return err
			}
			defer st.Close()
			kinds := splitCSV(kindList)
			hits, err := st.Search(cmd.Context(), query, kinds, limit)
			if err != nil {
				return fmt.Errorf("search %q: %w", query, err)
			}
			return printJSONFiltered(cmd.OutOrStdout(), hits, flags)
		},
	}
	cmd.Flags().StringVar(&kindList, "kind", "", "Restrict to kind set (prompt,citation,doc,topic,traffic)")
	cmd.Flags().IntVar(&limit, "limit", 25, "Max hits")
	return cmd
}

// ---------- sync diff ----------

func newSyncDiffCmd(flags *rootFlags) *cobra.Command {
	var since string
	cmd := &cobra.Command{
		Use:   "diff",
		Short: "Show per-resource row counts and last-sync timestamps",
		Long: strings.TrimSpace(`
Lists per-resource cursors and total row counts. Foundation primitive
for agent loops that want to act only on what's new since their last
run.
`),
		Example: strings.Trim(`
  surgegraph-pp-cli sync diff --agent
  surgegraph-pp-cli sync diff --since 2026-05-05T00:00:00Z --agent
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := store.Open("")
			if err != nil {
				return err
			}
			defer st.Close()
			t := time.Time{}
			if since != "" {
				if parsed, ok := parseTime(since); ok {
					t = parsed
				}
			}
			rows, err := st.SyncDiff(cmd.Context(), t)
			if err != nil {
				return err
			}
			cursors, _ := st.Cursors(cmd.Context())
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"rows":       rows,
				"cursors":    cursors,
				"store_path": st.Path,
			}, flags)
		},
	}
	cmd.Flags().StringVar(&since, "since", "", "RFC3339 timestamp filter (reserved — currently informational)")
	return cmd
}

// Attach "diff" to the existing sync command tree.
//
// The generated CLI does not have a sync parent (we added one in sync.go).
// To keep the public surface clean, we attach sync diff as a subcommand of
// the same parent.
func wireSyncDiff(syncParent *cobra.Command, flags *rootFlags) {
	syncParent.AddCommand(newSyncDiffCmd(flags))
}

// ---------- shared decoder fallback (re-used by docs stale tests if added) ----------

// dump exists to silence unused-import warnings if a future refactor
// removes the json reference; keep it as a no-op until then.
var _ = json.RawMessage(nil)
