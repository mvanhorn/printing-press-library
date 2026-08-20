// Copyright 2026 justinwfu and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newLogCmd(flags *rootFlags) *cobra.Command {
	var status, title string
	var rating int
	cmd := &cobra.Command{
		Use:         "log <work-key>",
		Short:       "Log or rate a read (start, finish, rating)",
		Long:        "log records a book's reading status (want|reading|read) and an optional rating. Finishing (status=read) stamps a finished date. Feeds 'stats'.",
		Example:     "  passage log OL45804W --status read --rating 5 --title \"Meditations\"",
		Annotations: map[string]string{"mcp:read-only": "false"}, // writes the reading log
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				return fmt.Errorf("log needs a <work-key> — e.g. passage log OL45804W --status read")
			}
			if !validWorkKey(args[0]) {
				return fmt.Errorf("invalid work-key %q — use an Open Library key (e.g. OL45804W) or a Gutenberg id (e.g. 2680)", args[0])
			}
			if status == "" {
				status = "reading"
			}
			if !shelfStatuses[status] {
				return fmt.Errorf("--status must be one of want|reading|read, got %q", status)
			}
			if rating < 0 || rating > 5 {
				return fmt.Errorf("--rating must be 0-5, got %d", rating)
			}
			p, err := openPractice(cmd)
			if err != nil {
				return err
			}
			defer p.Close()
			if err := p.LogRead(cmd.Context(), args[0], title, status, rating); err != nil {
				return err
			}
			if !flags.quiet {
				fmt.Fprintf(cmd.OutOrStdout(), "✓ logged %s as %s", dashOr(title, args[0]), status)
				if rating > 0 {
					fmt.Fprintf(cmd.OutOrStdout(), " (%d/5)", rating)
				}
				fmt.Fprintln(cmd.OutOrStdout())
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&status, "status", "", "want|reading|read (default reading)")
	cmd.Flags().IntVar(&rating, "rating", 0, "Rating 1-5 (0 = unrated)")
	cmd.Flags().StringVar(&title, "title", "", "Book title (for display)")
	return cmd
}
