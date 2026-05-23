// Copyright 2026 muhammad-khan. Licensed under Apache-2.0. See LICENSE.
// Hand-authored ZAZU-integration novel-feature commands.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// commonZazuFlags carries the flags shared by every ZAZU-aware command.
type commonZazuFlags struct {
	zazuDB       string
	projectTypes string
	minValue     int
	states       string
	mode         string
}

// ---- zazu-diff ----

func newZazuDiffCmd(flags *rootFlags) *cobra.Command {
	z := &commonZazuFlags{}
	cmd := &cobra.Command{
		Use:   "zazu-diff",
		Short: "Surface BuildAlert leads that aren't yet in your ZAZU bd-mirror.sqlite.",
		Long: strings.TrimSpace(`
Left-anti-join BuildAlert's matched leads against your ZAZU bd-mirror.sqlite
on (councilIdentifier, reference) -> (sheet, reference).

Use --mode overlap to flip the predicate and list leads present in BOTH systems.
`),
		Example: strings.Trim(`
  buildalert-pp-cli zazu-diff --zazu-db ~/Downloads/Zazu/bd-mirror.sqlite
  buildalert-pp-cli zazu-diff --zazu-db bd-mirror.sqlite --project-types Loft_Conversion --agent
  buildalert-pp-cli zazu-diff --zazu-db bd-mirror.sqlite --mode overlap --json
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":          "true",
			"pp:happy-args":          "--zazu-db=C:/Users/bazil/Downloads/Zazu/bd-mirror.sqlite",
			"pp:no-error-path-probe": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			return runZazuDiff(cmd, flags, z)
		},
	}
	addCommonZazuFlags(cmd, z)
	cmd.Flags().StringVar(&z.mode, "mode", "diff", "diff (default) or overlap")
	return cmd
}

func runZazuDiff(cmd *cobra.Command, flags *rootFlags, z *commonZazuFlags) error {
	dbs, err := openZazuDBs(z.zazuDB)
	if err != nil {
		return configErr(err)
	}
	defer closeAll(dbs)
	c, err := flags.newClient()
	if err != nil {
		return err
	}
	leads, _, err := fetchAllLeads(cmd.Context(), c, z.projectTypes, z.states, z.minValue)
	if err != nil {
		return classifyAPIError(err, flags)
	}
	councils := leadsCouncils(leads)
	keys, err := zazuApplicationKeys(cmd.Context(), dbs, councils)
	if err != nil {
		return apiErr(err)
	}

	type row struct {
		Council             string  `json:"council"`
		Reference           string  `json:"reference"`
		InZazu              bool    `json:"in_zazu"`
		Address             string  `json:"address"`
		PostCode            string  `json:"post_code,omitempty"`
		EstimationValueBand string  `json:"estimation_value_band,omitempty"`
		DistanceAway        float64 `json:"distance_away_miles"`
		Status              string  `json:"status"`
		URL                 string  `json:"url,omitempty"`
		FullDescription     string  `json:"full_description,omitempty"`
	}

	out := make([]row, 0, len(leads))
	wantOverlap := strings.EqualFold(z.mode, "overlap")
	for _, ld := range leads {
		app := ld.Application
		_, present := keys[zazuKey(app.CouncilIdentifier, app.Reference)]
		if wantOverlap && !present {
			continue
		}
		if !wantOverlap && present {
			continue
		}
		out = append(out, row{
			Council:             app.CouncilIdentifier,
			Reference:           app.Reference,
			InZazu:              present,
			Address:             app.Address,
			PostCode:            app.PostCode,
			EstimationValueBand: app.EstimationValueBand,
			DistanceAway:        app.DistanceAway,
			Status:              app.Status,
			URL:                 app.URL,
			FullDescription:     truncate(app.FullDescription, 240),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Council != out[j].Council {
			return out[i].Council < out[j].Council
		}
		return out[i].Reference < out[j].Reference
	})
	return emitJSONOrTable(cmd, flags, out, fmt.Sprintf("%d leads (mode=%s, %d in BuildAlert, %d in ZAZU)", len(out), z.mode, len(leads), len(keys)))
}

// ---- pending-letters ----

func newPendingLettersCmd(flags *rootFlags) *cobra.Command {
	z := &commonZazuFlags{}
	cmd := &cobra.Command{
		Use:   "pending-letters",
		Short: "List BuildAlert leads eligible for outreach AND untouched by ZAZU.",
		Long: strings.TrimSpace(`
Returns BuildAlert leads where canSendLetter=true AND letterBeenSent=false
AND the reference is NOT in ZAZU's letters_sent log.

This is the actionable worklist when running BuildAlert alongside ZAZU's
Telegram-manual-send pipeline.
`),
		Example: strings.Trim(`
  buildalert-pp-cli pending-letters --zazu-db ~/Downloads/Zazu/bd-mirror.sqlite
  buildalert-pp-cli pending-letters --zazu-db bd-mirror.sqlite --project-types Loft_Conversion --agent
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":          "true",
			"pp:happy-args":          "--zazu-db=C:/Users/bazil/Downloads/Zazu/bd-mirror.sqlite",
			"pp:no-error-path-probe": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			return runPendingLetters(cmd, flags, z)
		},
	}
	addCommonZazuFlags(cmd, z)
	return cmd
}

