// Copyright 2026 Jet Sng and contributors. Licensed under Apache-2.0.
// pp:data-source auto
package cli

import (
	"encoding/json"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/travel/wheelog/internal/client"
	"github.com/mvanhorn/printing-press-library/library/travel/wheelog/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/wheelog/internal/store"
	"github.com/mvanhorn/printing-press-library/library/travel/wheelog/internal/wheelog"
	"github.com/spf13/cobra"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

type wheelogSearchView struct {
	Results  []wheelogSpotView `json:"results"`
	Query    any               `json:"source_query"`
	Coverage map[string]any    `json:"coverage"`
	Failures []wheelogFailure  `json:"fetch_failures"`
	Note     string            `json:"note"`
}

func newSpotsSearchCmd(flags *rootFlags) *cobra.Command {
	var query, categories, from, to, timezone string
	var limit, maxPages int
	var details bool
	var options wheelogReadOptions
	cmd := &cobra.Command{Use: "search [keyword]", Short: "Search bounded public spot records by keyword, category and record dates.",
		Long:        "Discover candidate places by source keyword, category and record date window. --require-question expands bounded real details rather than treating search filters as positive evidence. For chosen IDs use spots compare. Local mode searches only saved places. Dates select source records, not trip availability or individual report dates.",
		Example:     "  wheelog-pp-cli spots search 成田空港 --category toilet --limit 3 --agent\n  wheelog-pp-cli spots search 成田空港 --require-question 102 --limit 3 --agent\n  wheelog-pp-cli spots search --query 成田空港 --from 2026-10-02 --to 2026-10-02 --timezone Asia/Tokyo --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "auto", "pp:happy-args": "keyword=成田空港;--category=toilet;--require-question=102;--limit=3;--data-source=live"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "search bounded public WheeLog spot records")
			}
			if len(args) > 1 || (len(args) > 0 && cmd.Flags().Changed("query")) {
				return usageErr(fmt.Errorf("provide one keyword or --query, not both"))
			}
			if len(args) == 1 {
				query = args[0]
			}
			if len([]rune(query)) > 256 {
				return usageErr(fmt.Errorf("--query/keyword is limited to 256 characters"))
			}
			if limit < 1 || limit > 50 || maxPages < 1 || maxPages > 5 {
				return usageErr(fmt.Errorf("--limit must be 1..50 and --max-scan-pages must be 1..5"))
			}
			if (details || len(options.Questions) > 0) && limit > wheelog.MaxDetailReads {
				return usageErr(fmt.Errorf("detail expansion is limited to 5 spots; set --limit 5 or smaller"))
			}
			maxAge, err := validateWheelogOptions(options)
			if err != nil {
				return err
			}
			cats, err := wheelogCategories(categories)
			if err != nil {
				return err
			}
			start, end, err := wheelogDateWindow(from, to, timezone)
			if err != nil {
				return err
			}
			mode, err := wheelogMode(flags)
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			view := wheelogSearchView{Results: make([]wheelogSpotView, 0), Failures: make([]wheelogFailure, 0), Coverage: map[string]any{"scope": "bounded_source_keyword_search", "max_scan_pages": maxPages, "scanned_records": 0, "listed": 0, "checked": 0, "unavailable": 0, "unexpanded": 0, "complete_coverage": false}, Note: wheelogEvidenceNote}
			fields := url.Values{"word": {query}, "type": {"spot"}, "isDetail": {"1"}}
			if start != nil {
				fields.Set("startDatetime", *start)
			}
			if end != nil {
				fields.Set("endDatetime", *end)
			}
			for _, category := range cats {
				fields.Add("categoryList[]", category)
			}
			view.Query = map[string]any{"keyword": query, "categories": cats, "record_from_utc": start, "record_to_utc": end, "input_timezone": timezone}
			var cached []store.WheelogObservation
			var c *client.Client
			if mode == "local" {
				cached, err = savedWheelog(ctx, options.DB)
				if err != nil {
					return err
				}
			}
			local := mode == "local"
			cards := make([]wheelog.Spot, 0)
			if !local {
				c, err = newWheelogClient(flags)
				if err != nil {
					return err
				}
				seen := map[int64]bool{}
				scanned, pages := 0, 0
				if cliutil.IsDogfoodEnv() && maxPages > 1 {
					maxPages = 1
					view.Coverage["max_scan_pages"] = maxPages
				}
				for page := 1; page <= maxPages && len(cards) < limit; page++ {
					fields.Set("pagenum", strconv.Itoa(page))
					data, _, fetchErr := c.PostQueryFormWithParams(ctx, "/timeline/custom/user/getWebTimelineList", nil, fields)
					if fetchErr != nil {
						if isWheelogThrottle(fetchErr) {
							return wheelogError(fetchErr)
						}
						if mode == "auto" && page == 1 {
							cached, err = savedWheelog(ctx, options.DB)
							if err != nil {
								return configErr(fmt.Errorf("live WheeLog search failed (%v); saved fallback unavailable: %w", fetchErr, err))
							}
							if len(cached) == 0 {
								return wheelogError(fetchErr)
							}
							local = true
							view.Coverage["fallback_reason"] = fetchErr.Error()
							break
						}
						return wheelogError(fetchErr)
					}
					var envelope wheelog.Envelope
					if err := json.Unmarshal(data, &envelope); err != nil || envelope.Content == nil || envelope.Content.Request == nil {
						return apiErr(fmt.Errorf("WheeLog search response contract changed"))
					}
					echo := envelope.Content.Request
					if echo.Word != query || echo.Type != "spot" || (start != nil && (echo.From == nil || *echo.From != *start)) || (end != nil && (echo.To == nil || *echo.To != *end)) {
						return apiErr(fmt.Errorf("WheeLog did not echo the requested keyword/type/date window"))
					}
					sourceCats := append([]string(nil), echo.Categories...)
					sort.Strings(sourceCats)
					if strings.Join(sourceCats, ",") != strings.Join(cats, ",") {
						return apiErr(fmt.Errorf("WheeLog did not echo requested categories"))
					}
					pages++
					if len(envelope.Content.Timeline) == 0 {
						view.Coverage["empty_page_observed"] = true
						break
					}
					for _, item := range envelope.Content.Timeline {
						scanned++
						if seen[item.Spot.ID] {
							continue
						}
						seen[item.Spot.ID] = true
						spot, err := wheelog.Normalize(item.Spot, time.Now().UTC())
						if err != nil {
							return apiErr(err)
						}
						cards = append(cards, spot)
						if len(cards) == limit {
							break
						}
					}
					view.Coverage["source_request_echo"] = echo
				}
				view.Coverage["scanned_records"] = scanned
				view.Coverage["pages_requested"] = pages
			}
			if local {
				view.Query.(map[string]any)["execution"] = "saved_record_metadata_filter"
				wheelogLocalHint(cmd, cached, maxAge)
				view.Coverage["scope"] = "saved_shortlist"
				view.Coverage["scanned_records"] = len(cached)
				for _, item := range cached {
					spot := item.Latest
					if query != "" && !strings.Contains(strings.ToLower(spot.Name+" "+spot.Address), strings.ToLower(query)) {
						continue
					}
					if len(cats) > 0 && !containsWheelogCategory(cats, spot.Category) {
						continue
					}
					if !wheelogRecordInWindow(spot, start, end) {
						continue
					}
					spot.Source = "local"
					cards = append(cards, spot)
					if len(cards) == limit {
						break
					}
				}
			}
			checked, failed, liveRows, localRows := 0, 0, 0, 0
			for _, card := range cards {
				spot := card
				if !local && (details || len(options.Questions) > 0) {
					fresh, err := resolveWheelogSpot(ctx, flags, options, card.ID, c, cached)
					if err != nil {
						if isWheelogThrottle(err) {
							return wheelogError(err)
						}
						failed++
						spot.DetailStatus = "unavailable"
						view.Failures = append(view.Failures, wheelogFailure{card.ID, err.Error()})
					} else {
						spot = fresh
						checked++
					}
				} else if spot.DetailStatus == "checked" {
					checked++
				}
				if spot.Source == "local" {
					localRows++
				} else if spot.DetailStatus != "unavailable" {
					liveRows++
				}
				view.Results = append(view.Results, viewWheelog(spot, options.Questions, maxAge))
			}
			view.Coverage["live"] = liveRows
			view.Coverage["local"] = localRows
			view.Coverage["listed"] = len(cards)
			view.Coverage["checked"] = checked
			view.Coverage["unavailable"] = failed
			view.Coverage["unexpanded"] = len(cards) - checked - failed
			view.Coverage["output_limit"] = limit
			if len(cards) == 0 {
				view.Note += " No records matched within this bounded scope; widen the keyword or --max-scan-pages for live discovery."
			}
			if failed > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d of %d detail fetches failed; evidence is checked for %d records\n", failed, len(cards), checked)
			}
			source := "live"
			if local {
				source = "local"
			} else if localRows > 0 {
				source = "auto"
			}
			return emitWheelog(cmd, flags, view, source)
		}}
	cmd.Flags().StringVar(&query, "query", "", "Source keyword; use instead of the positional keyword.")
	cmd.Flags().StringVar(&categories, "category", "", "Exact source categories, comma-separated; run categories for values.")
	cmd.Flags().StringVar(&from, "from", "", "Inclusive source record date in YYYY-MM-DD using --timezone.")
	cmd.Flags().StringVar(&to, "to", "", "Inclusive source record date in YYYY-MM-DD using --timezone.")
	cmd.Flags().StringVar(&timezone, "timezone", "Asia/Tokyo", "IANA timezone for record-date inputs; backend requests use UTC.")
	cmd.Flags().IntVar(&limit, "limit", 5, "Maximum records returned, 1..50; detail expansion permits at most 5.")
	cmd.Flags().IntVar(&maxPages, "max-scan-pages", 1, "Maximum source pages scanned, 1..5; separate from the output limit.")
	cmd.Flags().BoolVar(&details, "details", false, "Fetch actual aggregate question details for every returned candidate.")
	addWheelogReadFlags(cmd, &options)
	return cmd
}

func containsWheelogCategory(categories []string, value string) bool {
	for _, category := range categories {
		if category == value {
			return true
		}
	}
	return false
}
func wheelogRecordInWindow(spot wheelog.Spot, start, end *string) bool {
	if start == nil && end == nil {
		return true
	}
	if spot.Created == nil {
		return false
	}
	record, err := time.Parse(time.RFC3339Nano, *spot.Created)
	if err != nil {
		return false
	}
	if start != nil {
		stamp, _ := time.ParseInLocation("2006-01-02 15:04:05", *start, time.UTC)
		if record.Before(stamp) {
			return false
		}
	}
	if end != nil {
		stamp, _ := time.ParseInLocation("2006-01-02 15:04:05", *end, time.UTC)
		// Wire dates use whole seconds; the selected calendar day includes
		// fractional timestamps in its final second.
		if !record.Before(stamp.Add(time.Second)) {
			return false
		}
	}
	return true
}
