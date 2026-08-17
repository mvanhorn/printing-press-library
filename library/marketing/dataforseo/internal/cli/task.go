// Copyright 2026 mazzsterr. Licensed under Apache-2.0. See LICENSE.
// Hand-written transcendence command. Not generated.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	apiclient "github.com/mvanhorn/printing-press-library/library/marketing/dataforseo/internal/client"
	"github.com/mvanhorn/printing-press-library/library/marketing/dataforseo/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/marketing/dataforseo/internal/store"
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
		// PATCH: File-backed commands must not be exposed through MCP because the caller chooses --in.
		Annotations: map[string]string{"mcp:hidden": "true"},
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

			paths, err := resolveTaskPaths(cmd.Root(), postPath)
			if err != nil {
				return usageErr(err)
			}
			merged, err := runTaskLifecycleWithStore(ctx, c, s, paths, endpoint, postBody, pollInterval, func(message string) {
				fmt.Fprint(cmd.ErrOrStderr(), message)
			})
			if err != nil {
				return classifyAPIError(err, flags)
			}

			return printJSONFiltered(cmd.OutOrStdout(), merged, flags)
		},
	}
	cmd.Flags().StringVar(&inFile, "in", "", "JSON array file: one element per task")
	cmd.Flags().DurationVar(&pollInterval, "poll-interval", 30*time.Second, "Wait between tasks_ready polls")
	return cmd
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

	paths := defaultTaskPaths(postPath)
	return runTaskLifecycleWithStore(ctx, c, s, paths, endpoint, postBody, pollInterval, stderr)
}

type taskPaths struct {
	post  string
	ready string
	get   string
}

func defaultTaskPaths(postPath string) taskPaths {
	family := strings.TrimSuffix(postPath, "/task_post")
	return taskPaths{post: postPath, ready: family + "/tasks_ready", get: family + "/task_get/{id}"}
}

// PATCH: Some DataForSEO task families publish results outside task_get.
var taskResultPathOverrides = map[string]string{
	"/v3/on_page/task_post": "/v3/on_page/summary/{id}",
}

// resolveTaskPaths uses the generated command annotations as the source of
// truth for APIs whose task_get path includes an output format segment.
// PATCH: DataForSEO task results are GET endpoints and their shapes vary by
// family; deriving /task_get/<id> from task_post is not generally valid.
func resolveTaskPaths(root *cobra.Command, postPath string) (taskPaths, error) {
	paths := defaultTaskPaths(postPath)
	family := strings.TrimSuffix(postPath, "/task_post")
	getPath, hasOverride := taskResultPathOverrides[postPath]
	bestPriority := 0
	foundCandidate := false
	var visit func(*cobra.Command)
	visit = func(command *cobra.Command) {
		if command.Annotations != nil {
			path := command.Annotations["pp:path"]
			method := command.Annotations["pp:method"]
			if method == "GET" && path == family+"/tasks_ready" {
				paths.ready = path
			}
			if method == "GET" && strings.HasPrefix(path, family+"/task_get/") && strings.Contains(path, "{id}") {
				priority := taskGetPathPriority(path)
				// PATCH: Rank only discovered routes; the synthesized fallback is not an API candidate.
				if !foundCandidate || priority < bestPriority {
					getPath = path
					bestPriority = priority
					foundCandidate = true
				}
			}
		}
		for _, child := range command.Commands() {
			visit(child)
		}
	}
	visit(root)
	if !hasOverride && !foundCandidate {
		return taskPaths{}, fmt.Errorf("cannot resolve task result path for %s", postPath)
	}
	paths.get = getPath
	return paths, nil
}

func taskGetPathPriority(path string) int {
	switch {
	case strings.Contains(path, "/advanced/"):
		return 0
	case strings.Contains(path, "/regular/"):
		return 1
	case strings.HasSuffix(path, "/task_get/{id}"):
		return 2
	default:
		return 3
	}
}