func runPendingLetters(cmd *cobra.Command, flags *rootFlags, z *commonZazuFlags) error {
	dbs, err := openZazuDBs(z.zazuDB)
	if err != nil {
		return configErr(err)
	}
	defer closeAll(dbs)
	c, err := flags.newClient()
	if err != nil {
		return err
	}
	leads, _, err := fetchAllLeads(cmd.Context(), c, z.projectTypes, z.states, z.minValue)
	if err != nil {
		return classifyAPIError(err, flags)
	}
	sentRefs, err := zazuLetterReferences(cmd.Context(), dbs)
	if err != nil {
		return apiErr(err)
	}

	type row struct {
		Council             string  `json:"council"`
		Reference           string  `json:"reference"`
		Address             string  `json:"address"`
		EstimationValueBand string  `json:"estimation_value_band,omitempty"`
		DistanceAway        float64 `json:"distance_away_miles"`
		URL                 string  `json:"url,omitempty"`
	}

	out := []row{}
	for _, ld := range leads {
		app := ld.Application
		if !app.CanSendLetter || app.LetterBeenSent {
			continue
		}
		if _, alreadySent := sentRefs[app.Reference]; alreadySent {
			continue
		}
		out = append(out, row{
			Council:             app.CouncilIdentifier,
			Reference:           app.Reference,
			Address:             app.Address,
			EstimationValueBand: app.EstimationValueBand,
			DistanceAway:        app.DistanceAway,
			URL:                 app.URL,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].DistanceAway < out[j].DistanceAway })
	return emitJSONOrTable(cmd, flags, out, fmt.Sprintf("%d pending (%d total leads, %d sent in ZAZU)", len(out), len(leads), len(sentRefs)))
}

// ---- letter-conflict ----

