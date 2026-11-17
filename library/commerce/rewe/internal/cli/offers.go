// Copyright 2026 JimPresting and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/rewe/internal/rewe"
)

func newNovelOffersCmd(flags *rootFlags) *cobra.Command {
	var search, category, week, page string
	var limit, maxLazy int
	cmd := &cobra.Command{
		Use:   "offers",
		Short: "Current offers of your market (and next week's once REWE publishes them) as rows with title, price and category.",
		Long: "Loads the offers page of the selected market (or --page), renders the lazily loaded offer\n" +
			"tiles, stores them locally per ISO week and prints them. --week this|next|YYYY/WW filters\n" +
			"the week, --search matches title and details, --category matches the category slug.",
		Example:     "  rewe-pp-cli offers --search kaffee --json\n  rewe-pp-cli offers --week next --category obst-gemuese",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "offers")
			}
			sh, err := openShopAnonymous()
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			if page == "" {
				if page, err = sh.OffersURL(ctx); err != nil {
					return reweErr(err)
				}
			}
			all, err := sh.Offers(ctx, page, maxLazy)
			if err != nil && len(all) == 0 {
				return reweErr(err)
			}
			_ = recordOffers(ctx, all)
			rows := filterOffers(all, search, category, week)
			if limit > 0 && len(rows) > limit {
				rows = rows[:limit]
			}
			if err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: some offer tiles did not load: %v\n", err)
			}
			return emit(cmd, flags, rows, func(w io.Writer) {
				for _, o := range rows {
					fmt.Fprintf(w, "%-8s %-45s %-9s %s\n", o.Week, trunc(o.Title, 45), o.Price, trunc(o.Details, 60))
				}
				fmt.Fprintf(w, "%d offers\n", len(rows))
			})
		},
	}
	cmd.Flags().StringVar(&search, "search", "", "Match offer title or details, e.g. kaffee")
	cmd.Flags().StringVar(&category, "category", "", "Category slug substring, e.g. obst")
	cmd.Flags().StringVar(&week, "week", "", "this, next or an ISO week like 2026/48 (default: all)")
	cmd.Flags().StringVar(&page, "page", "", "Offers page path, e.g. /angebote/koeln/3200008/rewe-neumarkt-35-37/ (default: selected market)")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum rows (0 = all)")
	cmd.Flags().IntVar(&maxLazy, "max-lazy", 400, "Maximum lazily loaded tiles to render (keeps request volume low)")
	return cmd
}

func trunc(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

func filterOffers(all []rewe.Offer, search, category, week string) []rewe.Offer {
	weeks := map[string]bool{}
	for _, o := range all {
		weeks[o.Week] = true
	}
	sorted := []string{}
	for w := range weeks {
		sorted = append(sorted, w)
	}
	sort.Strings(sorted)
	switch strings.ToLower(week) {
	case "this":
		if len(sorted) > 0 {
			week = sorted[0]
		}
	case "next":
		if len(sorted) > 1 {
			week = sorted[1]
		} else {
			week = "none"
		}
	}
	q := strings.Fields(strings.ToLower(search))
	out := []rewe.Offer{}
	for _, o := range all {
		if week != "" && o.Week != week {
			continue
		}
		if category != "" && !strings.Contains(strings.ToLower(o.Category), strings.ToLower(category)) {
			continue
		}
		hay := strings.ToLower(o.Title + " " + o.Details)
		ok := true
		for _, t := range q {
			if !strings.Contains(hay, t) {
				ok = false
				break
			}
		}
		if ok {
			out = append(out, o)
		}
	}
	return out
}
