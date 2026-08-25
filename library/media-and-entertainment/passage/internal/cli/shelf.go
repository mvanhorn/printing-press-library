// Copyright 2026 justinwfu and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var shelfStatuses = map[string]bool{"want": true, "reading": true, "read": true}

func newShelfCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "shelf",
		Short: "Your reading shelf — want / reading / read",
		Long:  "Track books on your shelf by status. Add with 'shelf add', list with 'shelf list'.",
	}
	cmd.AddCommand(newShelfAddCmd(flags), newShelfListCmd(flags))
	return cmd
}

func newShelfAddCmd(flags *rootFlags) *cobra.Command {
	var to, title, author string
	cmd := &cobra.Command{
		Use:         "add <work-key>",
		Short:       "Add (or move) a book on your shelf",
		Example:     "  passage shelf add OL45804W --to want --title \"Meditations\" --author \"Marcus Aurelius\"",
		Annotations: map[string]string{"mcp:read-only": "false"}, // writes the shelf
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				return fmt.Errorf("shelf add needs a <work-key> — e.g. passage shelf add OL45804W --to want")
			}
			if !validWorkKey(args[0]) {
				return fmt.Errorf("invalid work-key %q — use an Open Library key (e.g. OL45804W) or a Gutenberg id (e.g. 2680)", args[0])
			}
			if to == "" {
				to = "want"
			}
			if !shelfStatuses[to] {
				return fmt.Errorf("--to must be one of want|reading|read, got %q", to)
			}
			p, err := openPractice(cmd)
			if err != nil {
				return err
			}
			defer p.Close()
			if err := p.AddShelf(cmd.Context(), args[0], title, author, to); err != nil {
				return err
			}
			if !flags.quiet {
				fmt.Fprintf(cmd.OutOrStdout(), "✓ %s → %s shelf\n", dashOr(title, args[0]), to)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&to, "to", "", "Shelf: want|reading|read (default want)")
	cmd.Flags().StringVar(&title, "title", "", "Book title (for display)")
	cmd.Flags().StringVar(&author, "author", "", "Author (for display)")
	return cmd
}

func newShelfListCmd(flags *rootFlags) *cobra.Command {
	var status string
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List books on your shelf",
		Example:     "  passage shelf list --status reading",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if status != "" && !shelfStatuses[status] {
				return fmt.Errorf("--status must be one of want|reading|read, got %q", status)
			}
			p, err := openPractice(cmd)
			if err != nil {
				return err
			}
			defer p.Close()
			items, err := p.ListShelf(cmd.Context(), status)
			if err != nil {
				return err
			}
			table := make([][]string, 0, len(items))
			for _, s := range items {
				table = append(table, []string{s.Status, dashOr(s.Title, s.WorkKey), s.Author, shortDate(s.AddedAt)})
			}
			return bookRender(cmd, flags, items, []string{"Status", "Title", "Author", "Added"}, table)
		},
	}
	cmd.Flags().StringVar(&status, "status", "", "Filter by shelf: want|reading|read")
	return cmd
}

func dashOr(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}