func newLetterConflictCmd(flags *rootFlags) *cobra.Command {
	z := &commonZazuFlags{}
	cmd := &cobra.Command{
		Use:   "letter-conflict",
		Short: "Detect homeowners who've been mailed by BOTH BuildAlert and ZAZU.",
		Long: strings.TrimSpace(`
Inner-join BuildAlert leads where letterBeenSent=true with ZAZU's letters_sent
log on reference. Each hit is a £2 BuildAlert charge that potentially
duplicates a ZAZU Telegram send to the same homeowner.
`),
		Example: strings.Trim(`
  buildalert-pp-cli letter-conflict --zazu-db ~/Downloads/Zazu/bd-mirror.sqlite
  buildalert-pp-cli letter-conflict --zazu-db bd-mirror.sqlite --json
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":          "true",
			"pp:happy-args":          "--zazu-db=C:/Users/bazil/Downloads/Zazu/bd-mirror.sqlite",
			"pp:no-error-path-probe": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			return runLetterConflict(cmd, flags, z)
		},
	}
	addCommonZazuFlags(cmd, z)
	return cmd
}

func runLetterConflict(cmd *cobra.Command, flags *rootFlags, z *commonZazuFlags) error {
	dbs, err := openZazuDBs(z.zazuDB)
	if err != nil {
		return configErr(err)
	}
	defer closeAll(dbs)
	c, err := flags.newClient()
	if err != nil {
		return err
	}
	leads, _, err := fetchAllLeads(cmd.Context(), c, z.projectTypes, z.states, z.minValue)
	if err != nil {
		return classifyAPIError(err, flags)
	}
	sentRefs, err := zazuLetterReferences(cmd.Context(), dbs)
	if err != nil {
		return apiErr(err)
	}

	type row struct {
		Council   string `json:"council"`
		Reference string `json:"reference"`
		Address   string `json:"address"`
		URL       string `json:"url,omitempty"`
	}

	out := []row{}
	for _, ld := range leads {
		app := ld.Application
		if !app.LetterBeenSent {
			continue
		}
		if _, dupe := sentRefs[app.Reference]; !dupe {
			continue
		}
		out = append(out, row{
			Council:   app.CouncilIdentifier,
			Reference: app.Reference,
			Address:   app.Address,
			URL:       app.URL,
		})
	}
	return emitJSONOrTable(cmd, flags, out, fmt.Sprintf("%d duplicate-mailed homeowners detected", len(out)))
}

// ---- coverage ----

func newCoverageCmd(flags *rootFlags) *cobra.Command {
	z := &commonZazuFlags{}
	cmd := &cobra.Command{
		Use:   "coverage",
		Short: "Per-council volume delta between BuildAlert and ZAZU.",
		Long: strings.TrimSpace(`
Aggregate BuildAlert's matched leads by councilIdentifier and ZAZU's
applications by sheet; emit a per-council row with both counts and the delta.

Use this when deciding which UK councils to add to ZAZU's scraper coverage:
councils where BuildAlert sees many but ZAZU sees zero are the highest-ROI
new scrapers to build.
`),
		Example: strings.Trim(`
  buildalert-pp-cli coverage --zazu-db ~/Downloads/Zazu/bd-mirror.sqlite
  buildalert-pp-cli coverage --zazu-db bd-mirror.sqlite --json
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":          "true",
			"pp:happy-args":          "--zazu-db=C:/Users/bazil/Downloads/Zazu/bd-mirror.sqlite",
			"pp:no-error-path-probe": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			return runCoverage(cmd, flags, z)
		},
	}
	addCommonZazuFlags(cmd, z)
	return cmd
}

