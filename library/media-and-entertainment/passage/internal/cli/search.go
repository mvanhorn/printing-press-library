// Copyright 2026 justinwfu and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Federated book search: Open Library (metadata) blended with Project Gutenberg
// (Gutendex) so results flag which books have free full text you can sit with.
package cli

import (
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/passage/internal/source/gutendex"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/passage/internal/source/openlibrary"

	"github.com/spf13/cobra"
)

func newSearchCmd(flags *rootFlags) *cobra.Command {
	var limit int
	var freeOnly bool

	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search books across Open Library and Project Gutenberg, flagging free full text",
		Long: `search federates Open Library (metadata) and Project Gutenberg (Gutendex full
texts). Each result shows its source and, when a public-domain text exists, the
Gutenberg id you can read with 'passage sit <id>'. Use --free-only to show just
the books you can sit with right now.`,
		Example:     "  passage search \"marcus aurelius\" --free-only",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			query := strings.TrimSpace(strings.Join(args, " "))
			if query == "" {
				return fmt.Errorf("search needs a query — e.g. passage search \"stoicism\"")
			}
			if limit <= 0 {
				limit = 20
			}
			ctx := cmd.Context()

			olDocs, olErr := openlibrary.New().Search(ctx, query, limit)
			gutBooks, gutErr := gutClient().Search(ctx, query, false)
			if olErr != nil && gutErr != nil {
				return fmt.Errorf("both sources failed: open library: %v; gutendex: %v", olErr, gutErr)
			}

			type row struct {
				Title       string `json:"title"`
				Author      string `json:"author"`
				Year        int    `json:"year,omitempty"`
				Source      string `json:"source"`
				FreeText    bool   `json:"free_text"`
				GutenbergID int    `json:"gutenberg_id,omitempty"`
				Key         string `json:"key,omitempty"`
				SitCmd      string `json:"sit_cmd,omitempty"`
			}

			rows := []row{}
			claimed := map[string]bool{} // normalized titles already emitted

			for _, d := range olDocs {
				nt := normTitle(d.Title)
				r := row{Title: d.Title, Author: d.Author(), Year: d.Year, Source: "openlibrary",
					Key: strings.TrimPrefix(d.Key, "/works/")}
				if gb, ok := matchGutenberg(nt, d.Author(), gutBooks); ok {
					r.Source = "both"
					r.FreeText = true
					r.GutenbergID = gb.ID
					r.SitCmd = fmt.Sprintf("passage sit %d", gb.ID)
					claimed[normTitle(gb.Title)] = true
				} else if d.HasFulltext || d.EbookAccess == "public" {
					r.FreeText = true // an Internet Archive scan exists (not Gutenberg-sit-able)
				}
				rows = append(rows, r)
				claimed[nt] = true
			}

			// Append public-domain Gutenberg texts Open Library didn't surface.
			olCount := len(rows)
			for _, b := range gutBooks {
				nt := normTitle(b.Title)
				if claimed[nt] || b.TextURL() == "" {
					continue
				}
				rows = append(rows, row{Title: b.Title, Author: b.AuthorLine(), Source: "gutenberg",
					FreeText: true, GutenbergID: b.ID, SitCmd: fmt.Sprintf("passage sit %d", b.ID)})
				claimed[nt] = true
			}

			if freeOnly {
				kept := rows[:0:0]
				for _, r := range rows {
					if r.GutenbergID > 0 { // "sit with" = has a Gutenberg full text
						kept = append(kept, r)
					}
				}
				rows = kept
				if len(rows) > limit {
					rows = rows[:limit]
				}
			} else if len(rows) > limit {
				// Reserve up to half the limit for Gutenberg-only public-domain
				// texts Open Library never surfaced — the whole point of the
				// federation. Otherwise a verbose OL result set truncates them all.
				olRows, gutOnly := rows[:olCount], rows[olCount:]
				keepGut := len(gutOnly)
				if keepGut > limit/2 {
					keepGut = limit / 2
				}
				keepOL := limit - keepGut
				if keepOL > len(olRows) {
					keepOL = len(olRows)
					if keepGut > limit-keepOL {
						keepGut = limit - keepOL
					}
					if keepGut > len(gutOnly) {
						keepGut = len(gutOnly)
					}
				}
				merged := append([]row{}, olRows[:keepOL]...)
				rows = append(merged, gutOnly[:keepGut]...)
			}

			table := make([][]string, 0, len(rows))
			for _, r := range rows {
				free := "-"
				switch {
				case r.GutenbergID > 0:
					free = fmt.Sprintf("sit %d", r.GutenbergID)
				case r.FreeText:
					free = "scan"
				}
				year := ""
				if r.Year > 0 {
					year = fmt.Sprintf("%d", r.Year)
				}
				table = append(table, []string{r.Title, r.Author, year, r.Source, free})
			}
			return bookRender(cmd, flags, rows, []string{"Title", "Author", "Year", "Source", "Free"}, table)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 0, "Max results (default 20)")
	cmd.Flags().BoolVar(&freeOnly, "free-only", false, "Only books with a Gutenberg full text you can sit with")
	return cmd
}

