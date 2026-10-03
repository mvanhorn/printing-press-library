// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
// pp:data-source live
package cli

import (
	"encoding/json"
	"fmt"
	"math"
	"time"
	"unicode/utf8"

	"github.com/mvanhorn/printing-press-library/library/travel/carstay/internal/carstay"
	"github.com/mvanhorn/printing-press-library/library/travel/carstay/internal/cliutil"
	"github.com/spf13/cobra"
)

const carstayExampleID = "632c59b82b614b99a252d1b2"

type carstayOptions struct {
	language, prefecture, query, checkIn, checkOut string
	limit, maxScan, maxPages, textLimit            int
	facilities, required                           []string
	lat, lon, radius, length, width, height        float64
}
type carstayView struct {
	Meta          map[string]any      `json:"meta"`
	Results       any                 `json:"results"`
	FetchFailures []map[string]string `json:"-"`
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		spots, _, err := root.Find([]string{"spots"})
		if err != nil || spots == root {
			return
		}
		spots.Short = "Plan designated Carstay overnight stops with original Japanese evidence"
		addNovelCommandIfAbsent(spots, newCarstayFindCmd(flags))
		addNovelCommandIfAbsent(spots, newCarstayShowCmd(flags))
		addNovelCommandIfAbsent(spots, newCarstayHandoffCmd(flags))
		old, _, err := root.Find([]string{"directory"})
		if err == nil && old != root {
			root.RemoveCommand(old)
		}
		root.AddCommand(newCarstayDirectoryCmd(flags))
	})
}
func carstayMeta(command, language string) map[string]any {
	return map[string]any{"source": "live", "provider": "Carstay public website", "command": command, "source_language": "ja", "requested_language": language, "observed_at": time.Now().UTC().Format(time.RFC3339), "timezone": "Asia/Tokyo", "availability": "unknown", "price_basis": "provider_starting_reference_per_night_not_dated_total", "vehicle_acceptance": "unknown"}
}
func configureCarstayCommand(cmd *cobra.Command, flags *rootFlags, kind string, o *carstayOptions) {
	short, example := "", ""
	switch kind {
	case "find":
		short = "Find overnight candidates with Japanese coverage and optional JST date filtering"
		example = "  carstay-pp-cli spots find --prefecture Yamanashi --limit 5 --agent"
	case "show":
		short = "Inspect one station ID for original facilities, rules and parking-space dimensions"
		example = "  carstay-pp-cli spots show " + carstayExampleID + " --agent"
	case "handoff":
		short = "Print a canonical provider booking link for one station ID, with optional JST dates"
		example = "  carstay-pp-cli spots handoff " + carstayExampleID + " --check-in 2026-10-10 --check-out 2026-10-11 --agent"
	case "directory":
		short = "Read a bounded public directory summary including activity markers"
		example = "  carstay-pp-cli directory --limit 5 --agent"
	case "compare":
		short = "Compare 2–5 overnight station IDs with price, facility and parking-space evidence"
		example = "  carstay-pp-cli spots compare " + carstayExampleID + " 5cff4813839680041631c452 --agent"
	case "fit":
		short = "Assess 1–5 station IDs against explicit space or facility requirements; acceptance stays unknown"
		example = "  carstay-pp-cli spots fit " + carstayExampleID + " --length-m 6 --width-m 2.1 --require electricity,restroom --agent"
	case "near":
		short = "Rank designated overnight spots by straight-line distance from required --lat and --lon"
		example = "  carstay-pp-cli spots near --lat 35.5 --lon 138.75 --radius-km 60 --limit 5 --agent"
	case "coverage":
		short = "Audit Japanese overnight coverage, English approval and translated-name gaps"
		example = "  carstay-pp-cli spots coverage --prefecture Yamanashi --agent"
	case "audit":
		short = "Audit 1–5 station IDs for unknown fees, qualified facilities and dated-candidate evidence"
		example = "  carstay-pp-cli spots audit " + carstayExampleID + " --check-in 2026-10-10 --check-out 2026-10-11 --agent"
	}
	scopeRedirect := map[string]string{"compare": " Use this command for contrasting several stations. Use 'spots fit' for a vehicle or facility requirement decision, and 'spots audit' for unresolved evidence.", "fit": " Use this command for explicit dimensional and facility constraints. Use 'spots compare' for a side-by-side shortlist, and 'spots audit' for missing or qualified source evidence.", "audit": " Use this command to inspect unresolved or qualified booking evidence. Use 'spots compare' for station contrasts, and 'spots fit' for explicit requirements."}
	fixture := "--limit=3"
	switch kind {
	case "show", "handoff", "audit":
		fixture = "id=" + carstayExampleID
	case "compare":
		fixture = "first=" + carstayExampleID + ";second=5cff4813839680041631c452"
	case "fit":
		fixture = "id=" + carstayExampleID + ";--length-m=6;--require=electricity,restroom"
	case "near":
		fixture = "--lat=35.5;--lon=138.75;--radius-km=60;--limit=3"
	case "find":
		fixture = "--prefecture=Yamanashi;--limit=3"
	}
	base := &cobra.Command{Short: short, Long: short + scopeRedirect[kind] + ". Public live reads only. Source prices are starting references; date filtering is candidacy, and booking availability remains unknown. --data-source local is unavailable for spots; use sync/search for the separate offline directory mirror.", Example: example, Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": fixture}, RunE: func(cmd *cobra.Command, args []string) error {
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "Carstay "+kind)
		}
		if flags.dataSource == "local" {
			return usageErr(fmt.Errorf("spots and bounded directory views use public live data; --data-source local has no live-equivalent source here; use 'sync --resources directory' then 'search' for the offline mirror"))
		}
		if o.language != "ja" && o.language != "en" {
			return usageErr(fmt.Errorf("--lang must be ja or en; Japanese has fuller coverage"))
		}
		if o.limit < 1 || o.limit > 50 {
			return usageErr(fmt.Errorf("--limit must be 1 to 50"))
		}
		if o.maxScan < 1 || o.maxScan > 5000 {
			return usageErr(fmt.Errorf("--max-scan-records must be 1 to 5000"))
		}
		if o.maxPages < 1 || o.maxPages > 10 {
			return usageErr(fmt.Errorf("--max-scan-pages must be 1 to 10"))
		}
		if o.textLimit < 100 || o.textLimit > 12000 {
			return usageErr(fmt.Errorf("--text-limit must be 100 to 12000 Unicode characters"))
		}
		if (kind == "near" || kind == "coverage") && len(args) > 0 {
			return usageErr(fmt.Errorf("%s takes flags, not positional arguments", kind))
		}
		pref, err := carstay.Prefecture(o.prefecture)
		if err != nil {
			return usageErr(err)
		}
		if err := carstay.Dates(o.checkIn, o.checkOut); err != nil {
			return usageErr(err)
		}
		if kind == "find" {
			if len(args) > 1 || len(args) == 1 && o.query != "" {
				return usageErr(fmt.Errorf("find accepts one query or --query, not both"))
			}
			if len(args) == 1 {
				o.query = args[0]
			}
		}
		if kind == "directory" && len(args) > 0 {
			return usageErr(fmt.Errorf("directory takes no positional arguments"))
		}
		if kind == "show" || kind == "handoff" {
			if len(args) != 1 {
				return usageErr(fmt.Errorf("%s requires one stable station ID; see --help", kind))
			}
		}
		if kind == "show" || kind == "handoff" || kind == "compare" || kind == "fit" || kind == "audit" {
			min := 1
			if kind == "compare" {
				min = 2
			}
			if len(args) < min || len(args) > 5 {
				return usageErr(fmt.Errorf("%s requires %d to 5 distinct station IDs", kind, min))
			}
			seen := map[string]bool{}
			for _, id := range args {
				if !carstay.ValidID(id) {
					return usageErr(fmt.Errorf("invalid station ID %q: expected 24 lowercase hexadecimal characters", id))
				}
				if seen[id] {
					return usageErr(fmt.Errorf("duplicate station ID %s", id))
				}
				seen[id] = true
			}
		}
		if kind == "near" {
			if !cmd.Flags().Changed("lat") || !cmd.Flags().Changed("lon") {
				return usageErr(fmt.Errorf("near requires --lat and --lon"))
			}
			if !finite(o.lat) || !finite(o.lon) || o.lat < -90 || o.lat > 90 || o.lon < -180 || o.lon > 180 || !finite(o.radius) || o.radius <= 0 || o.radius > 2000 {
				return usageErr(fmt.Errorf("use finite latitude [-90,90], longitude [-180,180], and --radius-km (0,2000]"))
			}
		}
		for _, key := range append(append([]string{}, o.facilities...), o.required...) {
			if !carstay.FacilityKnown(key) {
				return usageErr(fmt.Errorf("unknown facility %q; use source keys such as restroom,electricity,petsAllowed,nonFreeShower", key))
			}
		}
		if kind == "fit" {
			if len(o.required) == 0 && !cmd.Flags().Changed("length-m") && !cmd.Flags().Changed("width-m") && !cmd.Flags().Changed("height-m") {
				return usageErr(fmt.Errorf("fit needs --length-m, --width-m, --height-m or --require"))
			}
			for _, d := range []struct {
				flag string
				v    float64
			}{{"length-m", o.length}, {"width-m", o.width}, {"height-m", o.height}} {
				if cmd.Flags().Changed(d.flag) && (!finite(d.v) || d.v <= 0) {
					return usageErr(fmt.Errorf("--%s must be positive and finite", d.flag))
				}
			}
		}
		if !finite(flags.rateLimit) {
			return usageErr(fmt.Errorf("--rate-limit must be finite; public read pacing is capped at 2 requests/second"))
		}
		ctx, cancel := boundCtx(cmd.Context(), flags)
		defer cancel()
		c := carstay.New("", flags.rateLimit)
		// Dogfood still makes real public requests; only cap expensive scan effort.
		if cliutil.IsDogfoodEnv() && o.maxPages > 1 {
			o.maxPages = 1
		}
		meta := carstayMeta(kind, o.language)
		view := carstayView{Meta: meta, FetchFailures: []map[string]string{}}
		if kind == "find" && o.checkIn != "" {
			cand, err := c.DateCandidates(ctx, o.language, o.checkIn, o.checkOut, o.maxPages)
			if err != nil {
				return classifyAPIErrorOnly(err)
			}
			meta["source_language"] = o.language
			meta["check_in_jst"] = o.checkIn
			meta["check_out_jst"] = o.checkOut
			meta["date_candidacy"] = cand.Scope
			meta["upstream_total_candidates"] = cand.Total
			meta["scanned_pages"] = cand.Pages
			meta["scanned_records"] = cand.Scanned
			meta["scan_complete"] = cand.Scanned >= cand.Total
			view.Results = carstayFindRows(cand.Spots, o.query, pref, o.language, o.limit, meta)
			return carstayPrint(cmd, flags, view)
		}
		directory, err := c.Directory(ctx)
		if err != nil {
			return classifyAPIErrorOnly(err)
		}
		meta["upstream_directory_records"] = len(directory)
		if kind == "find" || kind == "directory" {
			scan := directory
			if len(scan) > o.maxScan {
				scan = scan[:o.maxScan]
			}
			meta["scanned_records"] = len(scan)
			meta["scan_complete"] = len(scan) == len(directory)
			if kind == "directory" {
				rows := []carstay.Spot{}
				for _, s := range scan {
					if len(rows) < o.limit {
						rows = append(rows, carstay.Summary(s))
					}
				}
				view.Results = rows
				meta["output_truncated"] = len(scan) > len(rows)
			} else {
				view.Results = carstayFindRows(scan, o.query, pref, o.language, o.limit, meta)
			}
			return carstayPrint(cmd, flags, view)
		}
		selected := []carstay.Spot{}
		for _, id := range args {
			found := false
			for _, s := range directory {
				if s.ID == id {
					if !carstay.Overnight(s) {
						return usageErr(fmt.Errorf("station %s is activity-only or has unknown overnight status", id))
					}
					selected = append(selected, s)
					found = true
					break
				}
			}
			if !found {
				return notFoundErr(fmt.Errorf("station %s was not found in the observed public Japanese directory", id))
			}
		}
		if kind == "handoff" {
			s := selected[0]
			u, err := carstay.Handoff(s, o.language, o.checkIn, o.checkOut)
			if err != nil {
				return usageErr(err)
			}
			meta["check_in_jst"] = o.checkIn
			meta["check_out_jst"] = o.checkOut
			view.Results = []map[string]any{{"id": s.ID, "name_ja": s.Name, "canonical_url": s.SourceURL, "handoff_url": u, "requested_language": o.language, "observed_at": s.ObservedAt, "availability": "unknown", "complete_quote": "unknown", "next_action": "Confirm calendar, vehicle rules, options and total with Carstay; complete booking on the provider website."}}
			return carstayPrint(cmd, flags, view)
		}
		if kind == "show" {
			s, err := c.Detail(ctx, selected[0])
			if err != nil {
				return classifyAPIErrorOnly(err)
			}
			view.Results = []carstay.Spot{carstayBoundText(s, o.textLimit, meta)}
			return carstayPrint(cmd, flags, view)
		}
		return carstayNovelRun(ctx, cmd, flags, c, directory, selected, *o, pref, view)
	}}
	cmd.Short = base.Short
	cmd.Long = base.Long
	cmd.Example = base.Example
	cmd.Annotations = base.Annotations
	cmd.RunE = base.RunE
}
func defaultCarstayOptions() carstayOptions {
	return carstayOptions{language: "ja", limit: 10, maxScan: 500, maxPages: 2, textLimit: 2000, radius: 50, facilities: []string{"restroom", "water", "wifi", "electricity", "nonFreeShower", "wasteWaterDischarge", "petsAllowed", "campingBehaviorAllowed"}}
}

