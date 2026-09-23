// Copyright 2026 sambassio and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/zimmo/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/other/zimmo/internal/store"
	"github.com/mvanhorn/printing-press-library/library/other/zimmo/internal/zimmo"
)

type fetchFailure struct {
	Code  string `json:"ref"` // zimmo code or postcode
	Error string `json:"error"`
}

type enrichView struct {
	Candidates    int            `json:"candidates"`
	Checked       int            `json:"checked"`
	Refreshed     int            `json:"refreshed"`
	Geocoded      int            `json:"geocoded"`
	Gone          int            `json:"gone"`
	StatusChanged int            `json:"status_changed"`
	PriceChanged  int            `json:"price_changed"`
	Remaining     int            `json:"remaining"`
	FetchFailures []fetchFailure `json:"fetch_failures,omitempty"`
	Note          string         `json:"note,omitempty"`
}

var enrichFields = []string{"epc", "flood", "planning", "history", "gps", "status"}

func newNovelEnrichCmd(flags *rootFlags) *cobra.Command {
	var missing, postcode, stale, dbPath string
	var top int
	var rate float64
	cmd := &cobra.Command{
		Use:   "enrich",
		Short: "Refresh stored listings from Zimmo, resumably: status, price history, EPC, flood, planning, GPS",
		Long: `Walks stored listings that no search or detail fetch has refreshed within
--stale (typically listings that dropped out of your searches), re-fetches each one from Zimmo and updates the store: new price and
price history, status changes (a listing that is gone or sold is marked
gone), EPC kWh, flood and planning flags. --missing gps geocodes listings
whose coordinates are missing or not rooftop-precise, from their address.
Runs are resumable: refreshed rows are skipped until they are stale again.
Use this command to fill detail fields for many stored listings without
printing them. To read one listing, use 'show'.`,
		Example: strings.Trim(`
  zimmo-pp-cli enrich --top 200
  zimmo-pp-cli enrich --missing gps,epc --postcode 1030 --rate 1
  zimmo-pp-cli enrich --missing status --stale 1d --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": "--top=3;--missing=status"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "refresh stored listings from Zimmo")
			}
			if err := rejectDataSource(flags, "live"); err != nil {
				return usageErr(err)
			}
			want := map[string]bool{}
			for _, f := range splitCSV(strings.ToLower(missing)) {
				ok := false
				for _, k := range enrichFields {
					if f == k {
						ok = true
					}
				}
				if !ok {
					return usageErr(fmt.Errorf("--missing %q: use %s", f, strings.Join(enrichFields, ",")))
				}
				want[f] = true
			}
			if len(want) == 0 {
				for _, k := range enrichFields {
					want[k] = true
				}
			}
			staleFor, err := cliutil.ParseDurationLoose(stale)
			if err != nil {
				return usageErr(fmt.Errorf("--stale: %w", err))
			}
			if top <= 0 {
				return usageErr(fmt.Errorf("--top must be positive"))
			}
			if cliutil.IsAnyHarness() && top > 3 {
				top = 3
			}
			flags.agentSource = "live"
			ctx, cancel := boundCtx(cmd.Context(), batchFlags(flags))
			defer cancel()
			v := enrichView{FetchFailures: make([]fetchFailure, 0)}
			path, ok := localStoreExists(dbPath)
			if !ok {
				emptyStoreHint(cmd, path)
				return printZimmo(cmd.OutOrStdout(), v, flags)
			}
			db, err := openZimmoStore(ctx, path)
			if err != nil {
				return err
			}
			defer db.Close()
			rows, err := db.QueryZimmoListings(ctx, store.ListingFilter{Postcodes: splitCSV(postcode)})
			if err != nil {
				return err
			}
			cutoff := time.Now().Add(-staleFor).UTC().Format(time.RFC3339)
			type job struct {
				row     store.StoredListing
				refetch bool
				geocode bool
			}
			jobs := make([]job, 0)
			for _, r := range rows {
				// A search result carries the full listing, so a row seen by
				// find/watch recently is as fresh as a detail fetch.
				seen := r.DetailAt
				if r.LastSeen > seen {
					seen = r.LastSeen
				}
				fresh := seen >= cutoff
				j := job{row: r}
				if !fresh {
					if want["status"] || want["history"] {
						j.refetch = true
					}
					if want["epc"] && (r.EPC == "" || r.EPCKWh == nil) {
						j.refetch = true
					}
					if (want["flood"] || want["planning"]) && r.Subpoena == "" && len(r.FloodRisk) == 0 {
						j.refetch = true
					}
				}
				if want["gps"] && (r.Lat == nil || (r.GeoPrecision != "ROOFTOP" && r.GeoPrecision != "GEOCODED" && r.GeoPrecision != "GEOCODE_FAILED")) && r.Street != "" && r.Number != "" {
					j.geocode = true
				}
				if j.refetch || j.geocode {
					jobs = append(jobs, j)
				}
			}
			// Oldest detail first so repeated runs converge.
			sort.SliceStable(jobs, func(i, k int) bool { return jobs[i].row.DetailAt < jobs[k].row.DetailAt })
			v.Candidates = len(jobs)
			if len(jobs) > top {
				v.Remaining = len(jobs) - top
				jobs = jobs[:top]
			}
			zc := zimmoClient(flags)
			if rate > 0 {
				zc.Limiter = cliutil.NewAdaptiveLimiter(rate)
			}
			var gone []string
			for _, j := range jobs {
				if ctx.Err() != nil {
					v.Remaining += len(jobs) - v.Checked
					break
				}
				v.Checked++
				l := j.row.Listing
				if j.refetch {
					fresh, err := fetchListing(ctx, zc, l.Code)
					if err != nil {
						if isNotFound(err) {
							gone = append(gone, l.Code)
							v.Gone++
							continue
						}
						v.FetchFailures = append(v.FetchFailures, fetchFailure{Code: l.Code, Error: err.Error()})
						continue
					}
					if fresh.Status != l.Status {
						v.StatusChanged++
					}
					if fresh.Price != nil && l.Price != nil && *fresh.Price != *l.Price {
						v.PriceChanged++
					}
					if fresh.Lat == nil && l.Lat != nil {
						fresh.Lat, fresh.Lng, fresh.GeoPrecision = l.Lat, l.Lng, l.GeoPrecision
					}
					l = fresh
					v.Refreshed++
				}
				if j.geocode && l.Street != "" {
					g, err := zc.Geocode(ctx, zimmo.FormatAddress(l.Street, l.Number, "", l.PostalCode, l.Locality))
					switch {
					case err == nil:
						lat, lng := g.Latitude, g.Longitude
						l.Lat, l.Lng, l.GeoPrecision = &lat, &lng, "GEOCODED"
						v.Geocoded++
					case isNotFound(err):
						// Remember the miss so the row does not fill every batch.
						l.GeoPrecision = "GEOCODE_FAILED"
					default:
						v.FetchFailures = append(v.FetchFailures, fetchFailure{Code: l.Code, Error: "geocode: " + err.Error()})
					}
				}
				if j.refetch {
					if err := db.UpsertZimmoListings(ctx, []zimmo.Listing{l}, time.Now(), true); err != nil {
						return err
					}
				} else if err := db.UpdateZimmoGeo(ctx, l); err != nil {
					return err
				}
			}
			// Record departures even if the command deadline has passed.
			wctx, wcancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
			defer wcancel()
			if err := db.MarkZimmoGone(wctx, gone, time.Now()); err != nil {
				return err
			}
			if len(v.FetchFailures) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d of %d listings could not be refreshed\n", len(v.FetchFailures), v.Checked)
			}
			switch {
			case v.Candidates == 0:
				v.Note = "nothing to refresh: every stored listing is fresher than --stale"
			case v.Remaining > 0:
				v.Note = fmt.Sprintf("%d listings still to refresh; run again (resumable) or raise --top", v.Remaining)
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printZimmo(cmd.OutOrStdout(), v, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "checked %d: %d refreshed, %d geocoded, %d gone, %d status changes, %d price changes, %d failed\n",
				v.Checked, v.Refreshed, v.Geocoded, v.Gone, v.StatusChanged, v.PriceChanged, len(v.FetchFailures))
			if v.Note != "" {
				fmt.Fprintln(cmd.OutOrStdout(), v.Note)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&missing, "missing", "", "Fields to fill: epc,flood,planning,history,gps,status (default all)")
	cmd.Flags().StringVar(&postcode, "postcode", "", "Only these postcodes (comma-separated)")
	cmd.Flags().StringVar(&stale, "stale", "7d", "Re-fetch listings whose detail is older than this")
	cmd.Flags().IntVar(&top, "top", 100, "Maximum listings to refresh in this run")
	cmd.Flags().Float64Var(&rate, "rate", 0, "Requests per second ceiling (default 2)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local store path (default: the CLI's data.db)")
	return cmd
}
