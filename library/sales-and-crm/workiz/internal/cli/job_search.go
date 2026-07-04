// Copyright 2026 Eldar and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

// pp:data-source local

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/spf13/cobra"
)

type noteSearchMatch struct {
	EntityType string `json:"entity_type"`
	EntityID   string `json:"entity_id"`
	Field      string `json:"field"`
	Snippet    string `json:"snippet"`
}

type noteSearchResult struct {
	Query   string            `json:"query"`
	Matches []noteSearchMatch `json:"matches"`
	Note    string            `json:"note,omitempty"`
}

func newNovelJobSearchCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:         "search <term>",
		Short:       "Search job notes, lead notes, and comments for free text across your entire synced history.",
		Long:        "Use this for free-text search inside notes/comments. For structured filtering by status/date/open, use the 'job list'/'lead list' flags instead.",
		Example:     "  workiz-pp-cli job search \"leak\" --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would search job/lead notes and comments in the local mirror")
				return nil
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("search term is required"))
			}
			term := strings.ToLower(args[0])

			ctx := cmd.Context()
			var bail bool
			empty := noteSearchResult{Query: args[0], Matches: []noteSearchMatch{}, Note: "no local mirror synced yet"}
			if dbPath, bail = checkNovelMirror(cmd, flags, dbPath, "job,lead", empty); bail {
				return nil
			}
			db, err := openNovelStore(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			jobs, err := loadJobs(ctx, db.DB())
			if err != nil {
				return fmt.Errorf("loading jobs: %w", err)
			}
			leads, err := loadLeads(ctx, db.DB())
			if err != nil {
				return fmt.Errorf("loading leads: %w", err)
			}

			matches := make([]noteSearchMatch, 0)
			for _, j := range jobs {
				if strings.Contains(strings.ToLower(j.JobNotes), term) {
					matches = append(matches, noteSearchMatch{EntityType: "job", EntityID: j.UUID, Field: "JobNotes", Snippet: snippetAround(j.JobNotes, term)})
				}
				for _, c := range j.Comments {
					if strings.Contains(strings.ToLower(c), term) {
						matches = append(matches, noteSearchMatch{EntityType: "job", EntityID: j.UUID, Field: "Comments", Snippet: snippetAround(c, term)})
					}
				}
			}
			for _, l := range leads {
				if strings.Contains(strings.ToLower(l.LeadNotes), term) {
					matches = append(matches, noteSearchMatch{EntityType: "lead", EntityID: l.UUID, Field: "LeadNotes", Snippet: snippetAround(l.LeadNotes, term)})
				}
				for _, c := range l.Comments {
					if strings.Contains(strings.ToLower(c), term) {
						matches = append(matches, noteSearchMatch{EntityType: "lead", EntityID: l.UUID, Field: "Comments", Snippet: snippetAround(c, term)})
					}
				}
			}

			result := noteSearchResult{Query: args[0], Matches: matches}
			if len(matches) == 0 {
				result.Note = fmt.Sprintf("no matches for %q in synced job/lead notes and comments", args[0])
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			if len(matches) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "no matches for %q\n", args[0])
				return nil
			}
			for _, m := range matches {
				fmt.Fprintf(cmd.OutOrStdout(), "%s %s (%s): %s\n", m.EntityType, m.EntityID, m.Field, m.Snippet)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/workiz-pp-cli/data.db)")
	return cmd
}

// snippetAround returns up to ~80 chars of context around the first match
// of term (case-insensitive) inside text, for human-readable search output.
//
// Operates entirely in rune space rather than mixing a byte-offset match
// index (from strings.Index on a separately-lowercased copy) with slicing
// on the original string: strings.ToLower can change a rune's UTF-8 byte
// length (e.g. U+0130 'İ' -> 'i'), which desyncs byte offsets between the
// two strings and can panic with a slice-bounds-out-of-range on real-world
// non-ASCII job/lead notes.
func snippetAround(text, term string) string {
	runes := []rune(text)
	termRunes := []rune(strings.ToLower(term))

	idx := -1
	for i := 0; i+len(termRunes) <= len(runes); i++ {
		match := true
		for j, tr := range termRunes {
			if unicode.ToLower(runes[i+j]) != tr {
				match = false
				break
			}
		}
		if match {
			idx = i
			break
		}
	}

	if idx == -1 {
		if len(runes) > 80 {
			return string(runes[:80]) + "..."
		}
		return text
	}
	start := idx - 30
	if start < 0 {
		start = 0
	}
	end := idx + len(termRunes) + 30
	if end > len(runes) {
		end = len(runes)
	}
	prefix, suffix := "", ""
	if start > 0 {
		prefix = "..."
	}
	if end < len(runes) {
		suffix = "..."
	}
	return prefix + string(runes[start:end]) + suffix
}
