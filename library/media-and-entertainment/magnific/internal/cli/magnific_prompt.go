package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/magnific/internal/store"
)

func newMagnificPromptCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "prompt",
		Short: "Manage reusable Magnific prompt templates with {{placeholder}} substitution",
		Long: `prompt is the home for prompt templates: text bodies you reuse across
many generations with {{variable}} placeholders. 'prompt save' stores a
template, 'prompt run' renders it (overriding placeholders via --override
key=value) and dispatches the result to the real Magnific endpoint that
matches the saved model.`,
	}
	cmd.AddCommand(newMagnificPromptSaveCmd(flags))
	cmd.AddCommand(newMagnificPromptListCmd(flags))
	cmd.AddCommand(newMagnificPromptShowCmd(flags))
	cmd.AddCommand(newMagnificPromptRunCmd(flags))
	cmd.AddCommand(newMagnificPromptDeleteCmd(flags))
	return cmd
}

func newMagnificPromptSaveCmd(flags *rootFlags) *cobra.Command {
	var text, model string
	cmd := &cobra.Command{
		Use:     "save <name>",
		Short:   "Save a reusable prompt template under <name>",
		Example: "  magnific-pp-cli prompt save hero-shot --text \"a {{mood}} {{city}} skyline\" --model mystic",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			name := args[0]
			if strings.TrimSpace(text) == "" {
				return usageErr(fmt.Errorf("--text is required"))
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
			_, err = db.DB().ExecContext(ctx, `
				INSERT INTO magnific_prompts(name, prompt, model)
				VALUES (?, ?, ?)
				ON CONFLICT(name) DO UPDATE SET prompt=excluded.prompt, model=excluded.model
			`, name, text, model)
			if err != nil {
				return fmt.Errorf("saving prompt: %w", err)
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"name":  name,
				"saved": true,
				"model": model,
			}, flags)
		},
	}
	cmd.Flags().StringVar(&text, "text", "", "Prompt text (with {{placeholders}}) — required")
	cmd.Flags().StringVar(&model, "model", "", "Default model slug for `prompt run`")
	return cmd
}

func newMagnificPromptListCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List saved prompt templates",
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
			rows, err := db.DB().QueryContext(ctx, `
				SELECT name, prompt, COALESCE(model,''), created_at FROM magnific_prompts
				WHERE name IS NOT NULL AND name != ''
				ORDER BY created_at DESC`)
			if err != nil {
				return fmt.Errorf("query: %w", err)
			}
			defer rows.Close()
			type row struct {
				Name      string `json:"name"`
				Prompt    string `json:"prompt"`
				Model     string `json:"model,omitempty"`
				CreatedAt string `json:"created_at"`
			}
			out := []row{}
			for rows.Next() {
				var r row
				var n, p, m, c sql.NullString
				if err := rows.Scan(&n, &p, &m, &c); err != nil {
					continue
				}
				r.Name = n.String
				r.Prompt = p.String
				r.Model = m.String
				r.CreatedAt = c.String
				out = append(out, r)
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	return cmd
}

func newMagnificPromptShowCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "show <name>",
		Short:       "Show one saved prompt template",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			name := args[0]
			ctx := cmd.Context()
			db, err := store.OpenWithContext(ctx, defaultDBPath("magnific-pp-cli"))
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()
			if err := store.EnsureMagnificTables(ctx, db.DB()); err != nil {
				return fmt.Errorf("initializing magnific tables: %w", err)
			}
			var prompt, model, created sql.NullString
			err = db.DB().QueryRowContext(ctx,
				`SELECT prompt, COALESCE(model,''), created_at FROM magnific_prompts WHERE name = ?`,
				name).Scan(&prompt, &model, &created)
			if err == sql.ErrNoRows {
				return notFoundErr(fmt.Errorf("prompt %q not found", name))
			}
			if err != nil {
				return err
			}
			out := map[string]any{
				"name":         name,
				"prompt":       prompt.String,
				"model":        model.String,
				"placeholders": extractPlaceholders(prompt.String),
				"created_at":   created.String,
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	return cmd
}

func newMagnificPromptDeleteCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <name>",
		Short: "Delete a saved prompt template",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			name := args[0]
			db, err := store.OpenWithContext(ctx, defaultDBPath("magnific-pp-cli"))
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()
			if err := store.EnsureMagnificTables(ctx, db.DB()); err != nil {
				return fmt.Errorf("initializing magnific tables: %w", err)
			}
			res, err := db.DB().ExecContext(ctx, `DELETE FROM magnific_prompts WHERE name = ?`, name)
			if err != nil {
				return err
			}
			n, _ := res.RowsAffected()
			out := map[string]any{"name": name, "deleted": n}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	return cmd
}

