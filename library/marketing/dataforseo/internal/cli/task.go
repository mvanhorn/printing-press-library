// Copyright 2026 mazzsterr. Licensed under Apache-2.0. See LICENSE.
// Hand-written transcendence command. Not generated.

package cli

import (
	"context"
	"github.com/mvanhorn/printing-press-library/library/marketing/dataforseo/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/marketing/dataforseo/internal/store"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

const taskLedgerSchema = `CREATE TABLE IF NOT EXISTS task_ledger (
  task_id TEXT PRIMARY KEY,
  endpoint TEXT NOT NULL,
  posted_at TIMESTAMP NOT NULL,
  status TEXT NOT NULL,
  cost REAL,
  fetched_at TIMESTAMP,
  result TEXT
)`

func ensureTaskLedger(db *sql.DB) error {
	_, err := db.Exec(taskLedgerSchema)
	return err
}

type taskLedgerRow struct {
	TaskID    string  `json:"task_id"`
	Endpoint  string  `json:"endpoint"`
	PostedAt  string  `json:"posted_at"`
	Status    string  `json:"status"`
	Cost      float64 `json:"cost"`
	FetchedAt string  `json:"fetched_at,omitempty"`
}

func newTaskCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "task",
		Short: "Run Standard-mode task lifecycles (post -> ready poll -> get -> merge)",
		Long: `DataForSEO Standard mode is ~3.3x cheaper than Live, but the lifecycle
requires posting, polling tasks_ready (rate-limited 20/min), then fetching
each ready task with task_get. This group runs the whole loop and persists
task IDs to local SQLite so a Ctrl-C or laptop sleep doesn't lose the queue.`,
	}
	cmd.AddCommand(newTaskBundleCmd(flags))
	cmd.AddCommand(newTaskLsCmd(flags))
	cmd.AddCommand(newTaskGetCmd(flags))
	return cmd
}

