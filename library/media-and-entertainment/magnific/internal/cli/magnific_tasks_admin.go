package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/magnific/internal/store"
)

func newMagnificTasksCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tasks",
		Short: "Operate on the local Magnific task ledger (stale, reconcile, list)",
		Long: `tasks operates on the local magnific_tasks ledger that every async
generation/upscale/edit dispatch writes to. Use 'stale' to surface leaked
tasks the poller never finished, 'reconcile' to re-poll them from the API
and update terminal state, and 'list' to inspect recent rows.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
	}
	cmd.AddCommand(newMagnificTasksStaleCmd(flags))
	cmd.AddCommand(newMagnificTasksReconcileCmd(flags))
	cmd.AddCommand(newMagnificTasksListCmd(flags))
	return cmd
}

func newMagnificTasksStaleCmd(flags *rootFlags) *cobra.Command {
	var sinceStr string
	var limit int
	cmd := &cobra.Command{
		Use:         "stale",
		Short:       "Show tasks still in non-terminal state past a threshold",
		Example:     "  magnific-pp-cli tasks stale --since 24h --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			dur, err := parseDurationFlag(sinceStr)
			if err != nil {
				return usageErr(fmt.Errorf("--since %q: %w", sinceStr, err))
			}
			cutoff := time.Now().Add(-dur).UTC().Format(time.RFC3339)
			db, err := store.OpenWithContext(ctx, defaultDBPath("magnific-pp-cli"))
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()
			if err := store.EnsureMagnificTables(ctx, db.DB()); err != nil {
				return fmt.Errorf("initializing magnific tables: %w", err)
			}
			rows, err := db.DB().QueryContext(ctx, `
				SELECT task_id, COALESCE(model,''), COALESCE(endpoint,''), COALESCE(status,''), COALESCE(updated_at,''), COALESCE(created_at,'')
				FROM magnific_tasks
				WHERE status NOT IN ('COMPLETED','DONE','SUCCESS','FAILED','ERROR','CANCELLED','CANCELED')
				  AND created_at <= ?
				ORDER BY created_at DESC LIMIT ?`, cutoff, limit)
			if err != nil {
				return fmt.Errorf("query: %w", err)
			}
			defer rows.Close()
			type row struct {
				TaskID    string `json:"task_id"`
				Model     string `json:"model"`
				Endpoint  string `json:"endpoint"`
				Status    string `json:"status"`
				UpdatedAt string `json:"updated_at"`
				CreatedAt string `json:"created_at"`
			}
			out := []row{}
			for rows.Next() {
				var r row
				var t, m, e, s, u, c sql.NullString
				if err := rows.Scan(&t, &m, &e, &s, &u, &c); err != nil {
					continue
				}
				r.TaskID = t.String
				r.Model = m.String
				r.Endpoint = e.String
				r.Status = s.String
				r.UpdatedAt = u.String
				r.CreatedAt = c.String
				out = append(out, r)
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&sinceStr, "since", "24h", "Threshold (e.g. 24h, 7d)")
	cmd.Flags().IntVar(&limit, "limit", 100, "Max rows")
	return cmd
}

func newMagnificTasksReconcileCmd(flags *rootFlags) *cobra.Command {
	var sinceStr string
	var limit int
	cmd := &cobra.Command{
		Use:     "reconcile",
		Short:   "Re-poll stale tasks via the API and update terminal state in the local store",
		Example: "  magnific-pp-cli tasks reconcile --since 24h --json",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			dur, err := parseDurationFlag(sinceStr)
			if err != nil {
				return usageErr(fmt.Errorf("--since %q: %w", sinceStr, err))
			}
			cutoff := time.Now().Add(-dur).UTC().Format(time.RFC3339)
			db, err := store.OpenWithContext(ctx, defaultDBPath("magnific-pp-cli"))
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()
			if err := store.EnsureMagnificTables(ctx, db.DB()); err != nil {
				return fmt.Errorf("initializing magnific tables: %w", err)
			}
			rows, err := db.DB().QueryContext(ctx, `
				SELECT task_id, COALESCE(model,''), COALESCE(endpoint,''), COALESCE(status,'')
				FROM magnific_tasks
				WHERE status NOT IN ('COMPLETED','DONE','SUCCESS','FAILED','ERROR','CANCELLED','CANCELED')
				  AND created_at <= ?
				ORDER BY created_at DESC LIMIT ?`, cutoff, limit)
			if err != nil {
				return fmt.Errorf("query: %w", err)
			}
			type pending struct {
				taskID, model, endpoint string
			}
			var pendings []pending
			for rows.Next() {
				var p pending
				var t, m, e, _s sql.NullString
				if err := rows.Scan(&t, &m, &e, &_s); err != nil {
					continue
				}
				p.taskID = t.String
				p.model = m.String
				p.endpoint = e.String
				pendings = append(pendings, p)
			}
			rows.Close()

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			type res struct {
				TaskID  string `json:"task_id"`
				Model   string `json:"model"`
				Status  string `json:"status"`
				Updated bool   `json:"updated"`
				Error   string `json:"error,omitempty"`
			}
			out := []res{}
			for _, p := range pendings {
				getPath := strings.TrimSuffix(p.endpoint, "/") + "/" + p.taskID
				body, gerr := c.Get(getPath, nil)
				r := res{TaskID: p.taskID, Model: p.model}
				if gerr != nil {
					r.Error = gerr.Error()
					out = append(out, r)
					continue
				}
				status := extractTaskStatus(body)
				r.Status = status
				_ = updateTaskRow(ctx, db.DB(), p.taskID, status, body)
				r.Updated = true
				out = append(out, r)
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&sinceStr, "since", "24h", "Only reconcile tasks created at least this long ago")
	cmd.Flags().IntVar(&limit, "limit", 50, "Max tasks to reconcile per run")
	return cmd
}

func newMagnificTasksListCmd(flags *rootFlags) *cobra.Command {
	var limit int
	var statusFilter string
	var modelFilter string
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List recent local task rows",
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
			where := []string{}
			args2 := []any{}
			if statusFilter != "" {
				where = append(where, "status = ?")
				args2 = append(args2, statusFilter)
			}
			if modelFilter != "" {
				where = append(where, "model = ?")
				args2 = append(args2, modelFilter)
			}
			args2 = append(args2, limit)
			whereStr := ""
			if len(where) > 0 {
				whereStr = "WHERE " + strings.Join(where, " AND ")
			}
			q := fmt.Sprintf(`
				SELECT task_id, COALESCE(model,''), COALESCE(status,''), COALESCE(created_at,''),
					COALESCE(credit_cost, 0), COALESCE(output_url,'')
				FROM magnific_tasks %s
				ORDER BY created_at DESC LIMIT ?`, whereStr)
			rows, err := db.DB().QueryContext(ctx, q, args2...)
			if err != nil {
				return fmt.Errorf("query: %w", err)
			}
			defer rows.Close()
			type row struct {
				TaskID    string  `json:"task_id"`
				Model     string  `json:"model"`
				Status    string  `json:"status"`
				CreatedAt string  `json:"created_at"`
				Cost      float64 `json:"credit_cost,omitempty"`
				OutputURL string  `json:"output_url,omitempty"`
			}
			out := []row{}
			for rows.Next() {
				var r row
				var t, m, s, c, u sql.NullString
				var cost sql.NullFloat64
				if err := rows.Scan(&t, &m, &s, &c, &cost, &u); err != nil {
					continue
				}
				r.TaskID = t.String
				r.Model = m.String
				r.Status = s.String
				r.CreatedAt = c.String
				r.Cost = cost.Float64
				r.OutputURL = u.String
				out = append(out, r)
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 50, "Max rows")
	cmd.Flags().StringVar(&statusFilter, "status", "", "Filter by status (COMPLETED, IN_PROGRESS, etc)")
	cmd.Flags().StringVar(&modelFilter, "model", "", "Filter by model slug")
	return cmd
}

// avoid unused import warnings if a future refactor drops references
var _ = json.RawMessage(nil)
