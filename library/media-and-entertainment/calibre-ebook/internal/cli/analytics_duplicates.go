package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

func newLibraryDuplicatesCmd(flags *rootFlags) *cobra.Command {
	var tolerance string

	cmd := &cobra.Command{
		Use:   "duplicates",
		Short: "Find books with duplicate titles or authors",
		Long: `Scans the library for potential duplicate books by comparing
title and author combinations. Returns groups of book IDs that
may represent the same work.

Tolerance levels:
  exact     - exact title + author match (default)
  fuzzy     - normalized title (lowercase, stripped punctuation) + author
  broad     - first 30 chars of normalized title + author`,
		Example:     "  calibre-ebook-pp-cli library duplicates --agent",
		Annotations: map[string]string{"pp:endpoint": "library.duplicates", "pp:method": "GET", "pp:path": "/library/duplicates", "mcp:read-only": "true", "pp:insight": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			data, err := c.Get("/books", map[string]string{
				"fields": "id,title,authors,formats,size",
				"limit":  "0",
			})
			if err != nil {
				return classifyAPIError(err, flags)
			}

			var books []map[string]any
			if err := json.Unmarshal(data, &books); err != nil {
				return fmt.Errorf("parsing book list: %w", err)
			}

			index := map[string][]map[string]any{}
			for _, book := range books {
				key := normalizeForDup(book["title"], book["authors"], tolerance)
				if key != "" {
					index[key] = append(index[key], book)
				}
			}

			var groups []map[string]any
			for _, books := range index {
				if len(books) > 1 {
					ids := make([]any, len(books))
					titles := make([]string, len(books))
					for i, b := range books {
						ids[i] = b["id"]
						titles[i] = fmt.Sprintf("%v", b["title"])
					}
					groups = append(groups, map[string]any{
						"ids":       ids,
						"count":     len(books),
						"title":     titles[0],
						"documents": books,
					})
				}
			}

			result := map[string]any{
				"total_books":    len(books),
				"duplicate_sets": len(groups),
				"tolerance":      tolerance,
				"groups":         groups,
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				wrapped, _ := wrapWithProvenance(mustMarshal(result), DataProvenance{Source: "live"})
				return printOutput(cmd.OutOrStdout(), wrapped, true)
			}

			if len(groups) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "  No duplicates found.")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "  Found %d duplicate sets (%s tolerance):\n\n", len(groups), tolerance)
			for _, g := range groups {
				fmt.Fprintf(cmd.OutOrStdout(), "    \"%v\" — %d copies: %v\n", g["title"], g["count"], g["ids"])
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&tolerance, "tolerance", "exact", "Match tolerance: exact, fuzzy, broad")
	return cmd
}

func normalizeForDup(title, authors any, tolerance string) string {
	t := strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", title)))
	a := strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", authors)))
	if t == "" || t == "<nil>" {
		return ""
	}
	if tolerance == "broad" && len([]rune(t)) > 30 {
		t = string([]rune(t)[:30])
	}
	if tolerance != "exact" {
		t = stripPunctuation(t)
		a = stripPunctuation(a)
	}
	return t + "||" + a
}

func stripPunctuation(s string) string {
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == ' ' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

var _ = os.Stdout
