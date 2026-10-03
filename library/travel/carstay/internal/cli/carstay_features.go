// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
// pp:data-source live
package cli

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/carstay/internal/carstay"
	"github.com/mvanhorn/printing-press-library/library/travel/carstay/internal/cliutil"
	"github.com/spf13/cobra"
)

func carstayNovelRun(ctx context.Context, cmd *cobra.Command, flags *rootFlags, c *carstay.Client, directory, selected []carstay.Spot, o carstayOptions, pref string, view carstayView) error {
	kind := view.Meta["command"].(string)
	if kind == "near" || kind == "coverage" {
		scan := directory
		if len(scan) > o.maxScan {
			scan = scan[:o.maxScan]
		}
		view.Meta["scanned_records"] = len(scan)
		view.Meta["scan_complete"] = len(scan) == len(directory)
		if kind == "near" {
			filtered := []carstay.Spot{}
			for _, s := range scan {
				if carstay.Match(s, "", pref, o.language) {
					filtered = append(filtered, s)
				}
			}
			ranked := carstay.RankNear(filtered, o.lat, o.lon, o.radius)
			rows := []carstay.Spot{}
			for _, s := range ranked {
				if len(rows) < o.limit {
					rows = append(rows, carstay.Summary(s))
				}
			}
			view.Meta["distance_basis"] = "Haversine straight-line kilometers, not road distance or drive time"
			view.Meta["matched_within_scan"] = len(ranked)
			view.Meta["output_truncated"] = len(ranked) > len(rows)
			if len(rows) == 0 {
				view.Meta["note"] = "No coordinate-bearing overnight station within the radius in the examined source; widen radius or --max-scan-records."
			}
			view.Results = rows
			return carstayPrint(cmd, flags, view)
		}
		ja, en, name, both, activities, unknown := 0, 0, 0, 0, 0, 0
		gaps := []carstay.Spot{}
		for _, s := range scan {
			if pref != "" && s.Prefecture != pref {
				continue
			}
			if s.ActivityOnly == nil {
				unknown++
				continue
			}
			if *s.ActivityOnly {
				activities++
				continue
			}
			ja++
			if carstay.EnglishPublished(s) {
				en++
			}
			if s.NameEN != "" {
				name++
			}
			if carstay.EnglishPublished(s) && s.NameEN != "" {
				both++
			}
			if (!carstay.EnglishPublished(s) || s.NameEN == "") && len(gaps) < o.limit {
				gaps = append(gaps, carstay.Summary(s))
			}
		}
		view.Meta["coverage_baseline"] = "Japanese public directory; counts apply only to scanned records and selected prefecture"
		view.Results = map[string]any{"japanese_overnight": ja, "english_approved_overnight": en, "english_name_present_overnight": name, "english_approved_with_name_overnight": both, "missing_english_approval_count": ja - en, "missing_english_name_count": ja - name, "activity_only_excluded": activities, "unknown_activity_status_excluded": unknown, "gap_examples": gaps, "gap_examples_complete": len(gaps) == ja-both}
		return carstayPrint(cmd, flags, view)
	}
	details := []carstay.Spot{}
	for _, s := range selected {
		d, err := c.Detail(ctx, s)
		if err != nil {
			var rate *cliutil.RateLimitError
			if errors.As(err, &rate) || ctx.Err() != nil {
				return classifyAPIErrorOnly(err)
			}
			view.FetchFailures = append(view.FetchFailures, map[string]string{"id": s.ID, "error": err.Error()})
			continue
		}
		details = append(details, carstayBoundText(d, o.textLimit, view.Meta))
	}
	if len(view.FetchFailures) > 0 {
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d of %d detail fetches failed; evidence covers %d successful stations\n", len(view.FetchFailures), len(selected), len(details))
	}
	view.Meta["successful_stations"] = len(details)
	view.Meta["requested_stations"] = len(selected)
	view.Meta["detail_fetch_complete"] = len(view.FetchFailures) == 0
	switch kind {
	case "compare":
		rows := []carstay.Spot{}
		for _, s := range details {
			keep := []carstay.Facility{}
			for _, key := range o.facilities {
				for _, f := range s.Facilities {
					if key == f.Key {
						keep = append(keep, f)
						break
					}
				}
			}
			s = carstay.Summary(s)
			s.Facilities = keep
			rows = append(rows, s)
		}
		view.Results = rows
	case "fit":
		rows := []carstay.Assessment{}
		constraints := carstay.Constraints{Length: o.length, Width: o.width, Height: o.height, Require: o.required}
		for _, s := range details {
			rows = append(rows, carstay.Assess(s, constraints))
		}
		view.Results = rows
		view.Meta["assessment_basis"] = "Reported parking-space dimensions and facility flags; vehicle acceptance is unknown"
	case "audit":
		candidacy := map[string]bool{}
		if o.checkIn != "" {
			cand, err := c.DateCandidates(ctx, "ja", o.checkIn, o.checkOut, o.maxPages)
			if err != nil {
				return classifyAPIErrorOnly(err)
			}
			for _, s := range cand.Spots {
				candidacy[s.ID] = true
			}
			view.Meta["upstream_total_candidates"] = cand.Total
			view.Meta["scanned_pages"] = cand.Pages
			view.Meta["scanned_records"] = cand.Scanned
			view.Meta["scan_complete"] = cand.Scanned >= cand.Total
			view.Meta["check_in_jst"] = o.checkIn
			view.Meta["check_out_jst"] = o.checkOut
		}
		rows := []carstay.EvidenceAudit{}
		for _, s := range details {
			state := "not_checked"
			if o.checkIn != "" {
				state = "not_observed_within_scan_not_unavailable"
				if candidacy[s.ID] {
					state = "provider_date_filtered_candidate"
				}
			}
			rows = append(rows, carstay.Audit(s, time.Now().UTC(), state))
		}
		view.Results = rows
	default:
		return usageErr(fmt.Errorf("unknown Carstay operation %s", kind))
	}
	if err := carstayPrint(cmd, flags, view); err != nil {
		return err
	}
	if len(view.FetchFailures) > 0 {
		return apiErr(fmt.Errorf("incomplete station evidence: %d of %d requested detail fetches failed", len(view.FetchFailures), len(selected)))
	}
	return nil
}
