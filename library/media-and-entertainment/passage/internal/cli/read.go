// Copyright 2026 justinwfu and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
)

func newReadCmd(flags *rootFlags) *cobra.Command {
	var chars int
	cmd := &cobra.Command{
		Use:         "read <gutenberg-id>",
		Short:       "Print an excerpt of a public-domain book's full text",
		Long:        "Fetches a Project Gutenberg book by id and prints a cleaned excerpt of its text — the passage without the reflection. Use 'sit' to also journal a reflection.",
		Example:     "  passage read 2680 --chars 2000",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			id, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("expected a numeric Gutenberg id (e.g. 2680), got %q", args[0])
			}
			if chars <= 0 {
				chars = 1500
			}
			ctx := cmd.Context()
			gc := gutClient()
			book, err := gc.Get(ctx, id)
			if err != nil {
				return err
			}
			excerpt, err := gc.Excerpt(ctx, book, chars)
			if err != nil {
				return err
			}
			out := struct {
				ID      int    `json:"gutenberg_id"`
				Title   string `json:"title"`
				Author  string `json:"author"`
				Excerpt string `json:"excerpt"`
			}{id, book.Title, book.AuthorLine(), excerpt}
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.quiet) {
				return flags.printJSON(cmd, out)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s — %s\n\n%s\n", book.Title, book.AuthorLine(), excerpt)
			return nil
		},
	}
	cmd.Flags().IntVar(&chars, "chars", 0, "Excerpt length in characters (default 1500)")
	return cmd
}