func newTaskBundleCmd(flags *rootFlags) *cobra.Command {
	var inFile string
	var pollInterval time.Duration

	cmd := &cobra.Command{
		Use:   "bundle <endpoint>",
		Short: "Post a batch of tasks, poll until ready, fetch results, merge into one output",
		Long: `Submits the JSON array in --in to the given Standard-mode endpoint, polls
tasks_ready respecting the 20/min limit, fetches each ready task, and prints
the merged results. Task IDs are persisted to the local SQLite task_ledger
table so 'task ls' shows in-flight work and 'task get' can recover a single
task post-crash.

Endpoint accepts kebab or slash form (e.g. 'serp/google/organic/task_post').`,
		Example: strings.Trim(`
  dataforseo-pp-cli task bundle serp/google/organic/task_post --in /tmp/batch.json
  dataforseo-pp-cli task bundle keywords_data/google_ads/search_volume/task_post --in batch.json --poll-interval 60s
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), "would submit task batch and poll until ready")
				return nil
			}
			if inFile == "" {
				return usageErr(fmt.Errorf("--in <batch.json> is required"))
			}

			endpoint := normalizeCostEndpoint(args[0])
			postPath := "/v3/" + endpoint

			data, err := os.ReadFile(inFile)
			if err != nil {
				return usageErr(fmt.Errorf("read %s: %w", inFile, err))
			}
			var batch []json.RawMessage
			if err := json.Unmarshal(data, &batch); err != nil {
				return usageErr(fmt.Errorf("parse %s as JSON array: %w", inFile, err))
			}
			if len(batch) == 0 {
				return usageErr(fmt.Errorf("%s: empty batch", inFile))
			}
			if len(batch) > 100 {
				return usageErr(fmt.Errorf("%s: %d items exceeds DataForSEO 100-tasks-per-post limit", inFile, len(batch)))
			}

			ctx := context.Background()
			s, err := store.OpenWithContext(ctx, defaultDBPath("dataforseo-pp-cli"))
			if err != nil {
				return err
			}
			defer s.Close()
			if err := ensureTaskLedger(s.DB()); err != nil {
				return err
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			postBody := make([]any, 0, len(batch))
			for _, b := range batch {
				var item any
				if err := json.Unmarshal(b, &item); err != nil {
					return usageErr(fmt.Errorf("invalid item in %s: %w", inFile, err))
				}
				postBody = append(postBody, item)
			}

			respRaw, _, err := c.Post(postPath, postBody)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			taskIDs, err := extractTaskIDs(respRaw)
			if err != nil {
				return err
			}
			if len(taskIDs) == 0 {
				return apiErr(fmt.Errorf("task_post returned no task_ids — check API response"))
			}

			now := time.Now().UTC().Format(time.RFC3339)
			for _, id := range taskIDs {
				_, _ = s.DB().Exec(
					`INSERT OR REPLACE INTO task_ledger (task_id, endpoint, posted_at, status) VALUES (?, ?, ?, ?)`,
					id, endpoint, now, "pending",
				)
			}

			// Family-derived tasks_ready path: replace trailing /task_post with /tasks_ready
			readyPath := strings.Replace(postPath, "/task_post", "/tasks_ready", 1)
			getPathBase := strings.Replace(postPath, "/task_post", "/task_get", 1)

			ready := map[string]bool{}
			limiter := cliutil.NewAdaptiveLimiter(20.0 / 60.0) // 20/min
			deadline := time.Now().Add(30 * time.Minute)
			for len(ready) < len(taskIDs) {
				if time.Now().After(deadline) {
					return apiErr(fmt.Errorf("timed out waiting for %d task(s); see 'task ls' to resume", len(taskIDs)-len(ready)))
				}
				limiter.Wait()
				readyResp, err := c.Get(readyPath, nil)
				if err != nil {
					var rerr *cliutil.RateLimitError
					if AsErr(err, &rerr) {
						return rerr
					}
					return classifyAPIError(err, flags)
				}
				for _, id := range extractReadyIDs(readyResp) {
					ready[id] = true
				}
				if len(ready) >= len(taskIDs) {
					break
				}
				time.Sleep(pollInterval)
			}

			merged := make([]json.RawMessage, 0, len(taskIDs))
			for _, id := range taskIDs {
				resp, _, err := c.Post(getPathBase+"/"+id, nil)
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: task_get %s failed: %v\n", id, err)
					continue
				}
				_, _ = s.DB().Exec(
					`UPDATE task_ledger SET status = ?, fetched_at = ?, result = ? WHERE task_id = ?`,
					"fetched", time.Now().UTC().Format(time.RFC3339), string(resp), id,
				)
				merged = append(merged, resp)
			}

			return printJSONFiltered(cmd.OutOrStdout(), merged, flags)
		},
	}
	cmd.Flags().StringVar(&inFile, "in", "", "JSON array file: one element per task")
	cmd.Flags().DurationVar(&pollInterval, "poll-interval", 30*time.Second, "Wait between tasks_ready polls")
	return cmd
}

// AsErr is errors.As exposed for in-package consumption without importing
// errors at every call site.
func AsErr(err error, target any) bool {
	return As(err, target)
}

// runStandardTaskLifecycle posts a task batch, polls tasks_ready until every
// task is fetchable, then fetches and merges each result. endpoint is the
// trailing path segment ending in /task_post (e.g.
// "keywords_data/google_ads/search_volume/task_post"). It persists task IDs
// to the local task_ledger SQLite table so 'task ls' can recover after a
// crash. Returns the merged raw JSON results in submission order.
//
// Extracted from newTaskBundleCmd so transcendence wrappers (keywords volume)
// can reuse the same loop without duplicating ~80 lines.
func runStandardTaskLifecycle(
	ctx context.Context,
	c clientPoster,
	endpoint string,
	postBody []any,
	pollInterval time.Duration,
	stderr writeFn,
) ([]json.RawMessage, error) {
	postPath := "/v3/" + endpoint
	s, err := store.OpenWithContext(ctx, defaultDBPath("dataforseo-pp-cli"))
	if err != nil {
		return nil, err
	}
	defer s.Close()
	if err := ensureTaskLedger(s.DB()); err != nil {
		return nil, err
	}

	respRaw, _, err := c.Post(postPath, postBody)
	if err != nil {
		return nil, err
	}

	taskIDs, err := extractTaskIDs(respRaw)
	if err != nil {
		return nil, err
	}
	if len(taskIDs) == 0 {
		return nil, apiErr(fmt.Errorf("task_post returned no task_ids — check API response"))
	}

	now := time.Now().UTC().Format(time.RFC3339)
	for _, id := range taskIDs {
		_, _ = s.DB().Exec(
			`INSERT OR REPLACE INTO task_ledger (task_id, endpoint, posted_at, status) VALUES (?, ?, ?, ?)`,
			id, endpoint, now, "pending",
		)
	}

	readyPath := strings.Replace(postPath, "/task_post", "/tasks_ready", 1)
	getPathBase := strings.Replace(postPath, "/task_post", "/task_get", 1)

	ready := map[string]bool{}
	limiter := cliutil.NewAdaptiveLimiter(20.0 / 60.0) // 20/min
	deadline := time.Now().Add(30 * time.Minute)
	for len(ready) < len(taskIDs) {
		if time.Now().After(deadline) {
			return nil, apiErr(fmt.Errorf("timed out waiting for %d task(s); see 'task ls' to resume", len(taskIDs)-len(ready)))
		}
		limiter.Wait()
		readyResp, err := c.Get(readyPath, nil)
		if err != nil {
			return nil, err
		}
		for _, id := range extractReadyIDs(readyResp) {
			ready[id] = true
		}
		if len(ready) >= len(taskIDs) {
			break
		}
		time.Sleep(pollInterval)
	}

	merged := make([]json.RawMessage, 0, len(taskIDs))
	for _, id := range taskIDs {
		resp, _, err := c.Post(getPathBase+"/"+id, nil)
		if err != nil {
			if stderr != nil {
				stderr(fmt.Sprintf("warning: task_get %s failed: %v\n", id, err))
			}
			continue
		}
		_, _ = s.DB().Exec(
			`UPDATE task_ledger SET status = ?, fetched_at = ?, result = ? WHERE task_id = ?`,
			"fetched", time.Now().UTC().Format(time.RFC3339), string(resp), id,
		)
		merged = append(merged, resp)
	}
	return merged, nil
}

// clientPoster is the minimal subset of *client.Client the lifecycle helper
// needs. Lets tests pass a stub without spinning up an HTTP server.
type clientPoster interface {
	Post(path string, body any) (json.RawMessage, int, error)
	Get(path string, params map[string]string) (json.RawMessage, error)
}

type writeFn func(string)

func extractTaskIDs(raw json.RawMessage) ([]string, error) {
	var resp struct {
		Tasks []struct {
			ID string `json:"id"`
		} `json:"tasks"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(resp.Tasks))
	for _, t := range resp.Tasks {
		if t.ID != "" {
			ids = append(ids, t.ID)
		}
	}
	return ids, nil
}

func extractReadyIDs(raw json.RawMessage) []string {
	var resp struct {
		Tasks []struct {
			Result []struct {
				ID string `json:"id"`
			} `json:"result"`
		} `json:"tasks"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil
	}
	var out []string
	for _, t := range resp.Tasks {
		for _, r := range t.Result {
			if r.ID != "" {
				out = append(out, r.ID)
			}
		}
	}
	return out
}

func newTaskLsCmd(flags *rootFlags) *cobra.Command {
	var statusFilter string
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List tasks tracked in the local task_ledger",
		Example: strings.Trim(`
  dataforseo-pp-cli task list
  dataforseo-pp-cli task list --status pending --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx := context.Background()
			s, err := store.OpenWithContext(ctx, defaultDBPath("dataforseo-pp-cli"))
			if err != nil {
				return err
			}
			defer s.Close()
			if err := ensureTaskLedger(s.DB()); err != nil {
				return err
			}

			q := `SELECT task_id, endpoint, posted_at, status, COALESCE(cost,0), COALESCE(fetched_at,'') FROM task_ledger`
			argsSQL := []any{}
			if statusFilter != "" {
				q += ` WHERE status = ?`
				argsSQL = append(argsSQL, statusFilter)
			}
			q += ` ORDER BY posted_at DESC LIMIT 200`
			rows, err := s.DB().Query(q, argsSQL...)
			if err != nil {
				return err
			}
			defer rows.Close()

			out := []taskLedgerRow{}
			for rows.Next() {
				var r taskLedgerRow
				if err := rows.Scan(&r.TaskID, &r.Endpoint, &r.PostedAt, &r.Status, &r.Cost, &r.FetchedAt); err != nil {
					return err
				}
				out = append(out, r)
			}
			if err := rows.Err(); err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&statusFilter, "status", "", "Filter by status: pending|ready|fetched")
	return cmd
}

func newTaskGetCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <task_id>",
		Short: "Fetch one task_id's result and update the local ledger",
		Example: strings.Trim(`
  dataforseo-pp-cli task get 07251516-1535-0066-0000-13b73d4f8ef0
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), "would fetch task", args[0])
				return nil
			}
			taskID := args[0]
			ctx := context.Background()
			s, err := store.OpenWithContext(ctx, defaultDBPath("dataforseo-pp-cli"))
			if err != nil {
				return err
			}
			defer s.Close()
			if err := ensureTaskLedger(s.DB()); err != nil {
				return err
			}

			var endpoint string
			err = s.DB().QueryRow(`SELECT endpoint FROM task_ledger WHERE task_id = ?`, taskID).Scan(&endpoint)
			if err == sql.ErrNoRows {
				return notFoundErr(fmt.Errorf("task %s not in local ledger", taskID))
			}
			if err != nil {
				return err
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			getPath := "/v3/" + strings.Replace(endpoint, "/task_post", "/task_get", 1) + "/" + taskID
			resp, _, err := c.Post(getPath, nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			_, _ = s.DB().Exec(
				`UPDATE task_ledger SET status = ?, fetched_at = ?, result = ? WHERE task_id = ?`,
				"fetched", time.Now().UTC().Format(time.RFC3339), string(resp), taskID,
			)
			return printJSONFiltered(cmd.OutOrStdout(), resp, flags)
		},
	}
	return cmd
}
