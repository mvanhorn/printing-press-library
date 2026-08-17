package cli

import (
	"context"
	"fmt"
	"math"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/bambu/internal/bambu"
	"github.com/mvanhorn/printing-press-library/library/devices/bambu/internal/cliutil"
	"github.com/spf13/cobra"
)

func runBambuETA(cmd *cobra.Command, flags *rootFlags, host string) error {
	if dryRunOK(flags) {
		return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"would_forecast": "current job"}, flags)
	}
	ctx, cancel := boundCtx(cmd.Context(), flags)
	defer cancel()
	_, snapshot, err := fetchBambuStatus(ctx, flags, host)
	if err != nil {
		return err
	}
	if err := persistSnapshot(ctx, snapshot); err != nil {
		return configErr(err)
	}
	history, err := openBambuHistory(ctx)
	if err != nil {
		return configErr(err)
	}
	defer history.Close()
	printerKey, err := selectedPrinterScope(ctx, flags, history)
	if err != nil {
		return err
	}
	jobs, err := allBambuJobs(ctx, history, time.Time{}, printerKey)
	if err != nil {
		return configErr(err)
	}
	corrections := make([]float64, 0)
	for _, job := range jobs {
		if job.Name != snapshot.JobName || job.Outcome != "finished" || job.DurationSeconds == nil || job.InitialRemainingMinutes == nil {
			continue
		}
		corrections = append(corrections, float64(*job.DurationSeconds-int64(*job.InitialRemainingMinutes)*60))
	}
	mean := 0.0
	for _, value := range corrections {
		mean += value
	}
	if len(corrections) > 0 {
		mean /= float64(len(corrections))
	}
	band := 0.0
	for _, value := range corrections {
		band += math.Abs(value - mean)
	}
	if len(corrections) > 0 {
		band /= float64(len(corrections))
	}
	var printerFinish, calibrated *time.Time
	if snapshot.RemainingMinutes != nil {
		base := snapshot.ObservedAt.Add(time.Duration(*snapshot.RemainingMinutes) * time.Minute)
		printerFinish = &base
		corrected := base.Add(time.Duration(mean) * time.Second)
		calibrated = &corrected
	}
	view := map[string]any{"job_name": snapshot.JobName, "observed_at": snapshot.ObservedAt, "printer_finish_at": printerFinish, "calibrated_finish_at": calibrated, "correction_seconds": int64(mean), "error_band_seconds": int64(band), "historical_runs": len(corrections), "basis": "printer remaining time plus mean historical error for the same job"}
	return printJSONFiltered(cmd.OutOrStdout(), view, flags)
}

func runBambuRunway(cmd *cobra.Command, flags *rootFlags, host string) error {
	if dryRunOK(flags) {
		return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"would_estimate": "filament runway"}, flags)
	}
	ctx, cancel := boundCtx(cmd.Context(), flags)
	defer cancel()
	device, snapshot, err := fetchBambuStatus(ctx, flags, host)
	if err != nil {
		return err
	}
	metadata, err := fetchLifecycleMetadata(ctx, device, snapshot)
	if err != nil {
		return apiErr(err)
	}
	_ = persistMetadata(ctx, snapshot, metadata)
	tray := activeTray(snapshot.Raw)
	available := any(nil)
	var availableValue float64
	if tray != nil {
		weight, weightOK := numberValue(tray["tray_weight"])
		remain, remainOK := numberValue(tray["remain"])
		if weightOK && remainOK && remain >= 0 {
			availableValue = weight * remain / 100
			available = availableValue
		}
	}
	required := any(nil)
	margin := any(nil)
	sufficient := any(nil)
	if metadata.WeightGrams != nil {
		fraction := 1.0
		if snapshot.Percent != nil {
			fraction = math.Max(0, 1-float64(*snapshot.Percent)/100)
		}
		need := *metadata.WeightGrams * fraction
		required = need
		if available != nil {
			m := availableValue - need
			margin = m
			sufficient = m >= 0
		}
	}
	view := map[string]any{"job_name": snapshot.JobName, "plate_weight_grams": metadata.WeightGrams, "progress_percent": snapshot.Percent, "required_remaining_grams": required, "active_tray": tray, "estimated_available_grams": available, "margin_grams": margin, "sufficient": sufficient, "limitations": []string{"AMS remain is a printer estimate", "multi-material plate allocation is not inferred when mapping is ambiguous"}}
	return printJSONFiltered(cmd.OutOrStdout(), view, flags)
}

