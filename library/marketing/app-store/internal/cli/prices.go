// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/appstore"

	"github.com/spf13/cobra"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newPricesCmd(flags))
	})
}

// priceRow is one app in one storefront. An app Apple does not sell in the
// storefront is a row with available=false and null price fields.
type priceRow struct {
	Storefront     string   `json:"storefront"`
	App            string   `json:"app"`
	Title          string   `json:"title"`
	Available      bool     `json:"available"`
	Price          *float64 `json:"price"`
	Currency       *string  `json:"currency"`
	FormattedPrice *string  `json:"formattedPrice"`
}

type pricesMeta struct {
	Apps        []string `json:"apps"`
	Storefronts []string `json:"storefronts"`
	Requests    int      `json:"requests"`
	Notes       []string `json:"notes"`
}

func newPricesCmd(flags *rootFlags) *cobra.Command {
	var countries string
	cmd := &cobra.Command{
		Use:   "prices <app> [app...]",
		Short: "Check the price and availability of one or more apps live across storefronts",
		Long: strings.Trim(`
Show the price and availability of one or more apps in each storefront: one
batched iTunes Lookup per storefront covers up to 150 apps. An app Apple does
not sell in a storefront is a row with available=false and null price fields;
every available app is recorded as a lookup-only listing snapshot (stamped
with Apple's fetch time) so changes can report price moves.

JSON is {meta, items, fetch_failures}: one item per app per storefront;
storefronts whose Lookup failed are warned on stderr and listed in
fetch_failures, never turned into rows. Bundle ids are resolved to app ids
first (one request each). No requested app sold in any requested storefront
exits 3.

Use this for prices and availability; 'iap' for in-app purchase prices,
'compare' for ratings and versions side by side.`, "\n"),
		Example: strings.Trim(`
  app-store-pp-cli prices 324684580 553834731 --countries us,gb,jp --json
  app-store-pp-cli prices 932747118 --countries all --select items.storefront,items.formattedPrice`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:data-source":      "live",
			"pp:typed-exit-codes": "0,7",
			"pp:happy-args":       "app=324684580;app=553834731",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "prices")
			}
			refs, err := s2aAppArgs(cmd, args, 1)
			if err != nil {
				return err
			}
			var single *appstore.AppRef
			if len(refs) == 1 {
				single = &refs[0]
			}
			list, err := s2aStorefronts(cmd, flags, countries, single)
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

			failures := make([]fetchFailure, 0)
			ids, resolveFailures, resolveErrs, err := s2aResolveAppIDs(ctx, c, refs, list)
			if err != nil {
				return err
			}
			failures = append(failures, resolveFailures...)
			if len(ids) == 0 {
				if len(resolveErrs) > 0 {
					return resolveErrs[0]
				}
				return notFoundErr(fmt.Errorf("none of the apps was found in storefront(s) %s (check the ids, or try other --countries)", strings.Join(list, ",")))
			}

			meta := pricesMeta{Apps: ids, Storefronts: list, Notes: make([]string, 0)}
			rows := make([]priceRow, 0, len(ids)*len(list))
			errs := make([]error, 0)
			available := 0
			// stopped: a throttle or pacing refusal ended the fan-out early;
			// what was gathered still prints, then exit 7.
			var stopped error
			for i, cc := range list {
				resp, err := c.Lookup(ctx, appstore.LookupQuery{IDs: ids, Country: cc})
				if err != nil {
					err = classifyAppStoreErr(err)
					failures = append(failures, fetchFailure{Storefront: cc, Error: err.Error()})
					if isRateLimited(err) || ctx.Err() != nil {
						stopped = err
						failures = append(failures, s2aNotAttempted("", list[i+1:], cc)...)
						break
					}
					errs = append(errs, err)
					continue
				}
				byID := make(map[string]appstore.LookupResult, len(resp.Results))
				for _, r := range resp.Results {
					byID[r.AppID()] = r
				}
				for _, id := range ids {
					lk, ok := byID[id]
					if !ok {
						rows = append(rows, priceRow{Storefront: cc, App: id})
						continue
					}
					available++
					price, currency, formatted := lk.Price, lk.Currency, lk.FormattedPrice
					rows = append(rows, priceRow{Storefront: cc, App: id, Title: lk.TrackName, Available: true, Price: &price, Currency: &currency, FormattedPrice: &formatted})
					s2aWarnStore(cmd, "the "+cc+" listing snapshot of "+id, s2aRecordLookupSnapshot(ctx, cmd, st, cc, lk, s2aIDFetchedAt(resp, ids, id)))
				}
			}
			meta.Requests = c.Requests()
			if stopped != nil && len(rows) == 0 {
				return s2aStopped(stopped)
			}
			if len(errs) == len(list) {
				return errs[0]
			}
			if available == 0 && len(errs) == 0 && stopped == nil {
				return notFoundErr(fmt.Errorf("none of %s is sold in storefront(s) %s (check the ids, or try other --countries)", strings.Join(ids, ","), strings.Join(list, ",")))
			}
			unavailable := 0
			for _, r := range rows {
				if !r.Available {
					unavailable++
				}
			}
			if unavailable > 0 {
				meta.Notes = append(meta.Notes, fmt.Sprintf("%d app/storefront pairs are not sold (available=false)", unavailable))
			}
			bundleRefs := 0
			for _, r := range refs {
				if r.ID == "" {
					bundleRefs++
				}
			}
			warnFetchFailures(cmd.ErrOrStderr(), len(failures), len(list)+bundleRefs, "lookups")
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
			table := make([][]string, 0, len(rows))
			for _, r := range rows {
				price := "not sold"
				if r.Available && r.FormattedPrice != nil {
					price = *r.FormattedPrice
					if r.Currency != nil && *r.Currency != "" {
						price += " (" + *r.Currency + ")"
					}
				}
				table = append(table, []string{r.Storefront, r.App, s2aOneLine(r.Title, 40), price})
			}
			if err := s2aTable(w, []string{"storefront", "app id", "title", "price"}, table); err != nil {
				return err
			}
			s2aFailuresHuman(w, failures)
			return s2aStopped(stopped)
		},
	}
	cmd.Flags().StringVar(&countries, "countries", "", "Storefronts to check, comma-separated (us,gb,jp) or all; empty uses --country")
	s2bDBFlag(cmd)
	return cmd
}

