// Copyright 2026 richardadonnell and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel feature (NOT generated).
// pp:data-source live

package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

type agendaFailure struct {
	Source string `json:"source"`
	Error  string `json:"error"`
}

type agendaView struct {
	Events         []lumaEventView `json:"events"`
	Count          int             `json:"count"`
	SourcesQueried int             `json:"sources_queried"`
	Window         string          `json:"window,omitempty"`
	FetchFailures  []agendaFailure `json:"fetch_failures"`
}

func newNovelAgendaCmd(flags *rootFlags) *cobra.Command {
	var flagCity []string
	var flagPlaceID []string
	var flagCategory []string
	var flagWindow string
	var flagLimit int
	var flagMaxScanPages int

	cmd := &cobra.Command{
		Use:   "agenda",
		Short: "One flat, date-sorted list of upcoming events across multiple cities and categories at once.",
		Long: "Fan out to several cities and/or categories, merge and dedupe by event id, and return one\n" +
			"flat list sorted by start time. The public API only lists one place OR one category per\n" +
			"call, so this is the multi-source view it cannot return.\n\n" +
			"Pass any mix of --city, --place-id, and --category (each repeatable).",
		Example:     "  luma-pp-cli agenda --city sf --city nyc --category cat-ai --window 7d --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would fetch events across the requested cities/categories")
				return nil
			}
			if len(flagCity) == 0 && len(flagPlaceID) == 0 && len(flagCategory) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("agenda needs at least one of --city, --place-id, or --category"))
			}
			window, err := parseWindow(flagWindow)
			if err != nil {
				return usageErr(fmt.Errorf("invalid --window %q: %w", flagWindow, err))
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			type src struct {
				label  string
				params map[string]string
			}
			var sources []src
			for _, city := range flagCity {
				// NOTE: Luma's get-paginated-events treats the place `slug`
				// loosely — for some city slugs it folds in nearby/featured
				// events from other metros, so results can include events
				// outside the requested city. We deliberately do NOT
				// post-filter on city_state here: a naive slug->city string
				// match would silently drop legitimate metro-area events
				// (e.g. an SF-slug event tagged "Oakland, CA") and events with
				// empty geo data. Each output row carries its own `city` field
				// so the caller can filter precisely if needed. Tightening this
				// safely needs a slug->region resolver (a generator/spec-level
				// concern), not a client-side heuristic.
				sources = append(sources, src{"city:" + city, map[string]string{"slug": city}})
			}
			for _, pid := range flagPlaceID {
				sources = append(sources, src{"place:" + pid, map[string]string{"discover_place_api_id": pid}})
			}
			for _, cat := range flagCategory {
				sources = append(sources, src{"category:" + cat, map[string]string{"category_api_id": cat}})
			}

			maxScan := scanPagesForEnv(flagMaxScanPages)
			// Page size is the upstream fetch batch, decoupled from --limit (the
			// output cap applied after merge). Keeps --limit 0 from sending
			// pagination_limit=0 to the API.
			const agendaPageSize = 50
			var all []lumaEntry
			failures := make([]agendaFailure, 0)
			for _, s := range sources {
				entries, ferr := fetchEventEntries(ctx, c, s.params, agendaPageSize, maxScan)
				// Keep entries fetched before an error so a late-page failure
				// still contributes its earlier pages.
				all = append(all, entries...)
				if ferr != nil {
					failures = append(failures, agendaFailure{Source: s.label, Error: ferr.Error()})
				}
			}

			all = dedupeByID(all)
			now := time.Now()
			kept := make([]lumaEntry, 0, len(all))
			for _, e := range all {
				t, ok := e.startTime()
				if !ok {
					// No parseable start_at: a window query is time-bounded, so an
					// undateable event cannot satisfy it — drop it. With no window,
					// keep it (it sorts last) so the unfiltered agenda is complete.
					if window > 0 {
						continue
					}
					kept = append(kept, e)
					continue
				}
				if !withinWindow(t, now, window) {
					continue
				}
				kept = append(kept, e)
			}
			sortByStart(kept)
			if flagLimit > 0 && len(kept) > flagLimit {
				kept = kept[:flagLimit]
			}

			views := make([]lumaEventView, 0, len(kept))
			for _, e := range kept {
				views = append(views, e.view())
			}

			if len(failures) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d of %d sources failed; results cover the remaining %d\n",
					len(failures), len(sources), len(sources)-len(failures))
			}

			view := agendaView{
				Events:         views,
				Count:          len(views),
				SourcesQueried: len(sources),
				FetchFailures:  failures,
			}
			if window > 0 {
				view.Window = flagWindow
			}
			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().StringSliceVar(&flagCity, "city", nil, "City slug to include (repeatable), e.g. sf, nyc, miami")
	cmd.Flags().StringSliceVar(&flagPlaceID, "place-id", nil, "Place api_id to include (repeatable), e.g. discplace-...")
	cmd.Flags().StringSliceVar(&flagCategory, "category", nil, "Category api_id to include (repeatable), e.g. cat-ai")
	cmd.Flags().StringVar(&flagWindow, "window", "", "Only events within this window from now (e.g. 7d, 24h); empty = all upcoming")
	cmd.Flags().IntVar(&flagLimit, "limit", 50, "Max events to return after merge")
	cmd.Flags().IntVar(&flagMaxScanPages, "max-scan-pages", 3, "Max pages to fetch per source before stopping")
	return cmd
}
