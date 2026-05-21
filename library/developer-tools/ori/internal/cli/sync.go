// Copyright 2026 error. Licensed under Apache-2.0.
// Hand-customized: replaces the generator's spec-derived sync (which assumed
// REST resources) with A2A-aware fan-out across configured agents.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/ori/internal/client"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/ori/internal/store"
)

const tasksCacheSchema = `
CREATE TABLE IF NOT EXISTS tasks_cache (
	id TEXT NOT NULL,
	agent TEXT NOT NULL,
	context_id TEXT NOT NULL DEFAULT '',
	state TEXT,
	state_timestamp TEXT,
	text TEXT,
	first_seen_at TEXT NOT NULL,
	last_synced_at TEXT NOT NULL,
	raw_json TEXT,
	PRIMARY KEY (agent, id)
);
CREATE INDEX IF NOT EXISTS idx_tasks_context ON tasks_cache(context_id);
CREATE INDEX IF NOT EXISTS idx_tasks_state ON tasks_cache(state);
CREATE VIRTUAL TABLE IF NOT EXISTS tasks_fts USING fts5(
	id UNINDEXED,
	agent UNINDEXED,
	text,
	tokenize='porter unicode61'
);
`

func ensureTasksCacheSchema(db *sql.DB) error {
	_, err := db.Exec(tasksCacheSchema)
	return err
}

// syncResult is a shim type kept for channel_workflow.go's compile-time call to
// syncResource. The generator's generic resource sync model doesn't fit our
// JSON-RPC A2A surface — sync above does the A2A-aware work directly.
type syncResult struct {
	Count int
	Err   error
	Warn  error
}

// syncResource is a no-op stub for the generated channel_workflow.go. Real
// per-agent sync work lives in syncAgent. Returns zero count so the workflow's
// summary line reads "well-known: 0 synced" and the command exits cleanly.
//
//nolint:unused // keeps channel_workflow.go compiling without an edit there.
func syncResource(_ *client.Client, _ *store.Store, _ string, _ string, _ bool, _ int, _ bool, _ any) syncResult {
	return syncResult{Count: 0}
}

func newSyncCmd(flags *rootFlags) *cobra.Command {
	var agentFilter string
	var pageSize int
	var maxPages int
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Pull tasks from every configured agent into the local SQLite mirror",
		Long: `Fans out to /a2a/{agent}/tasks across every configured agent (or just
--agent if set), paginates through every task page, and upserts into the
local tasks_cache table + tasks_fts FTS5 mirror. This is the foundation for
'tasks search' and 'contexts list'.

Run after every meaningful agent activity period — sync is idempotent and only
takes a few seconds against typical task counts.`,
		Example: `  ori-pp-cli sync
  ori-pp-cli sync --agent ori
  ori-pp-cli sync --page-size 100
  ori-pp-cli sync --json`,
		Annotations: map[string]string{"pp:typed-exit-codes": "0,5"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			var agents []string
			if agentFilter != "" {
				agents = []string{agentFilter}
			} else {
				agents, err = discoverAgentNames(c)
				if err != nil {
					return apiErr(err)
				}
			}

			dbPath := defaultDBPath("ori-pp-cli")
			s, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return apiErr(fmt.Errorf("opening store: %w", err))
			}
			defer s.Close()
			if err := ensureTasksCacheSchema(s.DB()); err != nil {
				return apiErr(fmt.Errorf("creating schema: %w", err))
			}

			now := time.Now().UTC().Format(time.RFC3339)
			report := map[string]any{
				"started_at": now,
				"agents":     map[string]any{},
			}

			for _, name := range agents {
				agentReport := syncAgent(cmd.Context(), c, s.DB(), name, pageSize, maxPages, now)
				report["agents"].(map[string]any)[name] = agentReport
			}
			report["finished_at"] = time.Now().UTC().Format(time.RFC3339)

			for _, name := range agents {
				_, _ = s.DB().Exec(`INSERT INTO sync_state(resource_type, total_count, last_synced_at)
					VALUES(?, (SELECT COUNT(*) FROM tasks_cache WHERE agent=?), CURRENT_TIMESTAMP)
					ON CONFLICT(resource_type) DO UPDATE SET total_count=excluded.total_count, last_synced_at=excluded.last_synced_at`,
					"tasks_"+name, name)
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), report, flags)
			}
			w := cmd.OutOrStdout()
			for name, r := range report["agents"].(map[string]any) {
				ar, _ := r.(map[string]any)
				fmt.Fprintf(w, "  %s %s: %v tasks synced (%v new, %v updated, %v pages)\n",
					green("OK"), name, ar["total"], ar["new"], ar["updated"], ar["pages"])
				if errStr, _ := ar["error"].(string); errStr != "" {
					fmt.Fprintf(w, "      %s %s\n", red("FAIL"), errStr)
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&agentFilter, "agent", "", "Sync only one agent (default: all configured)")
	cmd.Flags().IntVar(&pageSize, "page-size", 100, "Page size per ListTasks call")
	cmd.Flags().IntVar(&maxPages, "max-pages", 50, "Stop after N pages per agent (safety bound)")
	return cmd
}