func runBambuRepeats(cmd *cobra.Command, flags *rootFlags, query string) error {
	if dryRunOK(flags) {
		return printJSONFiltered(cmd.OutOrStdout(), []any{}, flags)
	}
	ctx, cancel := boundCtx(cmd.Context(), flags)
	defer cancel()
	history, err := openBambuHistory(ctx)
	if err != nil {
		return configErr(err)
	}
	defer history.Close()
	printerKey, err := selectedPrinterScope(ctx, flags, history)
	if err != nil {
		return err
	}
	jobs, err := allBambuJobs(ctx, history, time.Time{}, printerKey)
	if err != nil {
		return configErr(err)
	}
	if query == "" && len(jobs) > 0 {
		query = jobs[0].Name
	}
	matches := make([]bambu.StoredJob, 0)
	for _, job := range jobs {
		if strings.Contains(strings.ToLower(job.Name), strings.ToLower(query)) {
			matches = append(matches, job)
		}
	}
	durations := make([]int64, 0)
	outcomes := map[string]int{}
	runs := make([]map[string]any, 0, len(matches))
	for _, job := range matches {
		if job.DurationSeconds != nil {
			durations = append(durations, *job.DurationSeconds)
		}
		outcomes[job.Outcome]++
		events, eventErr := history.Events(ctx, job.JobKey, 5000)
		pauses, resumes, errors := 0, 0, 0
		if eventErr == nil {
			for _, event := range events {
				switch event.EventType {
				case "paused":
					pauses++
				case "resumed":
					resumes++
				case "failed", "canceled":
					errors++
				}
			}
		}
		runs = append(runs, map[string]any{"job": job, "pauses": pauses, "resumes": resumes, "terminal_errors": errors})
	}
	var total int64
	for _, v := range durations {
		total += v
	}
	var avg any
	if len(durations) > 0 {
		avg = total / int64(len(durations))
	}
	return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"query": query, "runs": runs, "run_count": len(matches), "average_duration_seconds": avg, "outcomes": outcomes}, flags)
}

func runBambuFieldDiff(cmd *cobra.Command, flags *rootFlags, since string) error {
	if dryRunOK(flags) {
		return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"added": []string{}, "removed": []string{}, "type_changed": []any{}}, flags)
	}
	duration, err := parseSince(since, 7*24*time.Hour)
	if err != nil {
		return usageErr(err)
	}
	ctx, cancel := boundCtx(cmd.Context(), flags)
	defer cancel()
	history, err := openBambuHistory(ctx)
	if err != nil {
		return configErr(err)
	}
	defer history.Close()
	printerKey, err := selectedPrinterScope(ctx, flags, history)
	if err != nil {
		return err
	}
	firstSnapshot, lastSnapshot, observationCount, err := history.SnapshotEndpointsForPrinter(ctx, time.Now().UTC().Add(-duration), printerKey)
	if err != nil {
		return configErr(err)
	}
	if observationCount < 2 {
		return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"observations": observationCount, "added": []string{}, "removed": []string{}, "type_changed": []any{}, "note": "at least two persisted snapshots are required"}, flags)
	}
	first, last := firstSnapshot.Raw, lastSnapshot.Raw
	added, removed := make([]string, 0), make([]string, 0)
	changed := make([]map[string]string, 0)
	for key, value := range last {
		old, ok := first[key]
		if !ok {
			added = append(added, key)
			continue
		}
		oldType, newType := reflect.TypeOf(old), reflect.TypeOf(value)
		if oldType != newType {
			changed = append(changed, map[string]string{"field": key, "before": fmt.Sprint(oldType), "after": fmt.Sprint(newType)})
		}
	}
	for key := range first {
		if _, ok := last[key]; !ok {
			removed = append(removed, key)
		}
	}
	sort.Strings(added)
	sort.Strings(removed)
	sort.Slice(changed, func(i, j int) bool { return changed[i]["field"] < changed[j]["field"] })
	return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"from": firstSnapshot.ObservedAt, "to": lastSnapshot.ObservedAt, "observations": observationCount, "added": added, "removed": removed, "type_changed": changed}, flags)
}

