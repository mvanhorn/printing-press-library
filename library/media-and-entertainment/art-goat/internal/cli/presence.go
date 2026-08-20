// Copyright 2026 justinwfu and contributors. Licensed under Apache-2.0.

package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/art-goat/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/art-goat/internal/store"

	"github.com/spf13/cobra"
)

func newPresenceCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var sourceFilter string

	cmd := &cobra.Command{
		Use:   "presence",
		Short: "Random piece + prompt, no timer",
		Long: `Pick a piece at random from the local corpus and pair it with a
contemplative prompt. Unlike 'today', presence is meant to be incidental —
no anti-repeat, no diversity scoring, no sit flow. Use it for a thirty-second
pause between meetings or as a screensaver-style nudge back to attention.

Use --json to emit a structured envelope. Use --source to constrain to one
configured source (e.g. aic, apod).`,
		Example: `  # A random piece + prompt
  art-goat-pp-cli presence

  # Constrain to one source
  art-goat-pp-cli presence --source aic

  # Structured envelope for agent use
  art-goat-pp-cli presence --json --select work_id,prompt,source_url`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliutil.IsVerifyEnv() && !flags.asJSON {
				return emitPresenceVerifyEnvelope(cmd)
			}

			if dbPath == "" {
				dbPath = defaultDBPath("art-goat-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()
			if err := db.EnsureArtGoatTables(cmd.Context()); err != nil {
				return err
			}

			var sources []string
			if sourceFilter != "" {
				sources = []string{sourceFilter}
			}
			pick, err := db.RandomWork(cmd.Context(), sources, nil)
			if err != nil {
				return err
			}
			if pick == nil {
				return fmt.Errorf("no works available — run `art-goat-pp-cli sources sync` first")
			}

			prompt := pickPrompt(promptSeed(pick.ID))
			envelope := map[string]any{
				"kind":        "presence",
				"work_id":     pick.ID,
				"source":      pick.Source,
				"title":       pick.Title,
				"creator":     pick.Creator,
				"date":        pick.DateText,
				"medium":      pick.Medium,
				"region":      pick.CultureRegion,
				"image_url":   pick.ImageURL,
				"source_url":  pick.SourceURL,
				"description": pick.Description,
				"prompt":      prompt,
				"chosen_at":   time.Now().UTC().Format(time.RFC3339),
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), envelope, flags)
			}
			renderPresence(cmd, pick, prompt)
			return nil
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/art-goat-pp-cli/data.db)")
	cmd.Flags().StringVar(&sourceFilter, "source", "", "Constrain to one source (e.g. aic, apod)")
	return cmd
}

func renderPresence(cmd *cobra.Command, w *store.Work, prompt string) {
	out := cmd.OutOrStdout()
	fmt.Fprintln(out, "")
	fmt.Fprintf(out, "%s", coalesce(w.Title, "(untitled)"))
	if w.Creator != "" {
		fmt.Fprintf(out, " — %s", w.Creator)
	}
	if w.DateText != "" {
		fmt.Fprintf(out, " (%s)", w.DateText)
	}
	fmt.Fprintln(out, "")
	if w.SourceURL != "" {
		fmt.Fprintf(out, "%s\n", w.SourceURL)
	}
	fmt.Fprintln(out, "")
	fmt.Fprintf(out, "  %s\n", prompt)
	fmt.Fprintln(out, "")
}

func emitPresenceVerifyEnvelope(cmd *cobra.Command) error {
	envelope := map[string]any{
		"command":                 "presence",
		"verify_noop":             true,
		"success":                 true,
		"__pp_verify_synthetic__": true,
		"reason":                  "verify_short_circuit",
		"note":                    "presence renders to terminal by default; PRINTING_PRESS_VERIFY=1 short-circuits rendering. Pass --json to get the data envelope.",
	}
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(envelope)
}
