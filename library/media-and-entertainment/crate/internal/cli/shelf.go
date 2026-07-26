// Copyright 2026 justinwfu and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/crate/internal/crate"

	"github.com/spf13/cobra"
)

// shelf tallies the locally stored shelf, syncing it once from Discogs
// if it has never been synced.
// pp:data-source auto
func newNovelShelfCmd(flags *rootFlags) *cobra.Command {
	var (
		user  string
		by    string
		limit int
		wants bool
	)
	cmd := &cobra.Command{
		Use:   "shelf",
		Short: "Breaks your collection down by decade, genre, style, label, format, or artist",
		Long: strings.Trim(`
What the collection actually is.

Counts every synced record along one dimension. Genres, styles, labels,
artists, and formats are multi-valued, so a record with two genres is counted
under both and the counts deliberately add up to more than the number of
records; the percentage is always against the record count, and the output says
which case applies.

Records with no release year are left out of the decade breakdown rather than
being bucketed into a decade that does not exist.
`, "\n"),
		Example: strings.Trim(`
  crate-pp-cli shelf --user example --by decade
  crate-pp-cli shelf --user example --by label --limit 15
  crate-pp-cli shelf --user <username> --by style --wantlist --json
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

			recs, err := loadShelf(ctx, cmd, c, h, u, wants)
			if err != nil {
				return err
			}

			dim := crate.Dimension(strings.ToLower(strings.TrimSpace(by)))
			tallies, err := crate.Breakdown(recs, dim)
			if err != nil {
				return usageErr(err)
			}
			shown := tallies
			if limit > 0 && len(shown) > limit {
				shown = shown[:limit]
			}

			if flags.asJSON {
				return flags.printJSON(cmd, map[string]any{
					"user": u, "by": string(dim), "records": len(recs),
					"multi_valued": crate.IsMultiValued(dim),
					"buckets":      shown, "bucket_total": len(tallies),
				})
			}

			out := cmd.OutOrStdout()
			kind := "collection"
			if wants {
				kind = "wantlist"
			}
			fmt.Fprintln(out, bold(fmt.Sprintf("%s — %s by %s (%d records)", u, kind, dim, len(recs))))

			rows := make([][]string, 0, len(shown))
			for _, t := range shown {
				rows = append(rows, []string{t.Key, fmt.Sprintf("%d", t.Count), fmt.Sprintf("%.0f%%", t.Share*100)})
			}
			if err := flags.printTable(cmd, []string{strings.ToUpper(string(dim)), "COUNT", "SHARE"}, rows); err != nil {
				return err
			}
			// Only say the counts overlap when they actually do. A record can
			// carry several genres, but if none of these do, printing "the
			// shares do not sum to 100%" directly under a table reading 100%
			// is simply wrong.
			var countSum int
			for _, t := range tallies {
				countSum += t.Count
			}
			if crate.IsMultiValued(dim) && countSum > len(recs) {
				fmt.Fprintf(out, "\n%s is multi-valued: %d records carry %d %s between them, so the shares overlap and do not sum to 100%%.\n",
					dim, len(recs), countSum, dim)
			}
			if dim == crate.ByDecade {
				var placed int
				for _, t := range tallies {
					placed += t.Count
				}
				if placed < len(recs) {
					fmt.Fprintf(out, "\n%d record(s) have no release year and are not in any decade.\n", len(recs)-placed)
				}
			}
			if len(tallies) > len(shown) {
				fmt.Fprintf(out, "\nShowing %d of %d buckets; raise --limit for the rest.\n", len(shown), len(tallies))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&user, "user", "", userFlagHelp)
	cmd.Flags().StringVar(&by, "by", "decade", "Dimension to count by: decade, genre, style, label, format, artist, or year")
	cmd.Flags().IntVar(&limit, "limit", 20, "Show only the top N buckets (0 for all)")
	cmd.Flags().BoolVar(&wants, "wantlist", false, "Break down the wantlist instead of the collection")
	return cmd
}
