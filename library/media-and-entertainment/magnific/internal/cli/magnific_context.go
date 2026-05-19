package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/magnific/internal/store"
)

func newMagnificContextCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "context",
		Short: "Emit agent-native orientation JSON (top models used, recent prompts, recent outputs, API reachability)",
		Long: `context returns a single JSON document an agent reads at session start
to orient itself: the models you've used most often, the last ~10 prompts
you've run, the directories where outputs landed, and whether the
Magnific API is currently reachable.

The Magnific OpenAPI spec does not expose a /v1/me or /v1/credits endpoint
(credit balance lives in the web dashboard only) so credit data comes from
local task accounting, not a live API call.`,
		Example: "  magnific-pp-cli context --json",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			dbPath := defaultDBPath("magnific-pp-cli")
			out := map[string]any{
				"cli":          "magnific-pp-cli",
				"generated_at": time.Now().UTC().Format(time.RFC3339),
				"db_path":      dbPath,
			}

			db, err := store.OpenWithContext(ctx, dbPath)
			if err == nil {
				defer db.Close()
				if err := store.EnsureMagnificTables(ctx, db.DB()); err == nil {
					out["top_models"] = collectTopModels(ctx, db.DB(), 10)
					out["recent_prompts"] = collectRecentPrompts(ctx, db.DB(), 10)
					out["recent_assets"] = collectRecentAssets(ctx, db.DB(), 10)
					out["task_counts"] = collectTaskCounts(ctx, db.DB())
				} else {
					out["store_warning"] = "tables not initialized: " + err.Error()
				}
			} else {
				out["store_warning"] = "could not open store: " + err.Error()
			}

			// Live reachability probe (cheap GET with no body)
			if c, err := flags.newClient(); err == nil {
				code, perr := c.ProbeGet("/v1/icons")
				out["api_reachable"] = perr == nil && code < 500
				out["api_status_code"] = code
			} else {
				out["api_reachable"] = false
				out["api_unconfigured_error"] = err.Error()
			}

			out["model_catalog_size"] = len(magnificModels)
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	return cmd
}

type modelCount struct {
	Model string `json:"model"`
	Count int    `json:"count"`
}

func collectTopModels(ctx context.Context, db *sql.DB, limit int) []modelCount {
	rows, err := db.QueryContext(ctx, `
		SELECT model, COUNT(*) AS c FROM magnific_tasks
		WHERE model != ''
		GROUP BY model ORDER BY c DESC LIMIT ?`, limit)
	if err != nil {
		return nil
	}
	defer rows.Close()
	out := make([]modelCount, 0, limit)
	for rows.Next() {
		var m modelCount
		var nm sql.NullString
		var c sql.NullInt64
		if err := rows.Scan(&nm, &c); err != nil {
			continue
		}
		m.Model = nm.String
		m.Count = int(c.Int64)
		out = append(out, m)
	}
	return out
}

type promptRow struct {
	Prompt    string `json:"prompt"`
	Model     string `json:"model"`
	CreatedAt string `json:"created_at"`
	TaskID    string `json:"task_id,omitempty"`
}

func collectRecentPrompts(ctx context.Context, db *sql.DB, limit int) []promptRow {
	rows, err := db.QueryContext(ctx, `
		SELECT prompt, COALESCE(model,''), COALESCE(created_at,''), COALESCE(task_id,'')
		FROM magnific_prompts
		WHERE prompt != ''
		ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil
	}
	defer rows.Close()
	out := make([]promptRow, 0, limit)
	for rows.Next() {
		var p promptRow
		var prompt, model, created, taskID sql.NullString
		if err := rows.Scan(&prompt, &model, &created, &taskID); err != nil {
			continue
		}
		p.Prompt = truncatePrompt(prompt.String, 120)
		p.Model = model.String
		p.CreatedAt = created.String
		p.TaskID = taskID.String
		out = append(out, p)
	}
	return out
}

type assetRow struct {
	ID         string `json:"id"`
	LocalPath  string `json:"local_path"`
	Model      string `json:"model,omitempty"`
	Tag        string `json:"tag,omitempty"`
	Downloaded string `json:"downloaded_at"`
}

func collectRecentAssets(ctx context.Context, db *sql.DB, limit int) []assetRow {
	rows, err := db.QueryContext(ctx, `
		SELECT id, local_path, COALESCE(model,''), COALESCE(tag,''), COALESCE(downloaded_at,'')
		FROM magnific_assets
		ORDER BY downloaded_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil
	}
	defer rows.Close()
	out := make([]assetRow, 0, limit)
	for rows.Next() {
		var a assetRow
		var id, lp, model, tag, dl sql.NullString
		if err := rows.Scan(&id, &lp, &model, &tag, &dl); err != nil {
			continue
		}
		a.ID = id.String
		a.LocalPath = lp.String
		a.Model = model.String
		a.Tag = tag.String
		a.Downloaded = dl.String
		out = append(out, a)
	}
	return out
}

func collectTaskCounts(ctx context.Context, db *sql.DB) map[string]int {
	rows, err := db.QueryContext(ctx, `SELECT status, COUNT(*) FROM magnific_tasks GROUP BY status`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var s sql.NullString
		var c sql.NullInt64
		if err := rows.Scan(&s, &c); err != nil {
			continue
		}
		out[s.String] = int(c.Int64)
	}
	return out
}

func truncatePrompt(s string, max int) string {
	s = strings.TrimSpace(s)
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "…"
}

// Avoid unused import warning when commands are commented out during dev.
var _ = json.Marshal
var _ = fmt.Sprintf