func newMagnificPromptRunCmd(flags *rootFlags) *cobra.Command {
	var overrides []string
	var modelOverride string
	var modelPath string
	cmd := &cobra.Command{
		Use:   "run <name>",
		Short: "Render a saved template (substituting {{vars}}) and dispatch to the matching Magnific endpoint",
		Long: `run loads the saved prompt template, substitutes {{key}} placeholders
from --override key=value pairs, then POSTs the rendered prompt to the
model's endpoint from the curated registry. The submitted task_id is
written to the local magnific_tasks ledger and the rendered prompt is
recorded in magnific_prompts for history search.`,
		Example: "  magnific-pp-cli prompt run hero-shot --override city=tokyo --override mood=neon --json",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			name := args[0]
			ctx := cmd.Context()
			db, err := store.OpenWithContext(ctx, defaultDBPath("magnific-pp-cli"))
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()
			if err := store.EnsureMagnificTables(ctx, db.DB()); err != nil {
				return fmt.Errorf("initializing magnific tables: %w", err)
			}
			var templatePrompt, savedModel sql.NullString
			err = db.DB().QueryRowContext(ctx,
				`SELECT prompt, COALESCE(model,'') FROM magnific_prompts WHERE name = ?`,
				name).Scan(&templatePrompt, &savedModel)
			if err == sql.ErrNoRows {
				return notFoundErr(fmt.Errorf("prompt %q not found", name))
			}
			if err != nil {
				return err
			}

			overrideMap, perr := parsePromptOverrides(overrides)
			if perr != nil {
				return usageErr(perr)
			}
			rendered := renderPromptTemplate(templatePrompt.String, overrideMap)
			missing := findUnresolvedPlaceholders(rendered)
			if len(missing) > 0 {
				return usageErr(fmt.Errorf("missing values for placeholders: %s (use --override key=value)", strings.Join(missing, ", ")))
			}

			model := modelOverride
			if model == "" {
				model = savedModel.String
			}
			endpoint := modelPath
			var creditCost float64
			if endpoint == "" {
				if model == "" {
					return usageErr(fmt.Errorf("no model resolved; saved template has no --model and you did not pass --model"))
				}
				mr := lookupModel(model)
				if mr == nil {
					return notFoundErr(fmt.Errorf("model %q not in curated registry (use --endpoint /v1/ai/... for unlisted models)", model))
				}
				endpoint = mr.Endpoint
				creditCost = mr.CreditCost
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			body := map[string]any{"prompt": rendered}
			respBody, status, perr := c.Post(endpoint, body)
			if perr != nil {
				return fmt.Errorf("dispatching to %s: %w", endpoint, perr)
			}
			taskID := extractTaskID(respBody)
			// Record dispatch in local ledger so wait/watch/history work.
			_, _ = db.DB().ExecContext(ctx,
				`INSERT OR REPLACE INTO magnific_tasks(task_id, model, endpoint, status, prompt, credit_cost)
				 VALUES (?, ?, ?, COALESCE(?,'IN_PROGRESS'), ?, ?)`,
				taskID, model, endpoint, extractTaskStatus(respBody), rendered, creditCost)
			_, _ = db.DB().ExecContext(ctx,
				`INSERT INTO magnific_prompts(prompt, model, task_id) VALUES (?, ?, ?)`,
				rendered, model, taskID)

			out := map[string]any{
				"template":      name,
				"rendered":      rendered,
				"model":         model,
				"endpoint":      endpoint,
				"http_status":   status,
				"task_id":       taskID,
				"response_body": json.RawMessage(respBody),
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringArrayVar(&overrides, "override", nil, "Override key=value (repeatable)")
	cmd.Flags().StringVar(&modelOverride, "model", "", "Override the saved model")
	cmd.Flags().StringVar(&modelPath, "endpoint", "", "Override the resolved endpoint path (e.g. /v1/ai/mystic)")
	return cmd
}

var placeholderRe = regexp.MustCompile(`\{\{\s*([a-zA-Z0-9_]+)\s*\}\}`)

func extractPlaceholders(prompt string) []string {
	seen := map[string]bool{}
	var out []string
	for _, m := range placeholderRe.FindAllStringSubmatch(prompt, -1) {
		if !seen[m[1]] {
			seen[m[1]] = true
			out = append(out, m[1])
		}
	}
	return out
}

func parsePromptOverrides(args []string) (map[string]string, error) {
	out := map[string]string{}
	for _, a := range args {
		idx := strings.Index(a, "=")
		if idx <= 0 {
			return nil, fmt.Errorf("override %q must be key=value", a)
		}
		out[a[:idx]] = a[idx+1:]
	}
	return out, nil
}

func renderPromptTemplate(prompt string, vars map[string]string) string {
	return placeholderRe.ReplaceAllStringFunc(prompt, func(match string) string {
		m := placeholderRe.FindStringSubmatch(match)
		if v, ok := vars[m[1]]; ok {
			return v
		}
		return match
	})
}

func findUnresolvedPlaceholders(rendered string) []string {
	return extractPlaceholders(rendered)
}

func extractTaskID(body []byte) string {
	var env struct {
		Data struct {
			TaskID string `json:"task_id"`
			ID     string `json:"id"`
		} `json:"data"`
		TaskID string `json:"task_id"`
		ID     string `json:"id"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return ""
	}
	if env.Data.TaskID != "" {
		return env.Data.TaskID
	}
	if env.Data.ID != "" {
		return env.Data.ID
	}
	if env.TaskID != "" {
		return env.TaskID
	}
	return env.ID
}