func runCoverage(cmd *cobra.Command, flags *rootFlags, z *commonZazuFlags) error {
	dbs, err := openZazuDBs(z.zazuDB)
	if err != nil {
		return configErr(err)
	}
	defer closeAll(dbs)
	c, err := flags.newClient()
	if err != nil {
		return err
	}
	leads, _, err := fetchAllLeads(cmd.Context(), c, z.projectTypes, z.states, z.minValue)
	if err != nil {
		return classifyAPIError(err, flags)
	}
	councils := leadsCouncils(leads)
	zazuCounts, err := zazuApplicationCountsByCouncil(cmd.Context(), dbs, councils)
	if err != nil {
		return apiErr(err)
	}

	buildAlertCounts := map[string]int{}
	for _, ld := range leads {
		council := strings.ToLower(strings.TrimSpace(ld.Application.CouncilIdentifier))
		if council != "" {
			buildAlertCounts[council]++
		}
	}

	type row struct {
		Council           string `json:"council"`
		BuildAlertCount   int    `json:"buildalert_count"`
		ZazuCount         int    `json:"zazu_count"`
		Delta             int    `json:"delta"`
		ZazuMissing       bool   `json:"zazu_missing"`
		BuildAlertMissing bool   `json:"buildalert_missing"`
	}

	seen := map[string]struct{}{}
	for k := range buildAlertCounts {
		seen[k] = struct{}{}
	}
	for k := range zazuCounts {
		seen[strings.ToLower(strings.TrimSpace(k))] = struct{}{}
	}

	out := make([]row, 0, len(seen))
	for council := range seen {
		ba := buildAlertCounts[council]
		zz := 0
		for zk, zv := range zazuCounts {
			if strings.EqualFold(strings.TrimSpace(zk), council) {
				zz += zv
			}
		}
		out = append(out, row{
			Council:           council,
			BuildAlertCount:   ba,
			ZazuCount:         zz,
			Delta:             ba - zz,
			ZazuMissing:       zz == 0 && ba > 0,
			BuildAlertMissing: ba == 0 && zz > 0,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Delta > out[j].Delta })
	return emitJSONOrTable(cmd, flags, out, fmt.Sprintf("%d councils (BuildAlert covers %d, ZAZU covers %d)", len(out), len(buildAlertCounts), len(zazuCounts)))
}

// ---- roi-per-lead ----

func newRoiPerLeadCmd(flags *rootFlags) *cobra.Command {
	z := &commonZazuFlags{}
	var dateFrom, dateTo int64
	cmd := &cobra.Command{
		Use:   "roi-per-lead",
		Short: "Join transactions × tracking × applications for per-lead ROI.",
		Long: strings.TrimSpace(`
Pulls /dapi/transactions and /dapi/tracking inside a date window and joins them
against the lead list to produce per-lead rows: cost, replied flag, work-won
flag, total return.

Defaults to the last 90 days when --date-from/--date-to are omitted.

Optional: pass --zazu-db to also report whether ZAZU's letters_sent log has the
same reference.
`),
		Example: strings.Trim(`
  buildalert-pp-cli roi-per-lead --json
  buildalert-pp-cli roi-per-lead --zazu-db bd-mirror.sqlite --agent
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":          "true",
			"pp:no-error-path-probe": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if dateFrom == 0 {
				dateFrom = time.Now().AddDate(0, 0, -90).Unix()
			}
			if dateTo == 0 {
				dateTo = time.Now().Unix()
			}
			return runRoiPerLead(cmd, flags, z, dateFrom, dateTo)
		},
	}
	addCommonZazuFlags(cmd, z)
	cmd.Flags().Int64Var(&dateFrom, "date-from", 0, "Start of window, unix seconds (default: 90 days ago)")
	cmd.Flags().Int64Var(&dateTo, "date-to", 0, "End of window, unix seconds (default: now)")
	return cmd
}

func runRoiPerLead(cmd *cobra.Command, flags *rootFlags, z *commonZazuFlags, from, to int64) error {
	c, err := flags.newClient()
	if err != nil {
		return err
	}
	ctx := cmd.Context()

	type trackingItem struct {
		Reference    string  `json:"reference"`
		LetterSentAt int64   `json:"letterSentAt"`
		Replied      bool    `json:"replied"`
		WorkWon      bool    `json:"workWon"`
		Return       float64 `json:"return"`
	}
	type trackingPage struct {
		TotalItems   int               `json:"totalItems"`
		ItemsPerPage int               `json:"itemsPerPage"`
		PageCount    int               `json:"pageCount"`
		CurrentPage  int               `json:"currentPage"`
		Items        []json.RawMessage `json:"items"`
	}
	trkByRef := map[string]trackingItem{}
	for page := 1; page <= 200; page++ {
		trkRaw, err := c.Get(ctx, "/dapi/tracking", map[string]string{
			"dateFrom":     fmt.Sprintf("%d", from),
			"dateTo":       fmt.Sprintf("%d", to),
			"page":         fmt.Sprintf("%d", page),
			"itemsPerPage": "200",
		})
		if err != nil {
			return classifyAPIError(err, flags)
		}
		var trkPage trackingPage
		if err := json.Unmarshal(trkRaw, &trkPage); err != nil {
			break
		}
		for _, it := range trkPage.Items {
			var ti trackingItem
			if err := json.Unmarshal(it, &ti); err != nil {
				continue
			}
			var ref struct {
				Reference   string `json:"reference"`
				Application struct {
					Reference string `json:"reference"`
				} `json:"application"`
			}
			_ = json.Unmarshal(it, &ref)
			key := ti.Reference
			if key == "" {
				key = ref.Reference
			}
			if key == "" {
				key = ref.Application.Reference
			}
			if key != "" {
				trkByRef[key] = ti
			}
		}
		if len(trkPage.Items) == 0 || page >= trkPage.PageCount {
			break
		}
	}

	type txRow struct {
		Reference string  `json:"reference"`
		Amount    float64 `json:"amount"`
		Date      int64   `json:"date"`
	}
	type txPage struct {
		TotalItems   int               `json:"totalItems"`
		ItemsPerPage int               `json:"itemsPerPage"`
		PageCount    int               `json:"pageCount"`
		CurrentPage  int               `json:"currentPage"`
		Data         []json.RawMessage `json:"data"`
	}
	costByRef := map[string]float64{}
	for page := 1; page <= 200; page++ {
		txRaw, err := c.Get(ctx, "/dapi/transactions", map[string]string{
			"dateFrom":     fmt.Sprintf("%d", from),
			"dateTo":       fmt.Sprintf("%d", to),
			"page":         fmt.Sprintf("%d", page),
			"itemsPerPage": "200",
		})
		if err != nil {
			return classifyAPIError(err, flags)
		}
		var txp txPage
		if err := json.Unmarshal(txRaw, &txp); err != nil {
			break
		}
		for _, it := range txp.Data {
			var tr txRow
			if err := json.Unmarshal(it, &tr); err != nil {
				continue
			}
			if tr.Reference == "" {
				var alt struct {
					LeadReference string `json:"leadReference"`
					Application   struct {
						Reference string `json:"reference"`
					} `json:"application"`
				}
				_ = json.Unmarshal(it, &alt)
				tr.Reference = alt.LeadReference
				if tr.Reference == "" {
					tr.Reference = alt.Application.Reference
				}
			}
			if tr.Reference != "" {
				costByRef[tr.Reference] += tr.Amount
			}
		}
		if len(txp.Data) == 0 || page >= txp.PageCount {
			break
		}
	}

	leads, _, err := fetchAllLeads(ctx, c, z.projectTypes, z.states, z.minValue)
	if err != nil {
		return classifyAPIError(err, flags)
	}

	var zazuSent map[string]struct{}
	if z.zazuDB != "" {
		dbs, err := openZazuDBs(z.zazuDB)
		if err == nil {
			defer closeAll(dbs)
			zazuSent, _ = zazuLetterReferences(ctx, dbs)
		}
	}

	type row struct {
		Council    string  `json:"council"`
		Reference  string  `json:"reference"`
		Address    string  `json:"address"`
		Cost       float64 `json:"cost"`
		Replied    bool    `json:"replied"`
		WorkWon    bool    `json:"work_won"`
		ReturnGBP  float64 `json:"return_gbp"`
		ROI        float64 `json:"roi"`
		InZazuSent bool    `json:"in_zazu_letters_sent,omitempty"`
	}

	out := []row{}
	for _, ld := range leads {
		app := ld.Application
		cost := costByRef[app.Reference]
		trk := trkByRef[app.Reference]
		if cost == 0 && trk.Reference == "" {
			continue
		}
		roi := 0.0
		if cost > 0 {
			roi = trk.Return / cost
		}
		r := row{
			Council:   app.CouncilIdentifier,
			Reference: app.Reference,
			Address:   app.Address,
			Cost:      cost,
			Replied:   trk.Replied,
			WorkWon:   trk.WorkWon,
			ReturnGBP: trk.Return,
			ROI:       roi,
		}
		if zazuSent != nil {
			_, r.InZazuSent = zazuSent[app.Reference]
		}
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ROI > out[j].ROI })
	return emitJSONOrTable(cmd, flags, out, fmt.Sprintf("%d leads with cost/tracking data (%d-%d window)", len(out), from, to))
}

// ---- nearby ----

func newNearbyCmd(flags *rootFlags) *cobra.Command {
	z := &commonZazuFlags{}
	var postcode string
	var radius float64
	cmd := &cobra.Command{
		Use:   "nearby",
		Short: "Re-filter BuildAlert leads against an arbitrary UK postcode, offline.",
		Long: strings.TrimSpace(`
Resolves --postcode to lat/lng via postcodes.io (free, no auth), then haversine-
distances each BuildAlert lead's lat/lng against it. Returns leads within
--radius miles.

Unlike BuildAlert's web filter (which only honors your account postcode), this
lets you score leads against a satellite-office postcode or evaluate radius
expansion experiments without changing your account.
`),
		Example: strings.Trim(`
  buildalert-pp-cli nearby --postcode "HA2 9RN" --radius 10
  buildalert-pp-cli nearby --postcode SW1A1AA --radius 5 --json
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":          "true",
			"pp:happy-args":          "--postcode=HA2 9RN;--radius=5",
			"pp:no-error-path-probe": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if postcode == "" {
				return usageErr(fmt.Errorf("--postcode is required (e.g. HA1, SW1A1AA)"))
			}
			if radius <= 0 {
				radius = 5
			}
			return runNearby(cmd, flags, z, postcode, radius)
		},
	}
	addCommonZazuFlags(cmd, z)
	cmd.Flags().StringVar(&postcode, "postcode", "", "UK postcode to center the search on (required)")
	cmd.Flags().Float64Var(&radius, "radius", 5, "Radius in miles (default 5)")
	return cmd
}

func runNearby(cmd *cobra.Command, flags *rootFlags, z *commonZazuFlags, postcode string, radius float64) error {
	lat, lon, err := lookupPostcode(cmd.Context(), postcode)
	if err != nil {
		return usageErr(fmt.Errorf("resolving postcode %q via postcodes.io: %w", postcode, err))
	}
	c, err := flags.newClient()
	if err != nil {
		return err
	}
	leads, _, err := fetchAllLeads(cmd.Context(), c, z.projectTypes, z.states, z.minValue)
	if err != nil {
		return classifyAPIError(err, flags)
	}

	type row struct {
		Council             string  `json:"council"`
		Reference           string  `json:"reference"`
		Address             string  `json:"address"`
		PostCode            string  `json:"post_code,omitempty"`
		Distance            float64 `json:"distance_miles"`
		EstimationValueBand string  `json:"estimation_value_band,omitempty"`
		URL                 string  `json:"url,omitempty"`
	}

	out := []row{}
	for _, ld := range leads {
		app := ld.Application
		if app.Latitude == 0 && app.Longitude == 0 {
			continue
		}
		d := haversineMiles(lat, lon, app.Latitude, app.Longitude)
		if d > radius {
			continue
		}
		out = append(out, row{
			Council:             app.CouncilIdentifier,
			Reference:           app.Reference,
			Address:             app.Address,
			PostCode:            app.PostCode,
			Distance:            roundTo(d, 2),
			EstimationValueBand: app.EstimationValueBand,
			URL:                 app.URL,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Distance < out[j].Distance })
	return emitJSONOrTable(cmd, flags, out, fmt.Sprintf("%d leads within %.1f miles of %s (%.4f,%.4f)", len(out), radius, strings.ToUpper(postcode), lat, lon))
}

// ---- shared helpers ----

func addCommonZazuFlags(cmd *cobra.Command, z *commonZazuFlags) {
	cmd.Flags().StringVar(&z.zazuDB, "zazu-db", "", "Path to ZAZU SQLite mirror, or comma-separated list of mirrors (e.g. 'harrow-mirror.sqlite,bd-mirror.sqlite')")
	cmd.Flags().StringVar(&z.projectTypes, "project-types", "", "Comma-separated project type IDs (Extension, Loft_Conversion, ...)")
	cmd.Flags().IntVar(&z.minValue, "min-value", 0, "Minimum estimated project value in GBP")
	cmd.Flags().StringVar(&z.states, "states", "", "Lead state filter (passes through to /dapi/leads/live-leads)")
}

// leadsCouncils returns the deduplicated set of councilIdentifier values
// present in the fetched lead pull, normalized to lowercase. Used by the
// ZAZU-aware commands so the sheet-name matcher (councilMatchesSheet) can
// fold ZAZU's category-prefixed sheets (e.g. "Harrow Residential") onto
// BuildAlert's bare council slugs (e.g. "harrow").
func leadsCouncils(leads []buildAlertLead) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, 8)
	for _, ld := range leads {
		c := strings.ToLower(strings.TrimSpace(ld.Application.CouncilIdentifier))
		if c == "" {
			continue
		}
		if _, ok := seen[c]; ok {
			continue
		}
		seen[c] = struct{}{}
		out = append(out, c)
	}
	return out
}

