// Copyright 2026 justinwfu and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"github.com/spf13/cobra"
)

// pp:data-source local
func newNovelJournalCmd(flags *rootFlags) *cobra.Command {
	var flagLimit string

	cmd := &cobra.Command{
		Use:         "journal",
		Short:       "Your reflections over time, newest first — the record of your reading practice",
		Long:        "journal lists the reflections you've captured while sitting with passages, newest first — the durable record of your reading practice.",
		Example:     "  passage journal --limit 20",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			limit := atoiOr(flagLimit, 20)
			p, err := openPractice(cmd)
			if err != nil {
				return err
			}
			defer p.Close()
			refs, err := p.ListReflections(cmd.Context(), limit)
			if err != nil {
				return err
			}
			table := make([][]string, 0, len(refs))
			for _, r := range refs {
				table = append(table, []string{shortDate(r.CreatedAt), r.Title, truncate(r.Note, 80)})
			}
			return bookRender(cmd, flags, refs, []string{"Date", "Title", "Reflection"}, table)
		},
	}
	cmd.Flags().StringVar(&flagLimit, "limit", "", "Max reflections to show (default 20)")
	return cmd
}
