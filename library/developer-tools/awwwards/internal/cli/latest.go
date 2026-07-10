// Copyright 2026 and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/awwwards/internal/cliutil"
)

func newLatestCmd(flags *rootFlags) *cobra.Command {
	var pages int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "latest",
		Short: "The newest award entries as parsed JSON cards - the feed Awwwards never shipped",
		Long: strings.Trim(`
Latest fetches the front of the /websites/ feed live and returns fully parsed
cards: slug, title, tags (including tech stack), created date, and a
ready-to-fetch thumbnail URL. Fetched cards are also stored into the local
design mirror opportunistically, so repeated use keeps analytics fresh.

Use this command for "what just won" checks. Do NOT use it for filtered or
historical queries; use 'find' (local mirror) or 'websites browse' (live raw
listing) instead.
`, "\n"),
		Example: strings.Trim(`
  awwwards-pp-cli latest --json
  awwwards-pp-cli latest --pages 2 --json --select slug,title,tags
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would fetch %d page(s) of the latest award entries\n", pages)
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			if pages < 1 {
				pages = 1
			}
			if pages > 5 {
				pages = 5
			}
			if cliutil.IsDogfoodEnv() && pages > 1 {
				pages = 1
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			if dbPath == "" {
				dbPath = defaultDBPath("awwwards-pp-cli")
			}
			mirrorDB, dbErr := openMirror(ctx, dbPath)
			if dbErr != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: mirror unavailable (%v); results will not be cached\n", dbErr)
			} else {
				defer mirrorDB.Close()
			}

			views := make([]cardView, 0, pages*30)
			for page := 1; page <= pages; page++ {
				cards, err := fetchListingCards(ctx, c, "", page, "")
				if err != nil {
					if page == 1 {
						return classifyAPIError(err, flags)
					}
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: page %d fetch failed (%v); returning %d cards from earlier pages\n", page, err, len(views))
					break
				}
				if len(cards) == 0 {
					break
				}
				for _, card := range cards {
					views = append(views, cardToView(card))
				}
				// Opportunistic mirror write: best-effort, never fails the read.
				if mirrorDB != nil {
					if upErr := mirrorDB.UpsertAwCards(ctx, cards, ""); upErr != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "warning: mirror write failed: %v\n", upErr)
					}
				}
			}

			return printJSONFiltered(cmd.OutOrStdout(), views, flags)
		},
	}

	cmd.Flags().IntVar(&pages, "pages", 1, "Pages to fetch (30 cards per page, max 5)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path for the opportunistic mirror write")
	return cmd
}
