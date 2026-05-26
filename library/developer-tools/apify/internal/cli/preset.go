// `apify-pp-cli preset save|list|show|delete` — persist named Actor input
// presets in the local store. Replay with `--preset <name>` on `run`.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/apify/internal/store"
)

func newPresetCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "preset",
		Short: "Save and replay named Actor input presets",
		Long: strings.Trim(`
Capture known-good Actor input JSON locally and replay it from any run
with --preset <name>. Solves the "what flags did I use last week" problem.

Subcommands:
  save     Save a named preset from inline input, a file, or a prior run
  list     List saved presets (optionally filtered by actor)
  show     Print one preset's input JSON
  delete   Remove a preset
`, "\n"),
		RunE: parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newPresetSaveCmd(flags))
	cmd.AddCommand(newPresetListCmd(flags))
	cmd.AddCommand(newPresetShowCmd(flags))
	cmd.AddCommand(newPresetDeleteCmd(flags))
	return cmd
}

func newPresetSaveCmd(flags *rootFlags) *cobra.Command {
	var (
		actor   string
		input   string
		fromRun string
	)
	cmd := &cobra.Command{
		Use:   "save <name>",
		Short: "Save a named Actor input preset",
		Long: strings.Trim(`
Save Actor input JSON under a name for replay via --preset on the run command.

Examples:
  apify-pp-cli preset save weekly-ai --actor apidojo/twitter-scraper --input @q.json
  apify-pp-cli preset save daily-reddit --actor trudax/reddit-scraper --from-run abc123
  apify-pp-cli preset save quick --actor apify/google-news-scraper --input '{"q":"AI"}'
`, "\n"),
		Example: strings.Trim(`
  apify-pp-cli preset save weekly-ai --actor apidojo/twitter-scraper --input @q.json
  apify-pp-cli preset save daily-reddit --actor trudax/reddit-scraper --from-run abc123
`, "\n"),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			name := args[0]
			if actor == "" {
				return usageErr(fmt.Errorf("--actor is required"))
			}
			if input == "" && fromRun == "" {
				return usageErr(fmt.Errorf("provide --input <json|@file> or --from-run <runId>"))
			}

			ctx := cmd.Context()
			db, err := store.OpenWithContext(ctx, defaultDBPath("apify-pp-cli"))
			if err != nil {
				return configErr(fmt.Errorf("opening local store: %w", err))
			}
			defer db.Close()
			if err := db.EnsureExtensions(ctx); err != nil {
				return configErr(err)
			}

			var data []byte
			switch {
			case input != "":
				data, err = resolveInput(input)
				if err != nil {
					return usageErr(fmt.Errorf("parsing --input: %w", err))
				}
			case fromRun != "":
				data, err = inputFromRun(ctx, flags, fromRun)
				if err != nil {
					return apiErr(fmt.Errorf("fetching input from run %s: %w", fromRun, err))
				}
			}

			if err := db.SavePreset(ctx, name, actor, fromRun, data); err != nil {
				return apiErr(fmt.Errorf("saving preset: %w", err))
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"name":      name,
				"actor":     actor,
				"bytes":     len(data),
				"from_run":  fromRun,
				"saved_at":  time.Now().UTC().Format(time.RFC3339),
			}, flags)
		},
	}
	cmd.Flags().StringVar(&actor, "actor", "", "Actor (required, e.g. apidojo/twitter-scraper)")
	cmd.Flags().StringVar(&input, "input", "", "Input JSON (literal or @file)")
	cmd.Flags().StringVar(&fromRun, "from-run", "", "Capture the input from an existing run ID")
	return cmd
}

