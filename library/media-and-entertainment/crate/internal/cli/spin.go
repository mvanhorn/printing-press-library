// Copyright 2026 justinwfu and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/crate/internal/crate"

	"github.com/spf13/cobra"
)

// spin reads the shelf out of the local store and syncs it once from
// Discogs if it has never been synced, then picks locally.
// pp:data-source auto
func newNovelSpinCmd(flags *rootFlags) *cobra.Command {
	var (
		user   string
		genre  string
		style  string
		label  string
		artist string
		decade string
		format string
		seed   int64
		anyRec bool
	)
	cmd := &cobra.Command{
		Use:   "spin",
		Short: "Chooses a record off your own shelf to play tonight, and says why it picked that one",
		Long: strings.Trim(`
Pick something to play from records you already own.

Unrated records are preferred, because an unrated record is the closest signal
Discogs offers for one you have not sat with. Pass --any to draw from the whole
matching shelf instead.

The pick is deterministic for a given --seed, so a choice can be reproduced;
without one the day's date is used, which means the same record all day and a
new one tomorrow.
`, "\n"),
		Example: strings.Trim(`
  crate-pp-cli spin --user example
  crate-pp-cli spin --user <username> --genre Jazz --decade 1960s
  crate-pp-cli spin --user <username> --label "Blue Note" --json
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

			recs, err := loadShelf(ctx, cmd, c, h, u, false)
			if err != nil {
				return err
			}

			// Unrated is a preference, not a filter, so it is passed to Pick
			// rather than set on the Filter: Pick can then fall back to the
			// whole pool when every record is rated. --any turns the
			// preference off entirely.
			f := crate.Filter{
				Genre: genre, Style: style, Label: label,
				Artist: artist, Decade: decade, Format: format,
			}

			s := seed
			if s == 0 {
				y, m, d := time.Now().Date()
				s = int64(y*10000 + int(m)*100 + d)
			}

			pick, reason, pool, ok := crate.Pick(recs, f, s, !anyRec)
			if !ok {
				return notFoundErr(fmt.Errorf(
					"nothing on the shelf matches those filters (%d records synced)", len(recs)))
			}
			if anyRec {
				reason = "drawn from the whole matching shelf (--any)"
			}

			res := map[string]any{
				"release_id": pick.ReleaseID,
				"title":      pick.Title,
				"artists":    pick.Artists,
				"year":       pick.Year,
				"labels":     pick.Labels,
				"genres":     pick.Genres,
				"styles":     pick.Styles,
				"rating":     pick.Rating,
				"reason":     reason,
				"pool":       pool,
				"seed":       s,
			}
			if flags.asJSON {
				return flags.printJSON(cmd, res)
			}
			out := cmd.OutOrStdout()
			fmt.Fprintln(out, bold(fmt.Sprintf("%s — %s", pick.ArtistLine(), pick.Title)))
			if pick.Year > 0 {
				fmt.Fprintf(out, "  %-10s %d\n", "year", pick.Year)
			}
			if len(pick.Labels) > 0 {
				fmt.Fprintf(out, "  %-10s %s\n", "label", strings.Join(pick.Labels, ", "))
			}
			if len(pick.Styles) > 0 {
				fmt.Fprintf(out, "  %-10s %s\n", "style", strings.Join(pick.Styles, ", "))
			}
			fmt.Fprintf(out, "  %-10s %s\n", "why", reason)
			fmt.Fprintf(out, "  %-10s %d records matched; seed %d\n", "pool", pool, s)
			fmt.Fprintf(out, "  %-10s https://www.discogs.com/release/%d\n", "discogs", pick.ReleaseID)
			return nil
		},
	}
	cmd.Flags().StringVar(&user, "user", "", userFlagHelp)
	cmd.Flags().StringVar(&genre, "genre", "", "Only consider this genre, e.g. Jazz, Electronic, Rock")
	cmd.Flags().StringVar(&style, "style", "", "Only consider this style, e.g. Hard Bop, Ambient, Post Punk")
	cmd.Flags().StringVar(&label, "label", "", "Only consider records on this label")
	cmd.Flags().StringVar(&artist, "artist", "", "Only consider this artist")
	cmd.Flags().StringVar(&decade, "decade", "", "Only consider this decade, e.g. 1970 or 1970s")
	cmd.Flags().StringVar(&format, "format", "", "Only consider this format, e.g. Vinyl, LP, 7\"")
	cmd.Flags().Int64Var(&seed, "seed", 0, "Reproduce a specific pick; defaults to today's date")
	cmd.Flags().BoolVar(&anyRec, "any", false, "Draw from every matching record, not just unrated ones")
	return cmd
}