// s2aResolveAppIDs turns app refs into numeric ids in input order. Numeric
// refs pass through; a bundle id is looked up in each storefront in turn
// until Apple knows it (one request per try). Unresolvable refs become fetch
// failures; a throttle is returned as err and stops the run.
func s2aResolveAppIDs(ctx context.Context, c *appstore.Client, refs []appstore.AppRef, storefronts []string) ([]string, []fetchFailure, []error, error) {
	ids := make([]string, 0, len(refs))
	failures := make([]fetchFailure, 0)
	errs := make([]error, 0)
	seen := map[string]bool{}
	add := func(id string) {
		if id != "" && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	for _, ref := range refs {
		if ref.ID != "" {
			add(ref.ID)
			continue
		}
		resolved := false
		var lastErr error
		for _, cc := range storefronts {
			lk, _, found, err := s2aLookupIn(ctx, c, ref, cc)
			if err != nil {
				if isRateLimited(err) {
					return nil, nil, nil, err
				}
				lastErr = err
				continue
			}
			if found {
				add(lk.AppID())
				resolved = true
				break
			}
		}
		if resolved {
			continue
		}
		msg := fmt.Sprintf("bundle id not found in storefront(s) %s; pass the numeric app id", strings.Join(storefronts, ","))
		if lastErr != nil {
			msg = lastErr.Error()
			errs = append(errs, lastErr)
		}
		failures = append(failures, fetchFailure{ID: ref.BundleID, Error: msg})
	}
	return ids, failures, errs, nil
}