// normTitle normalizes a title for cross-source matching: lowercase, any
// non-alphanumeric run becomes a single space, drop a leading "the ".
func normTitle(s string) string {
	var b strings.Builder
	prevSpace := false
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			prevSpace = false
		} else if !prevSpace {
			b.WriteByte(' ')
			prevSpace = true
		}
	}
	out := strings.TrimSpace(b.String())
	return strings.TrimPrefix(out, "the ")
}

// prefixWord reports whether short is a whole-word prefix of long ("walden" is a
// prefix-word of "walden or life in the woods", but not of "after walden").
func prefixWord(long, short string) bool {
	return strings.HasPrefix(long, short+" ")
}

// matchGutenberg finds a Gutenberg book whose normalized title matches the given
// normalized Open Library title exactly or as a whole-word prefix (title
// variants extend the canonical title) AND whose author agrees — so a title
// collision ("Essays" by Emerson vs Bacon) doesn't link to the wrong text.
// gutBooks is most-downloaded-first, so the first match is the most canonical.
func matchGutenberg(olNorm, olAuthor string, gutBooks []gutendex.Book) (gutendex.Book, bool) {
	if len(olNorm) < 4 {
		return gutendex.Book{}, false
	}
	for _, b := range gutBooks {
		gn := normTitle(b.Title)
		if len(gn) < 4 {
			continue
		}
		titleOK := gn == olNorm || prefixWord(olNorm, gn) || prefixWord(gn, olNorm)
		if titleOK && authorsAgree(olAuthor, b.AuthorLine()) {
			return b, true
		}
	}
	return gutendex.Book{}, false
}

// authorsAgree reports whether two author strings share a name token (OL gives
// "Jane Austen", Gutendex "Austen, Jane"). Unknown/empty authors can't disprove
// a title match, so they agree by default.
func authorsAgree(a, b string) bool {
	at, bt := authorTokens(a), authorTokens(b)
	if len(at) == 0 || len(bt) == 0 {
		return true // one side unknown — can't disprove the title match
	}
	for t := range bt {
		if at[t] {
			return true
		}
	}
	return false
}

var authorStop = map[string]bool{"the": true, "and": true, "sir": true, "von": true,
	"van": true, "emperor": true, "rome": true, "jr": true, "translated": true, "editor": true}

func authorTokens(s string) map[string]bool {
	if s == "" || strings.EqualFold(s, "unknown") {
		return nil
	}
	toks := map[string]bool{}
	for _, f := range strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !(r >= 'a' && r <= 'z')
	}) {
		if len(f) >= 3 && !authorStop[f] {
			toks[f] = true
		}
	}
	return toks
}
