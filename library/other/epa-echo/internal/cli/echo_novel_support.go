// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func echoGet(ctx context.Context, flags *rootFlags, path string, params url.Values) (map[string]any, error) {
	if params == nil {
		params = url.Values{}
	}
	params.Set("output", "JSON")
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	raw, err := c.GetWithHeadersValues(ctx, path, params, map[string]string{"Accept": "application/json"})
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.UseNumber()
	var out map[string]any
	if err := decoder.Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}

func echoResults(response map[string]any) map[string]any {
	if value, ok := response["Results"].(map[string]any); ok {
		return value
	}
	return map[string]any{}
}

func searchEcho(ctx context.Context, flags *rootFlags, params url.Values) (map[string]any, []map[string]any, error) {
	params.Set("responseset", "100")
	summary, err := echoGet(ctx, flags, "/echo_rest_services.get_facilities", params)
	if err != nil {
		return nil, nil, err
	}
	results := echoResults(summary)
	qid := fmt.Sprint(results["QueryID"])
	if qid == "" || qid == "<nil>" {
		return results, []map[string]any{}, nil
	}
	page, err := echoGet(ctx, flags, "/echo_rest_services.get_qid", url.Values{"qid": {qid}, "pageno": {"1"}})
	if err != nil {
		return nil, nil, err
	}
	var facilities []map[string]any
	if raw, ok := echoResults(page)["Facilities"].([]any); ok {
		for _, item := range raw {
			if row, ok := item.(map[string]any); ok {
				facilities = append(facilities, row)
			}
		}
	}
	return results, facilities, nil
}

func getDFR(ctx context.Context, flags *rootFlags, id string) (map[string]any, error) {
	response, err := echoGet(ctx, flags, "/dfr_rest_services.get_dfr", url.Values{"p_id": {id}})
	if err != nil {
		return nil, err
	}
	return echoResults(response), nil
}

func changedSections(previous, current map[string]any) []map[string]any {
	keys := map[string]bool{}
	for k := range previous {
		keys[k] = true
	}
	for k := range current {
		keys[k] = true
	}
	var out []map[string]any
	for key := range keys {
		before, _ := json.Marshal(previous[key])
		after, _ := json.Marshal(current[key])
		if string(before) != string(after) {
			out = append(out, map[string]any{"section": key, "previous": previous[key], "current": current[key]})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i]["section"].(string) < out[j]["section"].(string) })
	return out
}

func recordLevelChanges(previous, current map[string]any) map[string]any {
	prev := normalizedSectionRecords(previous)
	curr := normalizedSectionRecords(current)
	index := func(rows []map[string]any) map[string]map[string]any {
		out := map[string]map[string]any{}
		for _, row := range rows {
			raw, _ := json.Marshal(row)
			out[string(raw)] = row
		}
		return out
	}
	p, c := index(prev), index(curr)
	added, removed := []map[string]any{}, []map[string]any{}
	for key, row := range c {
		if _, ok := p[key]; !ok {
			added = append(added, row)
		}
	}
	for key, row := range p {
		if _, ok := c[key]; !ok {
			removed = append(removed, row)
		}
	}
	return map[string]any{"added_count": len(added), "removed_count": len(removed), "added": added, "removed": removed}
}

func facilityConcern(row map[string]any) map[string]any {
	return map[string]any{"registry_id": row["RegistryID"], "facility_name": row["FacName"], "address": strings.TrimSpace(fmt.Sprintf("%v, %v, %v %v", row["FacStreet"], row["FacCity"], row["FacState"], row["FacZip"])), "latitude": row["FacLat"], "compliance_status": row["FacComplianceStatus"], "significant_noncompliance": row["FacSNCFlg"], "quarters_with_noncompliance": row["FacQtrsWithNC"], "last_inspection": row["FacDateLastInspection"], "inspection_count": row["FacInspectionCount"], "last_formal_action": row["FacDateLastFormalAction"], "penalty_count": row["FacPenaltyCount"], "last_penalty": row["FacDateLastPenalty"]}
}

