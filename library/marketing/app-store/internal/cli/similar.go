// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newSimilarCmd(flags))
	})
}

// similarRow is one "You Might Also Like" card, in page order. ratingCount
// is Apple's display text ("1.6M"); exact counts come from 'compare'.
type similarRow struct {
	Position          int     `json:"position"`
	AppID             string  `json:"appId"`
	Title             string  `json:"title"`
	Subtitle          string  `json:"subtitle"`
	DeveloperName     string  `json:"developerName"`
	Rating            float64 `json:"rating"`
	RatingCount       string  `json:"ratingCount"`
	EditorsChoice     bool    `json:"editorsChoice"`
	PriceText         string  `json:"priceText"`
	IsFree            bool    `json:"isFree"`
	HasInAppPurchases bool    `json:"hasInAppPurchases"`
}

func newSimilarCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "similar <app>",
		Short: "List the apps Apple shows under \"You Might Also Like\" for an app, live, in page order",
		Long: strings.Trim(`
List the apps on an app's product page under "You Might Also Like", in the
order Apple shows them (about 10): app id, title, subtitle, developer, star
rating, Apple's display rating count, Editor's Choice and price. Read from the
product page; nothing is written to the local store.

Some apps (many of Apple's own) have no similar-apps shelf: the result is []
with a note. Use 'compare' on the returned ids for exact rating counts side by
side, 'developer' for the same developer's other apps. An unknown id exits 3.`, "\n"),
		Example: strings.Trim(`
  app-store-pp-cli similar 324684580 --json
  app-store-pp-cli similar 553834731 --limit 5 --select appId,title,rating`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:data-source":      "live",
			"pp:typed-exit-codes": "0,7",
			"pp:happy-args":       "app=324684580",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "similar")
			}
			ref, err := s2aOneAppArg(cmd, args)
			if err != nil {
				return err
			}
			if limit < 0 {
				return usageErr(fmt.Errorf("--limit must be 0 (all) or more; got %d", limit))
			}
			cc, err := refCountry(cmd, flags, ref)
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := newAppStoreClient(flags)
			if err != nil {
				return err
			}
			st, err := openAppStoreStore(cmd, flags)
			if err != nil {
				return err
			}
			defer func() { _ = st.Close() }()
			attachCaptureLog(c, st)

			lk, err := lookupAppRef(ctx, c, ref, cc)
			if err != nil {
				return err
			}
			page, err := s2aProductPage(ctx, c, lk, cc, "")
			if err != nil {
				return err
			}
			rows := make([]similarRow, 0, len(page.SimilarItems))
			for _, l := range page.SimilarItems {
				if limit > 0 && len(rows) == limit {
					break
				}
				rows = append(rows, similarRow{
					Position:          l.Position,
					AppID:             l.AdamID,
					Title:             l.Title,
					Subtitle:          l.Subtitle,
					DeveloperName:     l.DeveloperName,
					Rating:            l.Rating,
					RatingCount:       l.RatingCount,
					EditorsChoice:     l.EditorsChoice,
					PriceText:         l.PriceText,
					IsFree:            l.IsFree,
					HasInAppPurchases: l.HasIAP,
				})
			}
			if len(rows) == 0 {
				s2aNote(cmd, flags, fmt.Sprintf("the %s product page for %s has no \"You Might Also Like\" shelf", cc, lk.AppID()))
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}
			if len(rows) == 0 {
				return nil
			}
			table := make([][]string, 0, len(rows))
			for _, r := range rows {
				table = append(table, []string{fmt.Sprint(r.Position), r.AppID, s2aOneLine(r.Title, 40), s2aOneLine(r.DeveloperName, 28), s2aRating(r.Rating), r.RatingCount, r.PriceText})
			}
			return s2aTable(cmd.OutOrStdout(), []string{"#", "app id", "title", "developer", "rating", "ratings", "price"}, table)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum apps to return (0 = every app on the shelf, about 10)")
	s2bDBFlag(cmd)
	return cmd
}