func runBambuFailureCorrelations(cmd *cobra.Command, flags *rootFlags, since string) error {
	if dryRunOK(flags) {
		return printJSONFiltered(cmd.OutOrStdout(), []any{}, flags)
	}
	duration, err := parseSince(since, 30*24*time.Hour)
	if err != nil {
		return usageErr(err)
	}
	ctx, cancel := boundCtx(cmd.Context(), flags)
	defer cancel()
	history, err := openBambuHistory(ctx)
	if err != nil {
		return configErr(err)
	}
	defer history.Close()
	printerKey, err := selectedPrinterScope(ctx, flags, history)
	if err != nil {
		return err
	}
	jobs, err := allBambuJobs(ctx, history, time.Now().UTC().Add(-duration), printerKey)
	if err != nil {
		return configErr(err)
	}
	failed := map[string]bool{}
	for _, job := range jobs {
		if job.Outcome == "failed" {
			failed[job.JobKey] = true
		}
	}
	type count struct{ Total, Failures int }
	counts := map[string]count{}
	seen := map[string]bool{}
	for offset := 0; ; offset += 1000 {
		snapshots, pageErr := history.SnapshotsPage(ctx, time.Now().UTC().Add(-duration), printerKey, "", 1000, offset, false)
		if pageErr != nil {
			return configErr(pageErr)
		}
		for _, snapshot := range snapshots {
			jobKey := bambu.JobKey(snapshot)
			contexts := snapshotContexts(snapshot)
			for dimension, value := range contexts {
				key := dimension + "\x00" + value + "\x00" + jobKey
				if seen[key] {
					continue
				}
				seen[key] = true
				c := counts[dimension+"\x00"+value]
				c.Total++
				if failed[jobKey] {
					c.Failures++
				}
				counts[dimension+"\x00"+value] = c
			}
		}
		if len(snapshots) < 1000 {
			break
		}
	}
	rows := make([]map[string]any, 0, len(counts))
	for key, c := range counts {
		parts := strings.SplitN(key, "\x00", 2)
		rate := 0.0
		if c.Total > 0 {
			rate = float64(c.Failures) / float64(c.Total)
		}
		rows = append(rows, map[string]any{"dimension": parts[0], "value": parts[1], "jobs": c.Total, "failures": c.Failures, "failure_rate": rate})
	}
	sort.Slice(rows, func(i, j int) bool {
		ri := rows[i]["failure_rate"].(float64)
		rj := rows[j]["failure_rate"].(float64)
		if ri == rj {
			return rows[i]["dimension"].(string) < rows[j]["dimension"].(string)
		}
		return ri > rj
	})
	return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"since": time.Now().UTC().Add(-duration), "jobs": len(jobs), "failed_jobs": len(failed), "correlations": rows, "note": "correlation is descriptive and does not establish causation"}, flags)
}

