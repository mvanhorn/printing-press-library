package cli

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/magnific/internal/store"
)

func newMagnificTaskCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "task",
		Short: "Manage a single Magnific async task by id (uniform across 80+ model families)",
		Long: `task resolves any Magnific task_id back to the model endpoint that
created it and operates on it (wait, watch, status). The local
magnific_tasks table maps task_id → endpoint, so the same wait/watch logic
works whether you submitted to Mystic, a Kling video model, or the
upscaler.`,
	}
	cmd.AddCommand(newMagnificTaskWaitCmd(flags))
	cmd.AddCommand(newMagnificTaskWatchCmd(flags))
	cmd.AddCommand(newMagnificTaskStatusCmd(flags))
	return cmd
}

// resolveTaskEndpoint reads the magnific_tasks row for the task_id and
// computes the GET path. The spec uses /v1/ai/<model-prefix>/<task-id> for
// every async resource so the stored "endpoint" column is the POST path
// (e.g. /v1/ai/mystic). We rewrite it to the matching task-GET path.
func resolveTaskEndpoint(ctx interface{ Deadline() (time.Time, bool) }, db *sql.DB, taskID string) (model, getPath string, err error) {
	row := db.QueryRow(`SELECT COALESCE(model,''), COALESCE(endpoint,'') FROM magnific_tasks WHERE task_id = ?`, taskID)
	var m, ep sql.NullString
	if err := row.Scan(&m, &ep); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", "", fmt.Errorf("task %q not found in local store; submit it via this CLI first or run `magnific-pp-cli tasks reconcile`", taskID)
		}
		return "", "", err
	}
	getPath = strings.TrimSuffix(ep.String, "/") + "/" + taskID
	return m.String, getPath, nil
}

func newMagnificTaskWaitCmd(flags *rootFlags) *cobra.Command {
	var timeoutStr string
	var intervalStr string
	cmd := &cobra.Command{
		Use:   "wait <task-id>",
		Short: "Block until a Magnific task reaches a terminal state",
		Long: `wait polls the model-specific GET endpoint for <task-id> with an
adaptive backoff (starting at the --interval value, capped at 15s) until
the task reports COMPLETED or FAILED. The task must have been submitted
through this CLI so the model→endpoint mapping is in the local store.`,
		Example: "  magnific-pp-cli task wait task-7f3a-... --timeout 5m --json",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			taskID := args[0]
			ctx := cmd.Context()
			timeout, err := time.ParseDuration(timeoutStr)
			if err != nil {
				return usageErr(fmt.Errorf("--timeout %q: %w", timeoutStr, err))
			}
			interval, err := time.ParseDuration(intervalStr)
			if err != nil {
				return usageErr(fmt.Errorf("--interval %q: %w", intervalStr, err))
			}
			db, err := store.OpenWithContext(ctx, defaultDBPath("magnific-pp-cli"))
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()
			if err := store.EnsureMagnificTables(ctx, db.DB()); err != nil {
				return fmt.Errorf("initializing magnific tables: %w", err)
			}
			_, getPath, err := resolveTaskEndpoint(ctx, db.DB(), taskID)
			if err != nil {
				return err
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			deadline := time.Now().Add(timeout)
			step := interval
			max := 15 * time.Second
			var lastBody json.RawMessage
			var lastStatus string
			for time.Now().Before(deadline) {
				body, gerr := c.Get(getPath, nil)
				if gerr != nil {
					return fmt.Errorf("polling task: %w", gerr)
				}
				lastBody = body
				lastStatus = extractTaskStatus(body)
				_ = updateTaskRow(ctx, db.DB(), taskID, lastStatus, body)
				if isTerminalStatus(lastStatus) {
					out := map[string]any{
						"task_id": taskID,
						"status":  lastStatus,
						"body":    json.RawMessage(body),
					}
					return printJSONFiltered(cmd.OutOrStdout(), out, flags)
				}
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(step):
				}
				if step < max {
					step = time.Duration(float64(step) * 1.5)
					if step > max {
						step = max
					}
				}
			}
			return fmt.Errorf("timeout after %s waiting for task %s (last status: %s, body: %s)", timeout, taskID, lastStatus, string(lastBody))
		},
	}
	cmd.Flags().StringVar(&timeoutStr, "timeout", "5m", "Maximum total wait time (e.g. 5m, 30s)")
	cmd.Flags().StringVar(&intervalStr, "interval", "3s", "Initial poll interval (adaptive backoff applies)")
	return cmd
}