func emitJSONOrTable(cmd *cobra.Command, flags *rootFlags, v any, summary string) error {
	jsonOut := flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain)
	if jsonOut {
		raw, err := json.Marshal(v)
		if err != nil {
			return apiErr(err)
		}
		filtered := raw
		if flags.selectFields != "" {
			filtered = filterFields(filtered, flags.selectFields)
		} else if flags.compact {
			filtered = compactFields(filtered)
		}
		return printOutput(cmd.OutOrStdout(), filtered, true)
	}
	if summary != "" {
		fmt.Fprintln(cmd.ErrOrStderr(), summary)
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return apiErr(err)
	}
	var items []map[string]any
	if err := json.Unmarshal(raw, &items); err == nil && len(items) > 0 {
		return printAutoTable(cmd.OutOrStdout(), items)
	}
	fmt.Fprintln(cmd.OutOrStdout(), "(no results)")
	return nil
}

func haversineMiles(lat1, lon1, lat2, lon2 float64) float64 {
	const earthMiles = 3958.7613
	rad := func(d float64) float64 { return d * math.Pi / 180 }
	dLat := rad(lat2 - lat1)
	dLon := rad(lon2 - lon1)
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(rad(lat1))*math.Cos(rad(lat2))*math.Sin(dLon/2)*math.Sin(dLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return earthMiles * c
}

func roundTo(f float64, places int) float64 {
	shift := math.Pow(10, float64(places))
	return math.Round(f*shift) / shift
}

// postcodesClient is a dedicated HTTP client for the postcodes.io lookup with
// an explicit transport timeout. http.DefaultClient has Timeout=0 (unbounded);
// a hung postcodes.io connection would otherwise block `nearby` indefinitely
// even after the parent context is cancelled, because Go's default transport
// only respects cancellation at request boundaries.
var postcodesClient = &http.Client{Timeout: 15 * time.Second}

func lookupPostcode(ctx context.Context, postcode string) (float64, float64, error) {
	clean := strings.ReplaceAll(strings.ToUpper(strings.TrimSpace(postcode)), " ", "")
	if clean == "" {
		return 0, 0, fmt.Errorf("empty postcode")
	}
	url := "https://api.postcodes.io/postcodes/" + clean
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, 0, err
	}
	resp, err := postcodesClient.Do(req)
	if err != nil {
		return 0, 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return 0, 0, fmt.Errorf("postcodes.io HTTP %d: %s", resp.StatusCode, truncate(string(body), 200))
	}
	var payload struct {
		Result struct {
			Latitude  float64 `json:"latitude"`
			Longitude float64 `json:"longitude"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return 0, 0, err
	}
	if payload.Result.Latitude == 0 && payload.Result.Longitude == 0 {
		return 0, 0, fmt.Errorf("postcodes.io returned empty result for %s", clean)
	}
	return payload.Result.Latitude, payload.Result.Longitude, nil
}

var _ = sql.ErrNoRows
