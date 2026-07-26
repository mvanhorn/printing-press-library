// Copyright 2026 justinwfu and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// dig always queries the live Discogs search endpoint; the local store is
// consulted only to subtract the records already on the shelf.
// pp:data-source live
func newNovelDigCmd(flags *rootFlags) *cobra.Command {
	var (
		user    string
		label   string
		genre   string
		style   string
		artist  string
		year    string
		country string
		format  string
		limit   int
		pages   int
	)
	cmd := &cobra.Command{
		Use:   "dig",
		Short: "Lists records matching a label, genre, style, or year that you do NOT already own",
		Long: strings.Trim(`
Crate-digging with your shelf subtracted.

Runs a Discogs search and removes everything already in your synced collection,
so what is left is only what you could actually buy. Neither the API nor any
wrapper can do this subtraction, because only this CLI knows what you own.

Sync first, or every result will look like a gap.
`, "\n"),
		Example: strings.Trim(`
  crate-pp-cli dig --user example --label "Blue Note" --limit 10
  crate-pp-cli dig --user example --genre Jazz --year 1960-1969
  crate-pp-cli dig --user example --artist "Alice Coltrane" --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			u, err := resolveUser(user)
			if err != nil {
				return err
			}
			if label == "" && genre == "" && style == "" && artist == "" &&
				year == "" && country == "" && format == "" {
				return usageErr(fmt.Errorf(
					"give the dig a direction: --label, --genre, --style, --artist, --year, --country, or --format"))
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			h, err := openCrate(ctx)
			if err != nil {
				return err
			}
			defer h.closeFn()

			// Load through loadShelf so a first run syncs itself. Digging
			// against an unsynced collection would report every result as a
			// gap, which is the one answer guaranteed to be wrong.
			if _, err := loadShelf(ctx, cmd, c, h, u, false); err != nil {
				return err
			}
			owned, err := h.store.OwnedIDs(ctx, u)
			if err != nil {
				return err
			}

			params := map[string]string{"type": "release"}
			for k, v := range map[string]string{
				"label": label, "genre": genre, "style": style,
				"artist": artist, "year": year, "country": country, "format": format,
			} {
				if v != "" {
					params[k] = v
				}
			}

			rows, _, truncated, err := pageFetch(ctx, c, "/database/search", params, pages)
			if err != nil {
				return err
			}

			// Discogs search results key the release as "id", not
			// "release_id" — decoding the wrong field silently drops every
			// row and reports an empty dig as "you own it all".
			type hit struct {
				ReleaseID int64    `json:"id"`
				Title     string   `json:"title"`
				Year      string   `json:"year"`
				Label     []string `json:"label"`
				Format    []string `json:"format"`
				Country   string   `json:"country"`
			}
			var gaps []hit
			seen := map[int64]bool{}
			var scanned, unique, already, dupes int
			for _, raw := range rows {
				var r hit
				if err := json.Unmarshal(raw, &r); err != nil || r.ReleaseID == 0 {
					continue
				}
				scanned++
				// Discogs repeats a release across search pages. Dedupe
				// before counting anything: deduping after the owned check
				// counts a repeated owned release twice, and leaves the
				// headline's denominator larger than owned + gaps, so the
				// two lines contradict each other.
				if seen[r.ReleaseID] {
					dupes++
					continue
				}
				seen[r.ReleaseID] = true
				unique++
				if owned[r.ReleaseID] {
					already++
					continue
				}
				gaps = append(gaps, r)
			}

			shown := gaps
			if limit > 0 && len(shown) > limit {
				shown = shown[:limit]
			}

			if flags.asJSON {
				return flags.printJSON(cmd, map[string]any{
					"user": u, "scanned": scanned, "unique": unique,
					"duplicates": dupes, "already_owned": already,
					"gaps": shown, "gap_total": len(gaps), "truncated": truncated,
				})
			}

			out := cmd.OutOrStdout()
			if scanned == 0 {
				// An empty search and a fully-owned search are different
				// answers and must not print the same line.
				fmt.Fprintln(out, bold("the search matched nothing on Discogs"))
				fmt.Fprintln(out, "  widen the dig, or check the spelling of --label / --artist")
				return nil
			}
			fmt.Fprintln(out, bold(fmt.Sprintf("%d of %d releases are not on %s's shelf", len(gaps), unique, u)))
			if len(shown) == 0 {
				fmt.Fprintf(out, "  you already own all %d matching releases\n", unique)
				return nil
			}
			rowsOut := make([][]string, 0, len(shown))
			for _, g := range shown {
				rowsOut = append(rowsOut, []string{
					// Discogs repeats the label once per pressing and the
					// format once per component, so an undeduped firstN
					// renders "Blue Note, Blue Note" on nearly every row.
					g.Title, g.Year, strings.Join(firstN(dedupeStrings(g.Label), 2), ", "),
					strings.Join(firstN(dedupeStrings(g.Format), 2), ", "),
					fmt.Sprintf("%d", g.ReleaseID),
				})
			}
			if err := flags.printTable(cmd, []string{"TITLE", "YEAR", "LABEL", "FORMAT", "RELEASE"}, rowsOut); err != nil {
				return err
			}
			fmt.Fprintf(out, "\n%d already owned and filtered out.\n", already)
			if dupes > 0 {
				fmt.Fprintf(out, "%d repeated listings collapsed (%d rows returned).\n", dupes, scanned)
			}
			if len(gaps) > len(shown) {
				fmt.Fprintf(out, "Showing %d of %d; raise --limit for more.\n", len(shown), len(gaps))
			}
			if truncated {
				// On stdout as well as stderr: the headline count reads as a
				// complete answer, and a stderr-only caveat vanishes the
				// moment anyone pipes the output.
				fmt.Fprintf(out, "Search stopped at --pages=%d, so this is not the full catalogue.\n", pages)
				fmt.Fprintf(cmd.ErrOrStderr(),
					"warning: stopped at --pages=%d, so the search was not exhausted\n", pages)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&user, "user", "", userFlagHelp)
	cmd.Flags().StringVar(&label, "label", "", "Record label to dig, e.g. Blue Note")
	cmd.Flags().StringVar(&genre, "genre", "", "Genre to dig, e.g. Jazz, Electronic")
	cmd.Flags().StringVar(&style, "style", "", "Style to dig, e.g. Hard Bop, Ambient")
	cmd.Flags().StringVar(&artist, "artist", "", "Artist whose discography to dig")
	cmd.Flags().StringVar(&year, "year", "", "Year or range, e.g. 1967 or 1960-1969")
	cmd.Flags().StringVar(&country, "country", "", "Country of release, e.g. US, UK, Japan")
	cmd.Flags().StringVar(&format, "format", "", "Format, e.g. Vinyl, LP, Album")
	cmd.Flags().IntVar(&limit, "limit", 20, "Show only the first N gaps (0 for all)")
	cmd.Flags().IntVar(&pages, "pages", 3, "How many pages of 100 search results to scan")
	return cmd
}

func firstN(v []string, n int) []string {
	if len(v) <= n {
		return v
	}
	return v[:n]
}

// dedupeStrings drops repeats while keeping first-seen order. Discogs lists a
// label once per pressing and a format once per component, so the raw slices
// are full of duplicates that would otherwise fill the two visible columns
// with the same word twice.
func dedupeStrings(v []string) []string {
	if len(v) < 2 {
		return v
	}
	seen := make(map[string]bool, len(v))
	out := make([]string, 0, len(v))
	for _, s := range v {
		if seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}