func newMagnificTaskWatchCmd(flags *rootFlags) *cobra.Command {
	var intervalStr string
	var iterations int
	cmd := &cobra.Command{
		Use:     "watch <task-id>",
		Short:   "Stream JSON-line status updates for a Magnific task until terminal",
		Example: "  magnific-pp-cli task watch task-7f3a-... --interval 3s",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			taskID := args[0]
			ctx := cmd.Context()
			interval, err := time.ParseDuration(intervalStr)
			if err != nil {
				return usageErr(fmt.Errorf("--interval %q: %w", intervalStr, err))
			}
			db, err := store.OpenWithContext(ctx, defaultDBPath("magnific-pp-cli"))
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()
			if err := store.EnsureMagnificTables(ctx, db.DB()); err != nil {
				return fmt.Errorf("initializing magnific tables: %w", err)
			}
			_, getPath, err := resolveTaskEndpoint(ctx, db.DB(), taskID)
			if err != nil {
				return err
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			enc := json.NewEncoder(cmd.OutOrStdout())
			for i := 0; iterations == 0 || i < iterations; i++ {
				body, gerr := c.Get(getPath, nil)
				if gerr != nil {
					return fmt.Errorf("polling task: %w", gerr)
				}
				status := extractTaskStatus(body)
				_ = updateTaskRow(ctx, db.DB(), taskID, status, body)
				_ = enc.Encode(map[string]any{
					"timestamp": time.Now().UTC().Format(time.RFC3339),
					"task_id":   taskID,
					"status":    status,
				})
				if isTerminalStatus(status) {
					return nil
				}
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(interval):
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&intervalStr, "interval", "3s", "Poll interval")
	cmd.Flags().IntVar(&iterations, "iterations", 0, "Stop after N polls (0 = unlimited)")
	return cmd
}

func newMagnificTaskStatusCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "status <task-id>",
		Short:       "Print the latest status (local cache, then live re-poll)",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			taskID := args[0]
			db, err := store.OpenWithContext(ctx, defaultDBPath("magnific-pp-cli"))
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()
			if err := store.EnsureMagnificTables(ctx, db.DB()); err != nil {
				return fmt.Errorf("initializing magnific tables: %w", err)
			}
			_, getPath, err := resolveTaskEndpoint(ctx, db.DB(), taskID)
			if err != nil {
				return err
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			body, gerr := c.Get(getPath, nil)
			if gerr != nil {
				return fmt.Errorf("polling task: %w", gerr)
			}
			status := extractTaskStatus(body)
			_ = updateTaskRow(ctx, db.DB(), taskID, status, body)
			out := map[string]any{
				"task_id": taskID,
				"status":  status,
				"body":    json.RawMessage(body),
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	return cmd
}

// extractTaskStatus parses Magnific's task envelope. Common shapes:
//
//	{"data": {"status": "COMPLETED", ...}} or {"status": "..."}
func extractTaskStatus(body []byte) string {
	var env struct {
		Data struct {
			Status string `json:"status"`
		} `json:"data"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return "UNKNOWN"
	}
	if env.Data.Status != "" {
		return env.Data.Status
	}
	if env.Status != "" {
		return env.Status
	}
	return "UNKNOWN"
}

func isTerminalStatus(status string) bool {
	switch strings.ToUpper(status) {
	case "COMPLETED", "DONE", "SUCCESS", "FAILED", "ERROR", "CANCELLED", "CANCELED":
		return true
	}
	return false
}

func updateTaskRow(ctx interface{ Done() <-chan struct{} }, db *sql.DB, taskID, status string, body []byte) error {
	// Best-effort upsert; ignore errors from the trigger surface.
	// Pull output URL if present in body for terminal states.
	outputURL := ""
	if isTerminalStatus(status) {
		var env struct {
			Data struct {
				URL    string   `json:"url"`
				Output string   `json:"output"`
				Images []string `json:"images"`
			} `json:"data"`
		}
		if err := json.Unmarshal(body, &env); err == nil {
			switch {
			case env.Data.URL != "":
				outputURL = env.Data.URL
			case env.Data.Output != "":
				outputURL = env.Data.Output
			case len(env.Data.Images) > 0:
				outputURL = env.Data.Images[0]
			}
		}
	}
	if isTerminalStatus(status) {
		_, err := db.Exec(`UPDATE magnific_tasks SET status=?, updated_at=CURRENT_TIMESTAMP, completed_at=CURRENT_TIMESTAMP, output_url=COALESCE(NULLIF(?,''), output_url) WHERE task_id=?`, status, outputURL, taskID)
		return err
	}
	_, err := db.Exec(`UPDATE magnific_tasks SET status=?, updated_at=CURRENT_TIMESTAMP WHERE task_id=?`, status, taskID)
	return err
}