func finite(n float64) bool { return !math.IsNaN(n) && !math.IsInf(n, 0) }
func carstayFindRows(source []carstay.Spot, query, pref, lang string, limit int, meta map[string]any) []carstay.Spot {
	rows := []carstay.Spot{}
	matched := 0
	for _, s := range source {
		if carstay.Match(s, query, pref, lang) {
			matched++
			if len(rows) < limit {
				rows = append(rows, carstay.Summary(s))
			}
		}
	}
	meta["matched_within_scan"] = matched
	meta["output_truncated"] = matched > len(rows)
	if len(rows) == 0 {
		meta["note"] = "No matching overnight records in the examined source. Adjust query/prefecture or widen --max-scan-records / --max-scan-pages when scan_complete is false."
	}
	return rows
}
func carstayPrint(cmd *cobra.Command, flags *rootFlags, v carstayView) error {
	v.Meta["fetch_failures"] = v.FetchFailures
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if flags.selectFields == "" {
		return flags.printJSON(cmd, json.RawMessage(raw))
	}
	selected, selectErr := filterFieldsChecked(raw, flags.selectFields)
	originalSelect, originalAgent := flags.selectFields, flags.agent
	defer func() { flags.selectFields = originalSelect; flags.agent = originalAgent }()
	flags.selectFields = ""
	if flags.agent {
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(selected, &fields); err != nil {
			return err
		}
		if fields == nil {
			fields = map[string]json.RawMessage{}
		}
		if _, hasResults := fields["results"]; !hasResults {
			fields["results"] = json.RawMessage(`[]`)
			v.Meta["results_omitted_by_select"] = true
		}
		// Selection can narrow rows; provenance remains complete in agent mode.
		fields["meta"], err = json.Marshal(v.Meta)
		if err != nil {
			return err
		}
		selected, err = json.Marshal(fields)
		if err != nil {
			return err
		}
	}
	if err := flags.printJSON(cmd, json.RawMessage(selected)); err != nil {
		return err
	}
	return selectErr
}

func carstayBoundText(s carstay.Spot, limit int, meta map[string]any) carstay.Spot {
	cut := func(t string) string {
		if utf8.RuneCountInString(t) > limit {
			meta["source_text_truncated"] = true
			return string([]rune(t)[:limit]) + "… [truncated; see canonical source]"
		}
		return t
	}
	s.Description = cut(s.Description)
	s.DescriptionEN = cut(s.DescriptionEN)
	s.Rules = cut(s.Rules)
	s.BusinessHours = cut(s.BusinessHours)
	for i := range s.Facilities {
		s.Facilities[i].Notification = cut(s.Facilities[i].Notification)
	}
	return s
}

// Defined in the feature file after the three core workflows pass their live review gate.