func syncAgent(ctx context.Context, c *client.Client, db *sql.DB, agent string, pageSize, maxPages int, syncedAt string) map[string]any {
	report := map[string]any{"new": 0, "updated": 0, "total": 0, "pages": 0}
	pageToken := ""
	for page := 0; page < maxPages; page++ {
		params := map[string]string{
			"page_size": fmt.Sprintf("%d", pageSize),
		}
		if pageToken != "" {
			params["page_token"] = pageToken
		}
		body, err := c.Get("/a2a/"+agent+"/tasks", params)
		if err != nil {
			report["error"] = err.Error()
			break
		}
		var resp struct {
			Tasks []json.RawMessage `json:"tasks"`
			Next  string            `json:"nextPageToken"`
		}
		if jerr := json.Unmarshal(body, &resp); jerr != nil {
			report["error"] = "page parse: " + jerr.Error()
			break
		}
		report["pages"] = page + 1
		for _, raw := range resp.Tasks {
			// Hydrate text via GetTask: ListTasks returns artifacts=[] and
			// history=[] regardless of historyLength, so a follow-up fetch is
			// required to populate FTS search.
			full := hydrateTaskText(c, agent, raw)
			n, u := upsertTask(db, agent, full, syncedAt)
			report["new"] = report["new"].(int) + n
			report["updated"] = report["updated"].(int) + u
			report["total"] = report["total"].(int) + 1
		}
		if resp.Next == "" || len(resp.Tasks) == 0 {
			break
		}
		pageToken = resp.Next
		select {
		case <-ctx.Done():
			report["error"] = ctx.Err().Error()
			return report
		default:
		}
	}
	return report
}

// hydrateTaskText calls GetTask for one task id and merges the populated
// artifacts/history into the raw ListTasks shape. Falls back to the original
// raw bytes if anything goes wrong — sync remains best-effort.
func hydrateTaskText(c *client.Client, agent string, raw json.RawMessage) json.RawMessage {
	var head struct {
		ID string `json:"id"`
	}
	if jerr := json.Unmarshal(raw, &head); jerr != nil || head.ID == "" {
		return raw
	}
	body, gerr := c.Get("/a2a/"+agent+"/tasks/"+head.ID, nil)
	if gerr != nil || len(body) == 0 {
		return raw
	}
	return body
}

// upsertTask flattens an A2A task protobuf JSON into tasks_cache + tasks_fts.
// Returns (new, updated): exactly one is 1, the other 0. When the ListTasks
// response carries empty artifacts (the A2A server's normal behavior), the
// caller will follow up with GetTask to populate text.
func upsertTask(db *sql.DB, agent string, raw json.RawMessage, syncedAt string) (int, int) {
	var t struct {
		ID        string `json:"id"`
		ContextID string `json:"contextId"`
		Status    struct {
			State     string `json:"state"`
			Timestamp string `json:"timestamp"`
		} `json:"status"`
		Artifacts []struct {
			Name  string `json:"name"`
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"artifacts"`
	}
	if err := json.Unmarshal(raw, &t); err != nil {
		return 0, 0
	}
	var sb strings.Builder
	for _, a := range t.Artifacts {
		for _, p := range a.Parts {
			if p.Text != "" {
				if sb.Len() > 0 {
					sb.WriteByte('\n')
				}
				sb.WriteString(p.Text)
			}
		}
	}
	text := sb.String()

	var existed int
	_ = db.QueryRow(`SELECT 1 FROM tasks_cache WHERE agent=? AND id=?`, agent, t.ID).Scan(&existed)

	_, err := db.Exec(`INSERT INTO tasks_cache(id, agent, context_id, state, state_timestamp, text, first_seen_at, last_synced_at, raw_json)
		VALUES(?,?,?,?,?,?,?,?,?)
		ON CONFLICT(agent, id) DO UPDATE SET
			context_id=excluded.context_id,
			state=excluded.state,
			state_timestamp=excluded.state_timestamp,
			text=excluded.text,
			last_synced_at=excluded.last_synced_at,
			raw_json=excluded.raw_json`,
		t.ID, agent, t.ContextID, t.Status.State, t.Status.Timestamp, text, syncedAt, syncedAt, string(raw))
	if err != nil {
		return 0, 0
	}
	_, _ = db.Exec(`DELETE FROM tasks_fts WHERE id=? AND agent=?`, t.ID, agent)
	_, _ = db.Exec(`INSERT INTO tasks_fts(id, agent, text) VALUES(?,?,?)`, t.ID, agent, text)
	if existed == 1 {
		return 0, 1
	}
	return 1, 0
}