func rankedFacility(row map[string]any, query map[string]string) map[string]any {
	out := facilityConcern(row)
	score := 0
	matched, conflicts := []string{}, []map[string]any{}
	compare := func(label, wanted string, raw any, exactPoints, containsPoints int) {
		wanted = strings.ToUpper(strings.TrimSpace(wanted))
		got := strings.ToUpper(strings.TrimSpace(fmt.Sprint(raw)))
		if wanted == "" {
			return
		}
		switch {
		case got == wanted:
			score += exactPoints
			matched = append(matched, label+":exact")
		case containsPoints > 0 && (strings.Contains(got, wanted) || strings.Contains(wanted, got)):
			score += containsPoints
			matched = append(matched, label+":contains")
		default:
			conflicts = append(conflicts, map[string]any{"field": label, "query": wanted, "result": got})
		}
	}
	compare("frs", query["frs"], row["RegistryID"], 100, 0)
	compare("name", query["name"], row["FacName"], 50, 30)
	compare("state", query["state"], row["FacState"], 15, 0)
	compare("city", query["city"], row["FacCity"], 10, 0)
	compare("zip", query["zip"], row["FacZip"], 10, 0)
	identifiers := map[string]any{"frs_registry_id": row["RegistryID"]}
	for key, value := range row {
		lower := strings.ToLower(key)
		if key != "RegistryID" && (strings.HasSuffix(lower, "id") || strings.Contains(lower, "sourceid")) && value != nil && fmt.Sprint(value) != "" {
			identifiers[key] = value
		}
	}
	out["match_score"] = score
	out["match_evidence"] = matched
	out["conflicts"] = conflicts
	out["identifiers"] = identifiers
	return out
}

func queryRowCount(summary map[string]any) int {
	n, _ := strconv.Atoi(fmt.Sprint(summary["QueryRows"]))
	return n
}

func selectedSections(report map[string]any, names ...string) map[string]any {
	out := map[string]any{}
	for _, name := range names {
		if value, ok := report[name]; ok {
			out[name] = value
		}
	}
	return out
}

func normalizedSectionRecords(sections map[string]any) []map[string]any {
	out := make([]map[string]any, 0)
	for section, value := range sections {
		collectRecords(section, value, &out)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return fmt.Sprint(out[i]["period"])+fmt.Sprint(out[i]["section"])+fmt.Sprint(out[i]["identifiers"]) < fmt.Sprint(out[j]["period"])+fmt.Sprint(out[j]["section"])+fmt.Sprint(out[j]["identifiers"])
	})
	return out
}

func collectRecords(section string, value any, out *[]map[string]any) {
	switch typed := value.(type) {
	case []any:
		for _, item := range typed {
			collectRecords(section, item, out)
		}
	case map[string]any:
		hasScalar := false
		for _, v := range typed {
			switch v.(type) {
			case map[string]any, []any:
			default:
				hasScalar = true
			}
		}
		if hasScalar {
			normalized := map[string]any{"section": section, "identifiers": recordIdentifiers(typed), "record": typed}
			keys := make([]string, 0, len(typed))
			for key := range typed {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				val := typed[key]
				lower := strings.ToLower(key)
				switch {
				case strings.Contains(lower, "pollut") || strings.Contains(lower, "parameter"):
					if _, ok := normalized["pollutant"]; !ok {
						normalized["pollutant"] = val
					}
				case strings.Contains(lower, "date") || strings.Contains(lower, "period") || strings.Contains(lower, "year"):
					if _, ok := normalized["period"]; !ok {
						normalized["period"] = val
					}
				case strings.Contains(lower, "exceed"):
					if _, ok := normalized["exceedance"]; !ok {
						normalized["exceedance"] = val
					}
				}
			}
			*out = append(*out, normalized)
		}
		for _, child := range typed {
			collectRecords(section, child, out)
		}
	}
}

func explicitExceedanceCount(records []map[string]any) int {
	count := 0
	for _, row := range records {
		v, ok := row["exceedance"]
		if !ok {
			continue
		}
		s := strings.ToLower(strings.TrimSpace(fmt.Sprint(v)))
		if s == "y" || s == "yes" || s == "true" {
			count++
		}
		if n, err := strconv.ParseFloat(strings.ReplaceAll(s, ",", ""), 64); err == nil && n > 0 {
			count++
		}
	}
	return count
}

