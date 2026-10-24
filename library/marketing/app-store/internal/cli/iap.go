// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/cliutil"

	"github.com/spf13/cobra"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newIAPCmd(flags))
	})
}

// iapRow is one in-app purchase price row of one storefront's product page
// (the page lists the top 10). price is Apple's localized text; currency is
// the storefront's ISO code from Lookup.
type iapRow struct {
	Storefront string `json:"storefront"`
	Position   int    `json:"position"`
	Name       string `json:"name"`
	Price      string `json:"price"`
	Currency   string `json:"currency"`
}

// iapStorefront summarizes one storefront: whether the app is sold there and
// how many IAP rows its page lists.
type iapStorefront struct {
	Storefront        string `json:"storefront"`
	Available         bool   `json:"available"`
	HasInAppPurchases bool   `json:"hasInAppPurchases"`
	IAPCount          int    `json:"iapCount"`
}

type iapMeta struct {
	App         string          `json:"app"`
	Title       string          `json:"title"`
	Storefronts []iapStorefront `json:"storefronts"`
	Unavailable []string        `json:"unavailable"`
	Requests    int             `json:"requests"`
	Notes       []string        `json:"notes"`
}

func newIAPCmd(flags *rootFlags) *cobra.Command {
	var countries string
	cmd := &cobra.Command{
		Use:   "iap <app>",
		Short: "Fetch in-app purchase names and prices (top 10) live per storefront",
		Long: strings.Trim(`
List the in-app purchases an app's product page shows (Apple lists the top 10
by popularity): position, name and localized price, per storefront. With
--countries the same app is read in several storefronts (two requests each:
Lookup + page) for regional pricing. Product pages are paced about 20 s apart,
across runs, so each extra storefront adds about 20 s; raise --timeout for more
than two. The store keeps a new price list only when it changes, so changes
can report IAP price moves.

JSON is {meta, items, fetch_failures}: items holds the price rows, meta lists
each storefront (available, IAP count), storefronts where the app is not sold
and notes; fetch_failures lists storefronts whose fetch failed (warned on
stderr and never turned into rows). A storefront whose page lists no IAP
contributes no rows and a note. Not sold in any requested storefront exits 3.`, "\n"),
		Example: strings.Trim(`
  app-store-pp-cli iap 553834731 --json
  app-store-pp-cli iap 553834731 --countries us,gb,jp,pk --select items.storefront,items.name,items.price`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:data-source":      "live",
			"pp:typed-exit-codes": "0,7",
			"pp:happy-args":       "app=553834731",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "iap")
			}
			ref, err := s2aOneAppArg(cmd, args)
			if err != nil {
				return err
			}
			list, err := s2aStorefronts(cmd, flags, countries, &ref)
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

			meta := iapMeta{
				App:         s2aRefLabel(ref),
				Storefronts: make([]iapStorefront, 0, len(list)),
				Unavailable: make([]string, 0),
				Notes:       make([]string, 0),
			}
			rows := make([]iapRow, 0, 10*len(list))
			failures := make([]fetchFailure, 0)
			errs := make([]error, 0)
			available := 0
			// stopped: a throttle or pacing refusal ended the fan-out early;
			// what was gathered still prints, then exit 7.
			var stopped error
			for i, cc := range list {
				lk, _, found, err := s2aLookupIn(ctx, c, ref, cc)
				if err == nil && !found {
					meta.Unavailable = append(meta.Unavailable, cc)
					meta.Storefronts = append(meta.Storefronts, iapStorefront{Storefront: cc})
					continue
				}
				if err == nil {
					page, perr := s2aProductPage(ctx, c, lk, cc, "")
					if perr == nil {
						available++
						if meta.Title == "" {
							meta.Title = lk.TrackName
						}
						meta.App = lk.AppID()
						s2aWarnStore(cmd, "the "+cc+" IAP prices", s2aRecordProductPage(ctx, cmd, st, cc, page, lk))
						for _, it := range page.InAppPurchases {
							rows = append(rows, iapRow{Storefront: cc, Position: it.Position, Name: it.Name, Price: it.Price, Currency: lk.Currency})
						}
						meta.Storefronts = append(meta.Storefronts, iapStorefront{
							Storefront:        cc,
							Available:         true,
							HasInAppPurchases: page.HasInAppPurchases || len(page.InAppPurchases) > 0,
							IAPCount:          len(page.InAppPurchases),
						})
						if len(page.InAppPurchases) == 0 {
							meta.Notes = append(meta.Notes, fmt.Sprintf("%s: the product page lists no in-app purchase prices", cc))
						}
						continue
					}
					err = perr
				}
				failures = append(failures, fetchFailure{ID: meta.App, Storefront: cc, Error: err.Error()})
				if isRateLimited(err) || ctx.Err() != nil {
					stopped = err
					failures = append(failures, s2aNotAttempted(meta.App, list[i+1:], cc)...)
					break
				}
				errs = append(errs, err)
			}
			meta.Requests = c.Requests()
			if stopped != nil && available == 0 {
				return s2aStopped(stopped)
			}
			if err := s2aFanoutOutcome(len(list), available, errs, func() error {
				return notFoundErr(fmt.Errorf("no App Store app %s in storefront(s) %s (check the id, or try other --countries)", s2aRefLabel(ref), strings.Join(list, ",")))
			}); err != nil {
				return err
			}
			if len(meta.Unavailable) > 0 {
				meta.Notes = append(meta.Notes, fmt.Sprintf("not sold in: %s", strings.Join(meta.Unavailable, ",")))
			}
			warnFetchFailures(cmd.ErrOrStderr(), len(failures), len(list), "storefronts")
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				for _, n := range meta.Notes {
					s2aPrintln(cmd.ErrOrStderr(), "note: "+n)
				}
				if err := printJSONFiltered(cmd.OutOrStdout(), s2aEnvelope{Meta: meta, Items: rows, FetchFailures: failures}, flags); err != nil {
					return err
				}
				return s2aStopped(stopped)
			}
			w := cmd.OutOrStdout()
			s2aPrintf(w, "%s (%s) in-app purchases\n", cliutil.ScrubTerminal(meta.Title), meta.App)
			if len(rows) > 0 {
				table := make([][]string, 0, len(rows))
				for _, r := range rows {
					table = append(table, []string{r.Storefront, fmt.Sprint(r.Position), s2aOneLine(r.Name, 50), r.Price})
				}
				if err := s2aTable(w, []string{"storefront", "#", "name", "price"}, table); err != nil {
					return err
				}
			}
			for _, n := range meta.Notes {
				s2aPrintln(w, n)
			}
			s2aFailuresHuman(w, failures)
			return s2aStopped(stopped)
		},
	}
	cmd.Flags().StringVar(&countries, "countries", "", "Storefronts to compare, comma-separated (us,gb,jp) or all; empty uses --country")
	s2bDBFlag(cmd)
	return cmd
}
