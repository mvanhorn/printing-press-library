// Copyright 2026 justinwfu. Licensed under Apache-2.0.

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/art-goat/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/art-goat/internal/store"

	"github.com/spf13/cobra"
)

func newRandomCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var sourceFilter string

	cmd := &cobra.Command{
		Use:   "random",
		Short: "Pick one random work (bare envelope, no prompt)",
		Long: `Pick one random work from the local corpus and emit a bare envelope —
no prompt, no rendering frills. Useful as a building block for shell pipes
and agents that want raw work identity without contemplative framing.

Default output is a single line: '<id>  <title> — <creator>'. Pass --json
to get the full envelope (id, source, title, creator, image_url, source_url).`,
		Example: `  # Bare line: id, title, creator
  art-goat-pp-cli random

  # Constrain to one source
  art-goat-pp-cli random --source apod

  # Full envelope
  art-goat-pp-cli random --json`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliutil.IsVerifyEnv() && !flags.asJSON {
				return emitRandomVerifyEnvelope(cmd)
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

			envelope := map[string]any{
				"id":         pick.ID,
				"source":     pick.Source,
				"title":      pick.Title,
				"creator":    pick.Creator,
				"image_url":  pick.ImageURL,
				"source_url": pick.SourceURL,
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), envelope, flags)
			}
			out := cmd.OutOrStdout()
			title := coalesce(pick.Title, "(untitled)")
			if pick.Creator != "" {
				fmt.Fprintf(out, "%s  %s — %s\n", pick.ID, title, pick.Creator)
			} else {
				fmt.Fprintf(out, "%s  %s\n", pick.ID, title)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/art-goat-pp-cli/data.db)")
	cmd.Flags().StringVar(&sourceFilter, "source", "", "Constrain to one source (e.g. aic, apod)")
	return cmd
}

func emitRandomVerifyEnvelope(cmd *cobra.Command) error {
	envelope := map[string]any{
		"command":                 "random",
		"verify_noop":             true,
		"success":                 true,
		"__pp_verify_synthetic__": true,
		"reason":                  "verify_short_circuit",
		"note":                    "random prints to terminal by default; PRINTING_PRESS_VERIFY=1 short-circuits the bare line. Pass --json to get the data envelope.",
	}
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(envelope)
}