func runBambuTimeline(cmd *cobra.Command, flags *rootFlags, latest bool, jobKey string, limit, offset int) error {
	if limit <= 0 || limit > 2000 || offset < 0 {
		return usageErr(fmt.Errorf("--limit must be 1..2000 and --offset must be non-negative"))
	}
	if dryRunOK(flags) {
		return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"job_key": jobKey, "timeline": []any{}, "entries": 0}, flags)
	}
	ctx, cancel := boundCtx(cmd.Context(), flags)
	defer cancel()
	history, err := openBambuHistory(ctx)
	if err != nil {
		return configErr(err)
	}
	defer history.Close()
	printerKey, err := selectedPrinterScope(ctx, flags, history)
	if err != nil {
		return err
	}
	if latest || jobKey == "" {
		jobs, err := history.JobsForPrinter(ctx, time.Time{}, printerKey, 1)
		if err != nil {
			return configErr(err)
		}
		if len(jobs) == 0 {
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"job_key": nil, "timeline": []any{}, "entries": 0, "note": "no persisted jobs available"}, flags)
		}
		jobKey = jobs[0].JobKey
	}
	if printerKey != "" && !strings.HasPrefix(jobKey, printerKey+"/") {
		jobKey = printerKey + "/" + jobKey
	}
	snapshots, err := history.SnapshotsPage(ctx, time.Time{}, printerKey, jobKey, limit+1, offset, false)
	if err != nil {
		return configErr(err)
	}
	truncated := len(snapshots) > limit
	if truncated {
		snapshots = snapshots[:limit]
	}
	events := []bambu.StoredEvent{}
	if len(snapshots) > 0 {
		eventStart, includeStart := snapshots[0].ObservedAt, offset == 0
		if offset > 0 {
			lookBehind, lookErr := history.SnapshotsPage(ctx, time.Time{}, printerKey, jobKey, 1, offset-1, false)
			if lookErr != nil {
				return configErr(lookErr)
			}
			if len(lookBehind) == 1 {
				eventStart = lookBehind[0].ObservedAt
			}
		}
		events, err = history.EventsBetween(ctx, jobKey, eventStart, snapshots[len(snapshots)-1].ObservedAt, includeStart, 1001)
		if err != nil {
			return configErr(err)
		}
	}
	eventsTruncated := len(events) > 1000
	if eventsTruncated {
		events = events[:1000]
	}
	items := make([]map[string]any, 0)
	for _, event := range events {
		items = append(items, map[string]any{"occurred_at": event.OccurredAt, "type": "transition." + event.EventType, "data": event.Data})
	}
	type recoveryState struct {
		started time.Time
		maxGap  float64
	}
	recoveries := map[string]recoveryState{}
	var stageName string
	var stageStarted, lastObserved time.Time
	for _, snapshot := range snapshots {
		if bambu.JobKey(snapshot) != jobKey {
			continue
		}
		lastObserved = snapshot.ObservedAt
		currentStage := fmt.Sprint(snapshot.Raw["mc_print_stage"])
		if stageName == "" {
			stageName, stageStarted = currentStage, snapshot.ObservedAt
		} else if currentStage != stageName {
			interval := map[string]any{"occurred_at": snapshot.ObservedAt, "type": "stage_interval", "stage": stageName, "started_at": stageStarted, "ended_at": snapshot.ObservedAt, "duration_seconds": int64(snapshot.ObservedAt.Sub(stageStarted).Seconds())}
			if stageStarted == snapshots[0].ObservedAt && offset > 0 {
				interval["started_at_window_boundary"] = true
			}
			items = append(items, interval)
			stageName, stageStarted = currentStage, snapshot.ObservedAt
		}
		for _, heater := range []struct{ name, current, target string }{{"bed", "bed_temper", "bed_target_temper"}, {"nozzle", "nozzle_temper", "nozzle_target_temper"}} {
			current, currentOK := numberValue(snapshot.Raw[heater.current])
			target, targetOK := numberValue(snapshot.Raw[heater.target])
			if !currentOK || !targetOK || target <= 0 {
				continue
			}
			gap := math.Abs(target - current)
			state, active := recoveries[heater.name]
			if gap > 10 && !active {
				recoveries[heater.name] = recoveryState{started: snapshot.ObservedAt, maxGap: gap}
			} else if active {
				if gap > state.maxGap {
					state.maxGap = gap
					recoveries[heater.name] = state
				}
				if gap <= 5 {
					items = append(items, map[string]any{"occurred_at": snapshot.ObservedAt, "type": "temperature_recovery", "heater": heater.name, "started_at": state.started, "recovered_at": snapshot.ObservedAt, "duration_seconds": int64(snapshot.ObservedAt.Sub(state.started).Seconds()), "max_gap_c": state.maxGap})
					delete(recoveries, heater.name)
				}
			}
		}
		items = append(items, map[string]any{"occurred_at": snapshot.ObservedAt, "type": "snapshot", "state": snapshot.State, "percent": snapshot.Percent, "layer": snapshot.CurrentLayer, "stage": snapshot.Raw["mc_print_stage"], "temperatures": temperatureView(snapshot.Raw), "print_error": snapshot.PrintError})
	}
	if stageName != "" && !lastObserved.IsZero() {
		items = append(items, map[string]any{"occurred_at": lastObserved, "type": "stage_interval", "stage": stageName, "started_at": stageStarted, "ended_at": lastObserved, "duration_seconds": int64(lastObserved.Sub(stageStarted).Seconds()), "open": true, "continues_beyond_window": truncated})
	}
	sort.Slice(items, func(i, j int) bool { return fmt.Sprint(items[i]["occurred_at"]) < fmt.Sprint(items[j]["occurred_at"]) })
	result := map[string]any{"job_key": jobKey, "timeline": items, "entries": len(items), "snapshot_limit": limit, "snapshot_offset": offset, "truncated": truncated, "events_truncated": eventsTruncated, "event_limit": 1000, "page_semantics": "independent_snapshot_window", "events_scope": "transitions after the previous page's final snapshot through this page's final snapshot"}
	if truncated {
		result["next_snapshot_offset"] = offset + limit
	}
	return printJSONFiltered(cmd.OutOrStdout(), result, flags)
}