func runTaskLifecycleWithStore(
	ctx context.Context,
	c clientPoster,
	s *store.Store,
	paths taskPaths,
	endpoint string,
	postBody []any,
	pollInterval time.Duration,
	stderr writeFn,
) ([]json.RawMessage, error) {
	respRaw, _, err := c.Post(paths.post, postBody)
	if err != nil {
		var partialErr *apiclient.DataForSEOError
		if !errors.As(err, &partialErr) || !partialErr.Partial || len(respRaw) == 0 {
			return nil, err
		}
	}
	taskIDs, rejected, err := extractTaskPostOutcome(respRaw)
	if err != nil {
		return nil, err
	}
	for _, rejection := range rejected {
		if stderr != nil {
			stderr(fmt.Sprintf("warning: task_post item %d rejected (status %d): %s\n", rejection.index+1, rejection.statusCode, rejection.statusMessage))
		}
	}
	if len(taskIDs) == 0 {
		if len(rejected) > 0 {
			return nil, apiErr(fmt.Errorf("task_post accepted no tasks: all %d task(s) were rejected", len(rejected)))
		}
		return nil, apiErr(fmt.Errorf("task_post accepted no tasks: response contained no task IDs"))
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	// PATCH: A billable post is recoverable only if every accepted ID reaches the ledger atomically.
	tx, err := s.DB().BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("starting task ledger transaction: %w", err)
	}
	for _, id := range taskIDs {
		if _, err := tx.ExecContext(ctx,
			`INSERT OR REPLACE INTO task_ledger (task_id, endpoint, posted_at, status) VALUES (?, ?, ?, ?)`,
			id, endpoint, now, "pending",
		); err != nil {
			_ = tx.Rollback()
			return nil, fmt.Errorf("recording task %s: %w", id, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("committing task ledger transaction: %w", err)
	}

	pending := make(map[string]struct{}, len(taskIDs))
	for _, id := range taskIDs {
		pending[id] = struct{}{}
	}
	limiter := cliutil.NewAdaptiveLimiter(20.0 / 60.0)
	deadline := time.Now().Add(30 * time.Minute)
	for len(pending) > 0 {
		if time.Now().After(deadline) {
			return nil, apiErr(fmt.Errorf("timed out waiting for %d task(s); see 'task ls' to resume", len(pending)))
		}
		limiter.Wait()
		readyResp, err := c.GetFresh(paths.ready, nil)
		if err != nil {
			return nil, err
		}
		for _, id := range extractReadyIDs(readyResp) {
			delete(pending, id)
		}
		if len(pending) == 0 {
			break
		}
		timer := time.NewTimer(pollInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}

	merged := make([]json.RawMessage, 0, len(taskIDs))
	for _, id := range taskIDs {
		getPath := replacePathParam(paths.get, "id", url.PathEscape(id))
		resp, err := c.GetFresh(getPath, nil)
		if err != nil {
			if stderr != nil {
				stderr(fmt.Sprintf("task_get %s failed: %v\n", id, err))
			}
			return nil, fmt.Errorf("fetching task %s: %w", id, err)
		}
		if _, err := s.DB().Exec(
			`UPDATE task_ledger SET status = ?, fetched_at = ?, result = ? WHERE task_id = ?`,
			"fetched", time.Now().UTC().Format(time.RFC3339Nano), string(resp), id,
		); err != nil {
			return nil, fmt.Errorf("updating task %s: %w", id, err)
		}
		merged = append(merged, resp)
	}
	return merged, nil
}

// clientPoster is the minimal subset of *client.Client the lifecycle helper
// needs. Lets tests pass a stub without spinning up an HTTP server.
type clientPoster interface {
	Post(path string, body any) (json.RawMessage, int, error)
	GetFresh(path string, params map[string]string) (json.RawMessage, error)
}

type writeFn func(string)

type rejectedTaskPost struct {
	index         int
	statusCode    int
	statusMessage string
}

// PATCH: DataForSEO can accept some task_post siblings while rejecting others in one HTTP-success response.
func extractTaskPostOutcome(raw json.RawMessage) ([]string, []rejectedTaskPost, error) {
	var resp struct {
		Tasks []struct {
			ID            string `json:"id"`
			StatusCode    int    `json:"status_code"`
			StatusMessage string `json:"status_message"`
		} `json:"tasks"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, nil, err
	}
	ids := make([]string, 0, len(resp.Tasks))
	rejected := make([]rejectedTaskPost, 0)
	for i, t := range resp.Tasks {
		if t.ID != "" && t.StatusCode >= 20000 && t.StatusCode < 30000 {
			ids = append(ids, t.ID)
			continue
		}
		message := t.StatusMessage
		if message == "" {
			if t.ID == "" {
				message = "response did not include a task ID"
			} else {
				message = "task was not accepted"
			}
		}
		rejected = append(rejected, rejectedTaskPost{index: i, statusCode: t.StatusCode, statusMessage: message})
	}
	return ids, rejected, nil
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
			paths, err := resolveTaskPaths(cmd.Root(), "/v3/"+endpoint)
			if err != nil {
				return usageErr(err)
			}
			getPath := replacePathParam(paths.get, "id", url.PathEscape(taskID))
			resp, err := c.GetFresh(getPath, nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if _, err := s.DB().Exec(
				`UPDATE task_ledger SET status = ?, fetched_at = ?, result = ? WHERE task_id = ?`,
				"fetched", time.Now().UTC().Format(time.RFC3339), string(resp), taskID,
			); err != nil {
				return fmt.Errorf("updating task %s: %w", taskID, err)
			}
			return printJSONFiltered(cmd.OutOrStdout(), resp, flags)
		},
	}
	return cmd
}
