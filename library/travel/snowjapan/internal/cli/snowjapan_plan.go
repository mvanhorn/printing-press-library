// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
// pp:data-source local
package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/snowjapan/internal/snowjapan"
	"github.com/mvanhorn/printing-press-library/library/travel/snowjapan/internal/store"
	"github.com/spf13/cobra"
)

func snowNumber(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case json.Number:
		f, e := n.Float64()
		return f, e == nil
	}
	return 0, false
}

func snowMetric(name string) (string, error) {
	switch name {
	case "peak", "peak_m":
		return "peak_m", nil
	case "base", "base_m":
		return "base_m", nil
	case "vertical", "vertical_m":
		return "vertical_m", nil
	case "lifts", "installed_lifts":
		return "installed_lifts", nil
	case "courses":
		return "courses", nil
	case "longest", "longest_course_m":
		return "longest_course_m", nil
	case "steepest", "steepest_degrees":
		return "steepest_degrees", nil
	}
	return "", fmt.Errorf("--maximize supports peak,base,vertical,lifts,courses,longest,steepest")
}
func snowFrontier(rows []snowjapan.Fact, metrics []string) ([]snowjapan.Fact, int) {
	valid := make([]snowjapan.Fact, 0)
	missing := 0
	for _, r := range rows {
		ok := true
		for _, k := range metrics {
			if _, present := snowNumber(r[k]); !present {
				ok = false
			}
		}
		if ok {
			valid = append(valid, r)
		} else {
			missing++
		}
	}
	out := make([]snowjapan.Fact, 0)
	for i, a := range valid {
		dominated := false
		for j, b := range valid {
			if i == j {
				continue
			}
			ge, strict := true, false
			for _, k := range metrics {
				av, _ := snowNumber(a[k])
				bv, _ := snowNumber(b[k])
				if bv < av {
					ge = false
					break
				}
				if bv > av {
					strict = true
				}
			}
			if ge && strict {
				dominated = true
				break
			}
		}
		if !dominated {
			out = append(out, a)
		}
	}
	return out, missing
}

func snowSeasonJoin(rows []snowjapan.Fact) map[string][]snowjapan.Fact {
	out := map[string][]snowjapan.Fact{}
	for _, r := range rows {
		id := snowName(r, "resort_id")
		out[id] = append(out[id], r)
	}
	return out
}

func snowWindow(r snowjapan.Fact, seasons []snowjapan.Fact, from, to time.Time) (snowjapan.Fact, error) {
	out := snowjapan.Fact{"id": r["id"], "name": r["name"], "source_url": r["source_url"], "status": "unknown", "continuous_operation": "unknown", "overlap_calendar_days": 0}
	if len(seasons) != 1 {
		out["evidence_matches"] = len(seasons)
		return out, nil
	}
	s := seasons[0]
	if state := snowName(s, "endpoint_evidence_state"); state != "" && state != "recorded_span" {
		out["endpoint_evidence_state"] = state
		return out, nil
	}
	first, e := time.Parse("2006-01-02", snowName(s, "first_recorded_day"))
	if e != nil {
		return nil, fmt.Errorf("saved historical opening date invalid")
	}
	last, e := time.Parse("2006-01-02", snowName(s, "last_recorded_day"))
	if e != nil || first.After(last) {
		return nil, fmt.Errorf("saved historical closing date invalid")
	}
	a, z := from, to
	if first.After(a) {
		a = first
	}
	if last.Before(z) {
		z = last
	}
	out["first_recorded_day"] = s["first_recorded_day"]
	out["last_recorded_day"] = s["last_recorded_day"]
	out["season_observed_at"] = s["observed_at"]
	if a.After(z) {
		out["status"] = "outside_recorded_span"
		return out, nil
	}
	out["status"] = "partial_overlap"
	if !from.Before(first) && !to.After(last) {
		out["status"] = "within_recorded_span"
	}
	out["overlap_from"] = a.Format("2006-01-02")
	out["overlap_to"] = z.Format("2006-01-02")
	out["overlap_calendar_days"] = int(z.Sub(a).Hours()/24) + 1
	return out, nil
}

