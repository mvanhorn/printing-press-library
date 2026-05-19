package cli

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/magnific/internal/store"
)

func newMagnificHistoryCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "history",
		Short: "Browse your Magnific generation history (prompts, tasks, costs)",
		Long: `history is the entry point for offline queries over your Magnific
generation archive. Every dispatch writes a row to the local magnific_prompts
and magnific_tasks tables; this command provides FTS5 search over them.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
	}
	cmd.AddCommand(newMagnificHistorySearchCmd(flags))
	cmd.AddCommand(newMagnificHistoryListCmd(flags))
	return cmd
}

func newMagnificHistorySearchCmd(flags *rootFlags) *cobra.Command {
	var sinceStr string
	var modelFilter string
	var limit int
	cmd := &cobra.Command{
		Use:         "search [query]",
		Short:       "Full-text search every prompt you have ever sent to Magnific",
		Example:     "  magnific-pp-cli history search \"tokyo neon\" --since 30d --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			query := ""
			if len(args) > 0 {
				query = strings.TrimSpace(strings.Join(args, " "))
			}
			if query == "" {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			db, err := store.OpenWithContext(ctx, defaultDBPath("magnific-pp-cli"))
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()
			if err := store.EnsureMagnificTables(ctx, db.DB()); err != nil {
				return fmt.Errorf("initializing magnific tables: %w", err)
			}

			where := []string{"magnific_prompts_fts MATCH ?"}
			args2 := []any{query}
			if modelFilter != "" {
				where = append(where, "p.model = ?")
				args2 = append(args2, modelFilter)
			}
			if sinceStr != "" {
				dur, err := parseDurationFlag(sinceStr)
				if err != nil {
					return usageErr(fmt.Errorf("--since %q: %w", sinceStr, err))
				}
				cutoff := time.Now().Add(-dur).UTC().Format(time.RFC3339)
				where = append(where, "p.created_at >= ?")
				args2 = append(args2, cutoff)
			}
			args2 = append(args2, limit)

			q := fmt.Sprintf(`
				SELECT p.prompt, COALESCE(p.model,''), COALESCE(p.created_at,''),
					COALESCE(p.task_id,''),
					COALESCE(t.credit_cost, 0), COALESCE(t.output_url, ''), COALESCE(t.status,'')
				FROM magnific_prompts p
				JOIN magnific_prompts_fts fts ON fts.rowid = p.id
				LEFT JOIN magnific_tasks t ON t.task_id = p.task_id
				WHERE %s
				ORDER BY p.created_at DESC
				LIMIT ?`, strings.Join(where, " AND "))
			rows, err := db.DB().QueryContext(ctx, q, args2...)
			if err != nil {
				return fmt.Errorf("query: %w", err)
			}
			defer rows.Close()

			type hit struct {
				Prompt    string  `json:"prompt"`
				Model     string  `json:"model"`
				CreatedAt string  `json:"created_at"`
				TaskID    string  `json:"task_id,omitempty"`
				Cost      float64 `json:"credit_cost,omitempty"`
				OutputURL string  `json:"output_url,omitempty"`
				Status    string  `json:"status,omitempty"`
			}
			results := []hit{}
			for rows.Next() {
				var h hit
				var prompt, model, created, tid, ou, status sql.NullString
				var cost sql.NullFloat64
				if err := rows.Scan(&prompt, &model, &created, &tid, &cost, &ou, &status); err != nil {
					continue
				}
				h.Prompt = prompt.String
				h.Model = model.String
				h.CreatedAt = created.String
				h.TaskID = tid.String
				h.Cost = cost.Float64
				h.OutputURL = ou.String
				h.Status = status.String
				results = append(results, h)
			}
			return printJSONFiltered(cmd.OutOrStdout(), results, flags)
		},
	}
	cmd.Flags().StringVar(&sinceStr, "since", "", "Only return prompts newer than this duration (e.g. 7d, 24h)")
	cmd.Flags().StringVar(&modelFilter, "model", "", "Filter by model slug")
	cmd.Flags().IntVar(&limit, "limit", 50, "Max results")
	return cmd
}

func newMagnificHistoryListCmd(flags *rootFlags) *cobra.Command {
	var limit int
	var modelFilter string
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List the most recent prompts you have sent to Magnific",
		Example:     "  magnific-pp-cli history list --limit 20 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			db, err := store.OpenWithContext(ctx, defaultDBPath("magnific-pp-cli"))
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()
			if err := store.EnsureMagnificTables(ctx, db.DB()); err != nil {
				return fmt.Errorf("initializing magnific tables: %w", err)
			}
			where := ""
			args2 := []any{}
			if modelFilter != "" {
				where = "WHERE model = ?"
				args2 = append(args2, modelFilter)
			}
			args2 = append(args2, limit)
			q := fmt.Sprintf(`
				SELECT prompt, COALESCE(model,''), COALESCE(created_at,''), COALESCE(task_id,'')
				FROM magnific_prompts %s
				ORDER BY created_at DESC LIMIT ?`, where)
			rows, err := db.DB().QueryContext(ctx, q, args2...)
			if err != nil {
				return fmt.Errorf("query: %w", err)
			}
			defer rows.Close()
			type row struct {
				Prompt    string `json:"prompt"`
				Model     string `json:"model"`
				CreatedAt string `json:"created_at"`
				TaskID    string `json:"task_id,omitempty"`
			}
			out := []row{}
			for rows.Next() {
				var r row
				var p, m, c, t sql.NullString
				if err := rows.Scan(&p, &m, &c, &t); err != nil {
					continue
				}
				r.Prompt = p.String
				r.Model = m.String
				r.CreatedAt = c.String
				r.TaskID = t.String
				out = append(out, r)
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 20, "Max results")
	cmd.Flags().StringVar(&modelFilter, "model", "", "Filter by model slug")
	return cmd
}

// parseDurationFlag handles 7d / 24h / 30m / 90d shorthand.
func parseDurationFlag(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty")
	}
	if strings.HasSuffix(s, "d") {
		n := strings.TrimSuffix(s, "d")
		var days int
		if _, err := fmt.Sscanf(n, "%d", &days); err != nil {
			return 0, fmt.Errorf("not a number: %s", n)
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}
	return time.ParseDuration(s)
}
