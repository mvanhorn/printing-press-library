// Copyright 2026 justinwfu and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// pp:data-source local
func newNovelNextCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "next",
		Short:       "What to read next off your want-to-read shelf",
		Long:        "next suggests what to pick up next from your want-to-read shelf, oldest wants first — the book you've been meaning to read the longest.",
		Example:     "  passage next",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			p, err := openPractice(cmd)
			if err != nil {
				return err
			}
			defer p.Close()
			want, err := p.ListShelf(cmd.Context(), "want")
			if err != nil {
				return err
			}
			if len(want) == 0 {
				if !flags.asJSON {
					fmt.Fprintln(cmd.OutOrStdout(), "Your want-to-read shelf is empty — add one with: passage shelf add <work-key> --to want")
				}
				return bookRender(cmd, flags, []any{}, []string{"Title", "Author", "Added"}, nil)
			}
			// Oldest wants first (already sorted added_at ASC).
			table := make([][]string, 0, len(want))
			for _, s := range want {
				table = append(table, []string{s.Title, s.Author, shortDate(s.AddedAt)})
			}
			return bookRender(cmd, flags, want, []string{"Title", "Author", "Added"}, table)
		},
	}
	return cmd
}