func snowChanges(newer, older snowjapan.Fact) []snowjapan.Fact {
	keys := map[string]bool{}
	for k := range newer {
		keys[k] = true
	}
	for k := range older {
		keys[k] = true
	}
	names := make([]string, 0, len(keys))
	for k := range keys {
		if k != "observed_at" && k != "catalog_observed_at" && k != "detail_observed_at" && k != "id" && k != "source_url" && k != "projection" {
			names = append(names, k)
		}
	}
	sort.Strings(names)
	out := make([]snowjapan.Fact, 0)
	for _, k := range names {
		a, ap := older[k]
		b, bp := newer[k]
		if ap == bp && reflect.DeepEqual(a, b) {
			continue
		}
		out = append(out, snowjapan.Fact{"field": k, "before": a, "after": b, "before_present": ap, "after_present": bp})
	}
	return out
}

func snowTownView(rows []snowjapan.Fact, joined map[string][]snowjapan.Fact) []snowjapan.Fact {
	groups := map[string]snowjapan.Fact{}
	for _, r := range rows {
		key := snowName(r, "prefecture") + "\x00" + snowName(r, "town")
		g := groups[key]
		if g == nil {
			g = snowjapan.Fact{"town": r["town"], "prefecture": r["prefecture"], "listed_areas": 0, "confirmed_endpoint_areas": 0, "missing_endpoint_areas": 0, "ambiguous_endpoint_areas": 0, "inconsistent_endpoint_areas": 0, "min_vertical_m": nil, "max_vertical_m": nil, "unknown_vertical_areas": 0, "examples": []string{}}
			groups[key] = g
		}
		g["listed_areas"] = g["listed_areas"].(int) + 1
		n := len(joined[snowName(r, "id")])
		if n == 1 {
			state := snowName(joined[snowName(r, "id")][0], "endpoint_evidence_state")
			if state == "" || state == "recorded_span" {
				g["confirmed_endpoint_areas"] = g["confirmed_endpoint_areas"].(int) + 1
			} else {
				g["inconsistent_endpoint_areas"] = g["inconsistent_endpoint_areas"].(int) + 1
			}
		} else if n == 0 {
			g["missing_endpoint_areas"] = g["missing_endpoint_areas"].(int) + 1
		} else {
			g["ambiguous_endpoint_areas"] = g["ambiguous_endpoint_areas"].(int) + 1
		}
		v, ok := snowNumber(r["vertical_m"])
		if !ok {
			g["unknown_vertical_areas"] = g["unknown_vertical_areas"].(int) + 1
		} else {
			lo, known := snowNumber(g["min_vertical_m"])
			if !known || v < lo {
				g["min_vertical_m"] = v
			}
			hi, known := snowNumber(g["max_vertical_m"])
			if !known || v > hi {
				g["max_vertical_m"] = v
			}
		}
		examples := g["examples"].([]string)
		if len(examples) < 3 {
			g["examples"] = append(examples, snowName(r, "name"))
		}
	}
	out := make([]snowjapan.Fact, 0, len(groups))
	for _, g := range groups {
		out = append(out, g)
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i]["listed_areas"].(int), out[j]["listed_areas"].(int)
		if a != b {
			return a > b
		}
		return snowName(out[i], "prefecture")+snowName(out[i], "town") < snowName(out[j], "prefecture")+snowName(out[j], "town")
	})
	return out
}

func newSnowPlan(flags *rootFlags, kind string) *cobra.Command {
	cmd := &cobra.Command{Use: kind}
	configureSnowPlan(cmd, flags, kind)
	return cmd
}

