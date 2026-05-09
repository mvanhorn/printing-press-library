// Copyright 2026 nikica-jokic. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/fireflies/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/fireflies/internal/store"

	"github.com/spf13/cobra"
)

func newSummaryCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var format string

	cmd := &cobra.Command{
		Use:   "summary <id>",
		Short: "Get the AI-generated summary for a transcript",
		Long:  "Reads summary from local store. Run 'transcripts pull <id>' first to hydrate.",
		Example: strings.Trim(`
  fireflies-pp-cli summary abc123
  fireflies-pp-cli summary abc123 --format bullets
  fireflies-pp-cli summary abc123 --format topics --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), `{"overview":"test summary"}`)
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("fireflies-pp-cli")
			}
			db, err := store.OpenReadOnly(dbPath)
			if err != nil {
				return fmt.Errorf("opening store: %w\nRun 'fireflies-pp-cli transcripts pull %s'.", err, args[0])
			}
			defer db.Close()

			raw, err := db.Get("transcripts", args[0])
			if err != nil {
				return fmt.Errorf("transcript not found — run 'transcripts pull %s' first", args[0])
			}
			var t transcriptRow
			if err := json.Unmarshal(raw, &t); err != nil {
				return fmt.Errorf("parsing transcript: %w", err)
			}
			if t.Summary == nil {
				return fmt.Errorf("no summary available — run 'transcripts pull %s' to hydrate sentences and summary", args[0])
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !humanFriendly) {
				return printJSONFiltered(cmd.OutOrStdout(), t.Summary, flags)
			}

			switch strings.ToLower(format) {
			case "overview", "":
				fmt.Fprintf(cmd.OutOrStdout(), "# Summary: %s\n\n%s\n", t.Title, t.Summary.Overview)
			case "bullets", "shorthand":
				fmt.Fprintf(cmd.OutOrStdout(), "# Summary: %s\n\n%s\n", t.Title, t.Summary.ShorthandBullet)
			case "gist":
				fmt.Fprintf(cmd.OutOrStdout(), "%s\n", t.Summary.Gist)
			case "topics":
				fmt.Fprintf(cmd.OutOrStdout(), "Topics discussed in %q:\n\n", t.Title)
				for _, topic := range t.Summary.Topics {
					fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", topic)
				}
			case "keywords":
				fmt.Fprintf(cmd.OutOrStdout(), "Keywords: %s\n", strings.Join(t.Summary.Keywords, ", "))
			case "actions":
				fmt.Fprintf(cmd.OutOrStdout(), "Action Items:\n\n%s\n", t.Summary.ActionItems)
			default:
				return fmt.Errorf("unknown format %q — use: overview, bullets, gist, topics, keywords, actions", format)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&format, "format", "overview", "Output format: overview, bullets, gist, topics, keywords, actions")
	return cmd
}
