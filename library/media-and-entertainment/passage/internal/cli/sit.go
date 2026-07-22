// Copyright 2026 justinwfu and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
)

// pp:data-source live
func newNovelSitCmd(flags *rootFlags) *cobra.Command {
	var flagNote string
	var flagMood int
	var flagLen int

	cmd := &cobra.Command{
		Use:         "sit <gutenberg-id>",
		Short:       "Read a real Project Gutenberg passage and capture a reflection to your journal",
		Long:        "sit fetches a real Project Gutenberg text by its id, shows an excerpt to read, and — with --note — saves your reflection to your local journal. Find ids via 'passage today' or 'passage search'.",
		Example:     "  passage sit 1342 --note \"Austen's irony still lands\"",
		Annotations: map[string]string{"mcp:read-only": "false"}, // --note writes a reflection
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			id, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("expected a numeric Gutenberg id (e.g. 1342), got %q", args[0])
			}
			if flagMood < 0 || flagMood > 5 {
				return fmt.Errorf("--mood must be 0-5 (0 = unset), got %d", flagMood)
			}
			if flagLen <= 0 {
				flagLen = 1200
			}
			ctx := cmd.Context()
			gc := gutClient()
			book, err := gc.Get(ctx, id)
			if err != nil {
				return err
			}
			excerpt, err := gc.Excerpt(ctx, book, flagLen)
			if err != nil {
				return err
			}

			saved := false
			if flagNote != "" {
				p, err := openPractice(cmd)
				if err != nil {
					return err
				}
				defer p.Close()
				if err := p.AddReflection(ctx, id, book.Title, flagNote, flagMood); err != nil {
					return err
				}
				saved = true
			}

			out := struct {
				ID       int    `json:"gutenberg_id"`
				Title    string `json:"title"`
				Author   string `json:"author"`
				Excerpt  string `json:"excerpt"`
				Note     string `json:"note,omitempty"`
				Mood     int    `json:"mood,omitempty"`
				Reflected bool  `json:"reflection_saved"`
			}{id, book.Title, book.AuthorLine(), excerpt, flagNote, flagMood, saved}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.quiet) {
				return flags.printJSON(cmd, out)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s — %s\n\n%s\n", book.Title, book.AuthorLine(), excerpt)
			if saved {
				fmt.Fprintf(cmd.OutOrStdout(), "\n✓ reflection saved to your journal\n")
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "\n(add --note \"...\" to save a reflection)\n")
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagNote, "note", "", "A reflection to save to your journal for this sit")
	cmd.Flags().IntVar(&flagMood, "mood", 0, "Optional mood 1-5 to record with the reflection")
	cmd.Flags().IntVar(&flagLen, "chars", 0, "Excerpt length in characters (default 1200)")
	return cmd
}