func configureSnowPlan(cmd *cobra.Command, flags *rootFlags, kind string) {
	var pref, season, resortCSV, fromText, toText string
	maximize := "vertical,courses,longest"
	limit := 20
	short := map[string]string{"frontier": "Find nondominated choices across selected published resort statistics.", "windows": "Intersect a past trip window with confirmed first/last season endpoints.", "towns": "Compare exact source municipalities and their saved resort options.", "coverage": "Audit missing historical endpoint evidence for saved directory members.", "changes": "Compare the latest two saved factual observations of exact resorts."}
	fixtures := map[string]string{"frontier": "--prefecture=Nagano;--maximize=vertical,courses,longest;--limit=5", "windows": "--season=2025-2026;--from=2026-03-28;--to=2026-04-05;--resorts=able-hakuba-goryu", "towns": "--prefecture=Nagano;--season=2025-2026;--limit=5", "coverage": "--prefecture=Nagano;--season=2025-2026;--limit=5", "changes": "--resorts=able-hakuba-goryu"}
	long := map[string]string{"frontier": "Use this command for explicit numeric resort tradeoffs. Use 'plan towns' for municipality comparisons.", "windows": "Use this command for historical trip-date span boundaries. Use 'plan coverage' for missing evidence. Recorded spans do not establish uninterrupted operation.", "towns": "Use this command for source-defined municipality option portfolios. Use 'plan frontier' for individual resort tradeoffs. Areas may share terrain; totals are not combined terrain.", "coverage": "Use this command for missing historical endpoint evidence. Use 'plan windows' for a specific past trip-date range. Missing evidence does not mean closure.", "changes": "Use this command for edits between saved observations. Use 'plan windows' for historical trip dates. Run explicit sync again to capture a second observation."}
	cmd.Short = short[kind]
	cmd.Long = long[kind]
	cmd.Example = "  snowjapan-pp-cli plan " + kind + " " + strings.ReplaceAll(fixtures[kind], ";", " ") + " --agent"
	cmd.Annotations = map[string]string{"mcp:read-only": "true", "pp:data-source": "local", "pp:happy-args": snowHappyArgs(fixtures[kind])}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "plan "+kind)
		}
		if e := validateDataSourceStrategy(flags, "local"); e != nil {
			return usageErr(e)
		}
		if len(args) > 0 {
			return usageErr(fmt.Errorf("plan %s uses flags, not positional arguments", kind))
		}
		if e := snowLimit(limit); e != nil {
			return e
		}
		if kind == "windows" || kind == "towns" || kind == "coverage" {
			if e := snowjapan.ValidateSeason(season); e != nil {
				return usageErr(e)
			}
		}
		if kind == "windows" || kind == "changes" || (kind == "coverage" && resortCSV != "") {
			if resortCSV == "" || len(strings.Split(resortCSV, ",")) > 4 {
				return usageErr(fmt.Errorf("--resorts requires one to four exact ids or unique slugs"))
			}
		}
		var from, to time.Time
		if kind == "windows" {
			var e error
			from, e = time.Parse("2006-01-02", fromText)
			if e != nil {
				return usageErr(fmt.Errorf("--from must be YYYY-MM-DD"))
			}
			to, e = time.Parse("2006-01-02", toText)
			if e != nil || from.After(to) {
				return usageErr(fmt.Errorf("--to must be YYYY-MM-DD on or after --from"))
			}
			start, _ := time.Parse("2006-01-02", season[:4]+"-09-01")
			end := start.AddDate(1, 0, 0)
			if from.Before(start) || !to.Before(end) {
				return usageErr(fmt.Errorf("date range must lie within the selected historical winter's September-to-August evidence window"))
			}
		}
		metrics := make([]string, 0)
		if kind == "frontier" {
			seen := map[string]bool{}
			for _, m := range strings.Split(maximize, ",") {
				k, e := snowMetric(m)
				if e != nil {
					return usageErr(e)
				}
				if seen[k] {
					return usageErr(fmt.Errorf("duplicate --maximize metric"))
				}
				seen[k] = true
				metrics = append(metrics, k)
			}
			if len(metrics) > 3 {
				return usageErr(fmt.Errorf("--maximize accepts at most three metrics"))
			}
		}
		ctx, cancel := boundCtx(cmd.Context(), flags)
		defer cancel()
		rows, complete, e := snowLocal(ctx, cmd, flags, "resorts", "", true)
		if e != nil {
			return e
		}
		filtered := make([]snowjapan.Fact, 0)
		for _, r := range rows {
			if snowMatches(r, "", pref) {
				filtered = append(filtered, r)
			}
		}
		if kind == "coverage" && resortCSV != "" {
			filtered, e = snowExactScope(filtered, resortCSV)
			if e != nil {
				return usageErr(e)
			}
		}
		view := map[string]any{"results": []snowjapan.Fact{}, "scanned_resorts": len(rows), "scoped_resorts": len(filtered), "complete_saved_directory": complete, "season": season, "continuous_operation": "unknown"}
		firstObserved, lastObserved := snowObservationRange(rows)
		view["directory_source_url"] = "https://www.snowjapan.com/insights/japan-ski-areas-statistics"
		view["directory_observed_from"] = firstObserved
		view["directory_observed_to"] = lastObserved
		if !complete {
			view["note"] = "The local directory population is incomplete; run sync --resources resorts for a complete captured source population."
		}
		if len(rows) == 0 {
			return snowPrint(cmd, flags, view, "local")
		}
		var history []snowjapan.Fact
		if kind == "windows" || kind == "towns" || kind == "coverage" {
			var captured bool
			history, captured, e = snowLocal(ctx, cmd, flags, "seasons", season, false)
			if e != nil {
				return e
			}
			view["scanned_season_rows"] = len(history)
			view["requested_season_captured"] = captured
			firstObserved, lastObserved = snowObservationRange(history)
			view["season_source_url"] = "https://www.snowjapan.com/insights/" + season + "-ski-season-dates-sort"
			view["season_observed_from"] = firstObserved
			view["season_observed_to"] = lastObserved
			if !captured {
				view["evidence_state"] = "season_not_captured"
				view["note"] = "The requested winter has no complete local capture; missing source records cannot be inferred. Run sync --resources seasons --resource-param seasons:season=" + season + "."
				return snowPrint(cmd, flags, view, "local")
			}
		}
		joined := snowSeasonJoin(history)
		switch kind {
		case "frontier":
			out, missing := snowFrontier(filtered, metrics)
			view["maximize"] = metrics
			view["frontier_total"] = len(out)
			view["missing_metric_records"] = missing
			if len(out) > limit {
				out = out[:limit]
			}
			view["results"] = out
		case "towns":
			out := snowTownView(filtered, joined)
			view["towns_total"] = len(out)
			if len(out) > limit {
				out = out[:limit]
			}
			view["results"] = out
		case "coverage":
			out := make([]snowjapan.Fact, 0)
			confirmed, missing, ambiguous, inconsistent := 0, 0, 0, 0
			for _, r := range filtered {
				n := len(joined[snowName(r, "id")])
				state := "confirmed_endpoints"
				if n == 0 {
					state = "missing_source_evidence"
					missing++
				} else if n > 1 {
					state = "ambiguous_source_evidence"
					ambiguous++
				} else if endpointState := snowName(joined[snowName(r, "id")][0], "endpoint_evidence_state"); endpointState != "" && endpointState != "recorded_span" {
					state = "inconsistent_source_evidence"
					inconsistent++
				} else {
					confirmed++
				}
				if state != "confirmed_endpoints" {
					out = append(out, snowjapan.Fact{"id": r["id"], "name": r["name"], "source_url": r["source_url"], "evidence_state": state, "matches": n})
				}
			}
			view["denominator"] = len(filtered)
			view["confirmed"] = confirmed
			view["missing"] = missing
			view["ambiguous"] = ambiguous
			view["inconsistent"] = inconsistent
			if len(out) > limit {
				out = out[:limit]
			}
			view["results"] = out
			view["missing_means"] = "unknown endpoint evidence, not closure"
		case "windows":
			out := make([]snowjapan.Fact, 0)
			seen := map[string]bool{}
			for _, id := range strings.Split(resortCSV, ",") {
				r, e := snowResolve(rows, id)
				if e != nil {
					return usageErr(e)
				}
				key := snowName(r, "id")
				if seen[key] {
					return usageErr(fmt.Errorf("duplicate --resorts id"))
				}
				seen[key] = true
				v, e := snowWindow(r, joined[key], from, to)
				if e != nil {
					return e
				}
				out = append(out, v)
			}
			view["from"] = fromText
			view["to"] = toText
			view["results"] = out
		case "changes":
			out, e := snowSnapshotChanges(ctx, cmd, rows, resortCSV)
			if e != nil {
				return e
			}
			view["results"] = out
		}
		return snowPrint(cmd, flags, view, "local")
	}
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum computed result rows to return, 1..200")
	if kind == "frontier" || kind == "towns" || kind == "coverage" {
		cmd.Flags().StringVar(&pref, "prefecture", "", "Exact source prefecture to scope the saved population")
	}
	if kind == "frontier" {
		cmd.Flags().StringVar(&maximize, "maximize", "vertical,courses,longest", "One to three numeric statistics to maximize; this does not assign suitability weights")
	}
	if kind == "windows" || kind == "towns" || kind == "coverage" {
		cmd.Flags().StringVar(&season, "season", "", "Completed source season YYYY-YYYY to use for endpoint evidence")
	}
	if kind == "windows" || kind == "changes" || kind == "coverage" {
		cmd.Flags().StringVar(&resortCSV, "resorts", "", "One to four comma-separated canonical ids or unique exact slugs")
	}
	if kind == "windows" {
		cmd.Flags().StringVar(&fromText, "from", "", "First historical trip date, inclusive, YYYY-MM-DD")
		cmd.Flags().StringVar(&toText, "to", "", "Last historical trip date, inclusive, YYYY-MM-DD")
	}
}

