// Copyright 2026 dstevens. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/productivity/granola/internal/granola"
)

func newTiptapCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tiptap",
		Short: "TipTap renderer utilities",
	}
	cmd.AddCommand(newTiptapExtractCmd(flags))
	return cmd
}

func newTiptapExtractCmd(flags *rootFlags) *cobra.Command {
	var as string
	cmd := &cobra.Command{
		Use:   "extract <id>",
		Short: "Render documents[id].notes (TipTap JSON) as markdown, plain, or json",
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
			format := as
			if format == "" {
				format = "markdown"
			}
			switch format {
			case "markdown":
				md, err := granola.Render(d.Notes)
				if err != nil {
					return err
				}
				if md == "" {
					md = d.NotesMarkdown
				}
				fmt.Fprint(cmd.OutOrStdout(), md)
				return nil
			case "plain":
				fmt.Fprint(cmd.OutOrStdout(), d.NotesPlain)
				return nil
			case "json":
				out, _ := json.MarshalIndent(json.RawMessage(d.Notes), "", "  ")
				fmt.Fprintln(cmd.OutOrStdout(), string(out))
				return nil
			default:
				return usageErr(fmt.Errorf("invalid --as %q (markdown|plain|json)", format))
			}
		},
	}
	cmd.Flags().StringVar(&as, "as", "markdown", "Output format: markdown | plain | json")
	return cmd
}