func newPresetListCmd(flags *rootFlags) *cobra.Command {
	var actor string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List saved presets",
		Example: strings.Trim(`
  apify-pp-cli preset list --json
  apify-pp-cli preset list --actor apidojo/twitter-scraper --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			db, err := store.OpenWithContext(ctx, defaultDBPath("apify-pp-cli"))
			if err != nil {
				return configErr(err)
			}
			defer db.Close()
			if err := db.EnsureExtensions(ctx); err != nil {
				return configErr(err)
			}
			q := `SELECT name, actor_id, created_at, created_from_run, length(input_json)
			      FROM pp_presets`
			args2 := []any{}
			if actor != "" {
				q += ` WHERE actor_id = ?`
				args2 = append(args2, actor)
			}
			q += ` ORDER BY created_at DESC`
			rows, err := db.DB().QueryContext(ctx, q, args2...)
			if err != nil {
				return apiErr(err)
			}
			defer rows.Close()
			type presetRow struct {
				Name      string `json:"name"`
				Actor     string `json:"actor"`
				CreatedAt string `json:"created_at"`
				FromRun   string `json:"from_run,omitempty"`
				Bytes     int    `json:"bytes"`
			}
			var out []presetRow
			for rows.Next() {
				var r presetRow
				if err := rows.Scan(&r.Name, &r.Actor, &r.CreatedAt, &r.FromRun, &r.Bytes); err != nil {
					return apiErr(err)
				}
				out = append(out, r)
			}
			if err := rows.Err(); err != nil {
				return apiErr(fmt.Errorf("iterating presets: %w", err))
			}
			return printJSONFiltered(cmd.OutOrStdout(),
				map[string]any{"presets": out, "count": len(out)}, flags)
		},
	}
	cmd.Flags().StringVar(&actor, "actor", "", "Filter by Actor ID")
	return cmd
}

func newPresetShowCmd(flags *rootFlags) *cobra.Command {
	var actor string
	cmd := &cobra.Command{
		Use:   "show <name>",
		Short: "Print one preset's input JSON",
		Args:  cobra.MaximumNArgs(1),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if actor == "" {
				return usageErr(fmt.Errorf("--actor is required"))
			}
			ctx := cmd.Context()
			db, err := store.OpenWithContext(ctx, defaultDBPath("apify-pp-cli"))
			if err != nil {
				return configErr(err)
			}
			defer db.Close()
			if err := db.EnsureExtensions(ctx); err != nil {
				return configErr(err)
			}
			data, err := db.LoadPreset(ctx, args[0], actor)
			if err != nil {
				return apiErr(err)
			}
			if len(data) == 0 {
				return notFoundErr(fmt.Errorf("preset %q not found for actor %q", args[0], actor))
			}
			// Try to pretty-print if valid JSON
			var probe any
			if err := json.Unmarshal(data, &probe); err == nil {
				return printJSONFiltered(cmd.OutOrStdout(), probe, flags)
			}
			fmt.Fprintln(cmd.OutOrStdout(), string(data))
			return nil
		},
	}
	cmd.Flags().StringVar(&actor, "actor", "", "Actor ID")
	return cmd
}

func newPresetDeleteCmd(flags *rootFlags) *cobra.Command {
	var actor string
	cmd := &cobra.Command{
		Use:   "delete <name>",
		Short: "Remove a preset",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if actor == "" {
				return usageErr(fmt.Errorf("--actor is required"))
			}
			ctx := cmd.Context()
			db, err := store.OpenWithContext(ctx, defaultDBPath("apify-pp-cli"))
			if err != nil {
				return configErr(err)
			}
			defer db.Close()
			if err := db.EnsureExtensions(ctx); err != nil {
				return configErr(err)
			}
			res, err := db.DB().ExecContext(ctx,
				`DELETE FROM pp_presets WHERE name = ? AND actor_id = ?`, args[0], actor)
			if err != nil {
				return apiErr(err)
			}
			n, _ := res.RowsAffected()
			if n == 0 {
				if flags.ignoreMissing {
					return writeNoop(flags, "preset_not_found", "no preset deleted")
				}
				return notFoundErr(fmt.Errorf("preset %q not found for actor %q", args[0], actor))
			}
			return printJSONFiltered(cmd.OutOrStdout(),
				map[string]any{"deleted": n, "name": args[0], "actor": actor}, flags)
		},
	}
	cmd.Flags().StringVar(&actor, "actor", "", "Actor ID")
	return cmd
}

// inputFromRun fetches the input of an existing run via the API.
// The Apify API exposes input via /v2/actor-runs/{runId} → run.input field.
func inputFromRun(ctx context.Context, flags *rootFlags, runID string) ([]byte, error) {
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	body, err := c.Get(fmt.Sprintf("/v2/actor-runs/%s", escapeSeg(runID)), nil)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Data struct {
			Input map[string]any `json:"input"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	if resp.Data.Input == nil {
		return []byte("{}"), nil
	}
	return json.Marshal(resp.Data.Input)
}

// suppress unused import on os
var _ = os.Stdout