func snowObservationRange(rows []snowjapan.Fact) (string, string) {
	first, last := "", ""
	for _, r := range rows {
		at := snowName(r, "observed_at")
		if at != "" && (first == "" || at < first) {
			first = at
		}
		if at > last {
			last = at
		}
	}
	return first, last
}

func snowExactScope(rows []snowjapan.Fact, csv string) ([]snowjapan.Fact, error) {
	ids := strings.Split(csv, ",")
	if csv == "" || len(ids) > 4 {
		return nil, fmt.Errorf("--resorts requires one to four exact ids or unique slugs")
	}
	out := make([]snowjapan.Fact, 0, len(ids))
	seen := map[string]bool{}
	for _, id := range ids {
		r, e := snowResolve(rows, strings.TrimSpace(id))
		if e != nil {
			return nil, e
		}
		key := snowName(r, "id")
		if seen[key] {
			return nil, fmt.Errorf("duplicate --resorts id")
		}
		seen[key] = true
		out = append(out, r)
	}
	return out, nil
}

func snowSnapshotChanges(ctx context.Context, cmd *cobra.Command, rows []snowjapan.Fact, ids string) (result []snowjapan.Fact, err error) {
	db, e := store.OpenSnowJapanReadOnlyContext(ctx, snowDBPath(cmd))
	if e != nil {
		return nil, e
	}
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			result = nil
			err = errors.Join(err, closeErr)
		}
	}()
	out := make([]snowjapan.Fact, 0)
	seen := map[string]bool{}
	for _, id := range strings.Split(ids, ",") {
		r, e := snowResolve(rows, id)
		if e != nil {
			return nil, usageErr(e)
		}
		key := snowName(r, "id")
		if seen[key] {
			return nil, usageErr(fmt.Errorf("duplicate --resorts id"))
		}
		seen[key] = true
		pair, e := db.SnowJapanSnapshotPair(ctx, key)
		if e != nil {
			return nil, e
		}
		v := snowjapan.Fact{"id": key, "name": r["name"], "source_url": r["source_url"], "baseline_status": "missing_baseline", "changes": []snowjapan.Fact{}, "saved_observations": len(pair)}
		if len(pair) == 2 {
			var newer, older snowjapan.Fact
			if e = json.Unmarshal(pair[0], &newer); e != nil {
				return nil, e
			}
			if e = json.Unmarshal(pair[1], &older); e != nil {
				return nil, e
			}
			if newer["projection"] != older["projection"] {
				return nil, fmt.Errorf("incompatible saved source projections")
			}
			v["baseline_status"] = "available"
			v["changes"] = snowChanges(newer, older)
			v["older_observed_at"] = older["observed_at"]
			v["newer_observed_at"] = newer["observed_at"]
		}
		out = append(out, v)
	}
	return out, nil
}
