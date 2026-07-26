// Copyright 2026 justinwfu and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/crate/internal/client"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/crate/internal/crate"

	"github.com/spf13/cobra"
)

// newCrateSyncCmd mirrors a Discogs collection and wantlist into local SQLite.
//
// This is the foundation every other crate command reads from. It is separate
// from the generated `sync` command because collection endpoints take the
// username and folder as path parameters, which a generic resource sync cannot
// supply.
func newCrateSyncCmd(flags *rootFlags) *cobra.Command {
	var (
		user      string
		folder    int
		maxPages  int
		skipWants bool
		skipOwned bool
	)
	cmd := &cobra.Command{
		Use:   "shelf-sync",
		Short: "Mirror a Discogs collection and wantlist into the local database",
		Long: strings.Trim(`
Pulls every page of a user's collection and wantlist into local SQLite.

Everything else in this CLI reads from that local copy, because Discogs returns
one page at a time and computes nothing across pages. Public collections sync
without any credential; a private one needs DISCOGS_TOKEN set to that user's
own token.

A sync replaces rather than merges, so a record sold or removed upstream
disappears locally instead of lingering forever.
`, "\n"),
		Example: strings.Trim(`
  crate-pp-cli shelf-sync --user example
  crate-pp-cli shelf-sync --user example --skip-wantlist
`, "\n"),
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

			out := cmd.OutOrStdout()
			summary := map[string]int{}
			// Escape before interpolating into a URL path; an unescaped "?"
			// or "/" addresses a different endpoint entirely.
			esc := url.PathEscape(u)

			if !skipOwned {
				n, truncated, err := syncSide(ctx, c, h, u,
					fmt.Sprintf("/users/%s/collection/folders/%d/releases", esc, folder), false, maxPages)
				if err != nil {
					return err
				}
				summary["collection"] = n
				if !flags.asJSON {
					fmt.Fprintf(out, "collection: %d records\n", n)
				}
				if truncated {
					fmt.Fprintf(cmd.ErrOrStderr(),
						"warning: stopped at --max-pages=%d; the collection has more records than were synced\n", maxPages)
				}
			}
			if !skipWants {
				n, truncated, err := syncSide(ctx, c, h, u,
					fmt.Sprintf("/users/%s/wants", esc), true, maxPages)
				if err != nil {
					return err
				}
				summary["wantlist"] = n
				if !flags.asJSON {
					fmt.Fprintf(out, "wantlist:   %d records\n", n)
				}
				if truncated {
					fmt.Fprintf(cmd.ErrOrStderr(),
						"warning: stopped at --max-pages=%d; the wantlist has more records than were synced\n", maxPages)
				}
			}
			if flags.asJSON {
				return flags.printJSON(cmd, summary)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&user, "user", "", userFlagHelp)
	cmd.Flags().IntVar(&folder, "folder", 0, "Collection folder id; 0 is the All folder containing every record")
	cmd.Flags().IntVar(&maxPages, "max-pages", 50, "Stop after this many pages of 100 records (0 for no bound)")
	cmd.Flags().BoolVar(&skipWants, "skip-wantlist", false, "Sync only the collection")
	cmd.Flags().BoolVar(&skipOwned, "skip-collection", false, "Sync only the wantlist")
	return cmd
}

// syncSide pulls one side (collection or wantlist) into the store.
func syncSide(ctx context.Context, c *client.Client, h crateHandle, user, path string, wanted bool, maxPages int) (int, bool, error) {
	rows, _, truncated, err := pageFetch(ctx, c, path, nil, maxPages)
	if err != nil {
		return 0, false, err
	}
	recs := make([]crate.Record, 0, len(rows))
	for _, raw := range rows {
		if r, ok := decodeRecord(raw, wanted); ok {
			recs = append(recs, r)
		}
	}
	if err := h.store.ReplaceRecords(ctx, user, wanted, recs); err != nil {
		return 0, false, err
	}
	return len(recs), truncated, nil
}