func timelineEntries(report map[string]any) []map[string]any {
	sources := []string{"InspectionEnforcementSummary", "ComplianceHistory", "ViolationsEnforcementActions", "FormalActions", "ICISFormalActions", "CaseFormalActions", "EnforcementComplianceSummaries"}
	entries := make([]map[string]any, 0)
	for _, source := range sources {
		collectDated(source, report[source], &entries)
	}
	sort.SliceStable(entries, func(i, j int) bool { return fmt.Sprint(entries[i]["date"]) < fmt.Sprint(entries[j]["date"]) })
	return entries
}

func collectDated(source string, value any, out *[]map[string]any) {
	switch typed := value.(type) {
	case []any:
		for _, item := range typed {
			collectDated(source, item, out)
		}
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			val := typed[key]
			switch val.(type) {
			case map[string]any, []any:
				continue
			}
			if !isEchoEventDateField(key) || fmt.Sprint(val) == "<nil>" || fmt.Sprint(val) == "" {
				continue
			}
			*out = append(*out, map[string]any{
				"date":           normalizeEchoDate(fmt.Sprint(val)),
				"date_field":     key,
				"identifiers":    recordIdentifiers(typed),
				"source_section": source,
				"record":         typed,
			})
		}
		for _, child := range typed {
			collectDated(source, child, out)
		}
	}
}

func recordIdentifiers(record map[string]any) map[string]any {
	out := map[string]any{}
	for key, value := range record {
		lower := strings.ToLower(key)
		if (strings.HasSuffix(lower, "id") || strings.Contains(lower, "number")) && value != nil && fmt.Sprint(value) != "" {
			out[key] = value
		}
	}
	return out
}

func filterTimelineSince(entries []map[string]any, cutoff time.Time) []map[string]any {
	if cutoff.IsZero() {
		return entries
	}
	out := make([]map[string]any, 0, len(entries))
	for _, entry := range entries {
		date, err := time.Parse("2006-01-02", fmt.Sprint(entry["date"]))
		if err != nil {
			copy := make(map[string]any, len(entry)+1)
			for key, value := range entry {
				copy[key] = value
			}
			copy["since_filter_status"] = "unparsed_date_included"
			out = append(out, copy)
		} else if !date.Before(cutoff) {
			out = append(out, entry)
		}
	}
	return out
}

func parseLookback(raw string) (time.Duration, error) {
	raw = strings.TrimSpace(strings.ToLower(raw))
	if raw == "" {
		return 0, nil
	}
	multiplier := 0.0
	if strings.HasSuffix(raw, "d") {
		multiplier, raw = 24, strings.TrimSuffix(raw, "d")
	} else if strings.HasSuffix(raw, "y") {
		multiplier, raw = 24*365, strings.TrimSuffix(raw, "y")
	}
	if multiplier > 0 {
		n, err := strconv.ParseFloat(raw, 64)
		if err != nil || n <= 0 {
			return 0, fmt.Errorf("invalid lookback")
		}
		return time.Duration(n * multiplier * float64(time.Hour)), nil
	}
	return time.ParseDuration(raw)
}

// ProgramDates start/end values describe the detailed report's lookback
// window. They are coverage metadata, not inspection or enforcement events.
func isEchoEventDateField(key string) bool {
	normalized := strings.ToLower(strings.TrimSpace(key))
	if !strings.Contains(normalized, "date") {
		return false
	}
	switch normalized {
	case "startdate", "startdate3yr", "startdate5yr", "enddate":
		return false
	default:
		return true
	}
}

func normalizeEchoDate(raw string) string {
	for _, layout := range []string{"01/02/2006", "2006-01-02", "20060102", time.RFC3339} {
		if parsed, err := time.Parse(layout, raw); err == nil {
			return parsed.Format("2006-01-02")
		}
	}
	return raw
}

func emitECHO(cmd *cobra.Command, flags *rootFlags, source string, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return printOutputWithFlagsMeta(cmd.OutOrStdout(), raw, flags, map[string]any{"source": source, "provider": "US EPA ECHO", "retrieved_at": time.Now().UTC().Format(time.RFC3339)})
}

func echoCaveats() []string {
	return []string{"ECHO integrates multiple program systems; preserve FRS and program identifiers rather than merging facilities by name alone.", "Missing fields may reflect program coverage or reporting timing and are not a clean compliance grade.", "The CLI reports source evidence and changes; it does not compute a composite environmental risk score."}
}