func parseSince(value string, fallback time.Duration) (time.Duration, error) {
	if strings.TrimSpace(value) == "" {
		return fallback, nil
	}
	duration, err := cliutil.ParseDurationLoose(value)
	if err != nil {
		return 0, fmt.Errorf("invalid --since %q: %w", value, err)
	}
	return duration, nil
}

func allBambuJobs(ctx context.Context, history *bambu.History, since time.Time, printerKey string) ([]bambu.StoredJob, error) {
	jobs := make([]bambu.StoredJob, 0)
	for offset := 0; ; offset += 1000 {
		page, err := history.JobsPage(ctx, since, printerKey, 1000, offset)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, page...)
		if len(page) < 1000 {
			return jobs, nil
		}
	}
}

func activeTray(raw map[string]any) map[string]any {
	ams, _ := raw["ams"].(map[string]any)
	unitIndex, trayIndex, active := -1, -1, -1
	if mapping, ok := raw["mapping"].([]any); ok && len(mapping) == 1 {
		if encoded, encodedOK := numberValue(mapping[0]); encodedOK {
			value := int(encoded)
			if value >= 0 && value <= 0xffff {
				if value > 15 {
					unitIndex, trayIndex = (value>>8)&0xff, value&0xff
				} else {
					unitIndex, trayIndex = value/4, value%4
				}
				active = unitIndex*4 + trayIndex
			}
		}
	}
	if unitIndex < 0 || trayIndex < 0 || trayIndex > 3 {
		activeText := fmt.Sprint(ams["tray_now"])
		fallback, err := strconv.Atoi(activeText)
		if err != nil || fallback < 0 || fallback > 15 {
			return nil
		}
		active, unitIndex, trayIndex = fallback, fallback/4, fallback%4
	}
	units, _ := ams["ams"].([]any)
	if unitIndex >= len(units) {
		return nil
	}
	unit, _ := units[unitIndex].(map[string]any)
	trays, _ := unit["tray"].([]any)
	if trayIndex >= len(trays) {
		return nil
	}
	tray, _ := trays[trayIndex].(map[string]any)
	result := map[string]any{"global_id": active, "unit": unitIndex, "slot": trayIndex}
	for _, key := range []string{"tray_type", "tray_sub_brands", "tray_color", "remain", "tray_weight", "tray_uuid"} {
		result[key] = tray[key]
	}
	return result
}
func numberValue(value any) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case int:
		return float64(v), true
	case string:
		n, err := strconv.ParseFloat(v, 64)
		return n, err == nil
	default:
		return 0, false
	}
}
func snapshotContexts(snapshot bambu.Snapshot) map[string]string {
	result := map[string]string{"printer_state": snapshot.State, "speed": fmt.Sprint(snapshot.Raw["spd_lvl"]), "firmware": fmt.Sprint(snapshot.Raw["ver"]), "bed_target": fmt.Sprint(snapshot.Raw["bed_target_temper"]), "nozzle_target": fmt.Sprint(snapshot.Raw["nozzle_target_temper"])}
	if snapshot.PrinterKey != "" {
		result["printer"] = snapshot.PrinterKey
	}
	if snapshot.PlateNumber != nil {
		result["plate"] = strconv.Itoa(*snapshot.PlateNumber)
	}
	if tray := activeTray(snapshot.Raw); tray != nil {
		result["filament"] = fmt.Sprint(tray["tray_type"])
		result["filament_color"] = fmt.Sprint(tray["tray_color"])
	}
	return result
}

var _ context.Context
