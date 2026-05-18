// Copyright 2026 dstevens. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/productivity/granola/internal/granola"
)

// newNotesShowCmd is the human-typed-notes-only path. The three-streams
// rule is enforced here: this command NEVER returns AI panels, NEVER
// returns transcript. Use 'panel get' or 'transcript get' for those.
func newNotesShowCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "notes-show <id>",
		Short: "Show only the human-typed notes (TipTap-rendered markdown) for a meeting",
		Long: `The three-streams rule: this command returns ONLY the human-typed notes.
For AI summaries use 'panel get'; for transcript use 'transcript get'.

Renders documents[id].notes (TipTap JSON) to markdown. Falls back to
notes_markdown then notes_plain when the TipTap blob is empty.`,
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
			c, err := openGranolaCache()
			if err != nil {
				return err
			}
			d := c.DocumentByID(id)
			if d == nil {
				return notFoundErr(fmt.Errorf("meeting %s not in cache", id))
			}
			md := ""
			if len(d.Notes) > 0 {
				md, err = granola.Render(d.Notes)
				if err != nil {
					// Render failures fall through to notes_markdown.
					md = ""
				}
			}
			if md == "" || md == "\n" {
				md = d.NotesMarkdown
			}
			if md == "" {
				md = d.NotesPlain
			}
			if flags.asJSON || flags.agent {
				return emitJSON(cmd, flags, map[string]any{
					"id":          id,
					"title":       d.Title,
					"notes_human": md,
				})
			}
			fmt.Fprint(cmd.OutOrStdout(), md)
			if len(md) > 0 && md[len(md)-1] != '\n' {
				fmt.Fprintln(cmd.OutOrStdout())
			}
			return nil
		},
	}
	return cmd
}
