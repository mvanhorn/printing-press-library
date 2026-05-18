// Copyright 2026 dstevens. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newShowCmd(flags *rootFlags) *cobra.Command {
	var notesOnly, noSummary, withTranscript bool
	cmd := &cobra.Command{
		Use:   "show <id>",
		Short: "Print a full meeting card (notes + summary + optional transcript)",
		Example: `  # Show a meeting card (notes + AI summary)
  granola-pp-cli show ff1186df-593b-4ce5-bb1d-70e265f4a811

  # Notes only (no AI summary section)
  granola-pp-cli show ff1186df-593b-4ce5-bb1d-70e265f4a811 --notes-only

  # Notes + summary + transcript
  granola-pp-cli show ff1186df-593b-4ce5-bb1d-70e265f4a811 --transcript`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			id := args[0]
			a, err := buildArtifacts(id, flags.dataSource != "local", "")
			if err != nil {
				return err
			}
			w := cmd.OutOrStdout()
			if notesOnly {
				fmt.Fprintln(w, strings.TrimSpace(a.NotesHuman))
				return nil
			}
			var b strings.Builder
			b.WriteString("# ")
			b.WriteString(a.Doc.Title)
			b.WriteString("\n\n")
			writeMetadataBlock(&b, a)
			b.WriteString("\n## Notes (Human)\n\n")
			if a.NotesHuman != "" {
				b.WriteString(a.NotesHuman)
				b.WriteString("\n")
			} else {
				b.WriteString("_(no notes)_\n")
			}
			if !noSummary {
				b.WriteString("\n## AI Summary\n\n")
				if a.PanelSummary != "" {
					b.WriteString(strings.TrimSpace(a.PanelSummary))
					b.WriteString("\n")
				} else {
					b.WriteString("_(no panel)_\n")
				}
			}
			if withTranscript {
				b.WriteString("\n## Transcript\n\n")
				for _, s := range a.Transcript {
					fmt.Fprintf(&b, "[%s] %s\n", s.Source, s.Text)
				}
			}
			fmt.Fprint(w, b.String())
			return nil
		},
	}
	cmd.Flags().BoolVar(&notesOnly, "notes-only", false, "Print only the human notes")
	cmd.Flags().BoolVar(&noSummary, "no-summary", false, "Omit the AI Summary section")
	cmd.Flags().BoolVar(&withTranscript, "transcript", false, "Include the transcript section")
	return cmd
}
