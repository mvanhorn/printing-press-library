package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/bambu/internal/bambu"
	"github.com/mvanhorn/printing-press-library/library/devices/bambu/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/devices/bambu/internal/store"
	"github.com/spf13/cobra"
)

func openBambuHistory(ctx context.Context) (*bambu.History, error) {
	return bambu.OpenHistory(ctx, defaultDBPath("bambu-pp-cli"))
}

func persistSnapshot(ctx context.Context, snapshot bambu.Snapshot) error {
	history, err := openBambuHistory(ctx)
	if err != nil {
		return err
	}
	if err := history.RecordSnapshot(ctx, snapshot); err != nil {
		_ = history.Close()
		return err
	}
	if err := history.Close(); err != nil {
		return err
	}
	compact := bambu.SanitizeSnapshot(snapshot)
	compact.Raw = nil
	return mirrorObservation(ctx, "status", snapshot.ObservedAt, compact.PrinterKey, compact)
}

func persistEvent(ctx context.Context, event bambu.Event) error {
	history, err := openBambuHistory(ctx)
	if err != nil {
		return err
	}
	defer history.Close()
	if err := history.RecordLifecycleEvent(ctx, event); err != nil {
		return err
	}
	compact := bambu.SanitizeSnapshot(event.Snapshot)
	compact.Raw = nil
	return mirrorObservation(ctx, "transition."+event.Kind, event.Snapshot.ObservedAt, compact.PrinterKey, compact)
}

func mirrorObservation(ctx context.Context, observationType string, observedAt time.Time, printerKey string, payload any) error {
	if observedAt.IsZero() {
		observedAt = time.Now().UTC()
	}
	data, err := mirroredObservationData(observationType, observedAt, printerKey, payload)
	if err != nil {
		return err
	}
	s, err := store.OpenWithContext(ctx, defaultDBPath("bambu-pp-cli"))
	if err != nil {
		return err
	}
	defer s.Close()
	if err := os.Chmod(defaultDBPath("bambu-pp-cli"), 0o600); err != nil {
		return err
	}
	return s.UpsertObservations(data)
}

func mirroredObservationData(observationType string, observedAt time.Time, printerKey string, payload any) ([]byte, error) {
	id := fmt.Sprintf("%s-%s-%d", strings.ReplaceAll(observationType, ".", "-"), printerKey, observedAt.UnixNano())
	return json.Marshal(map[string]any{"id": id, "observed_at": observedAt, "printer_serial": printerKey, "observation_type": observationType, "payload": payload})
}

func persistMetadata(ctx context.Context, snapshot bambu.Snapshot, metadata bambu.Metadata) error {
	history, err := openBambuHistory(ctx)
	if err != nil {
		return err
	}
	defer history.Close()
	if err := history.RecordSnapshot(ctx, snapshot); err != nil {
		return err
	}
	return history.RecordMetadata(ctx, snapshot, metadata)
}

func newSyncCmd(flags *rootFlags) *cobra.Command {
	var host, resources, since string
	cmd := &cobra.Command{
		Use: "sync", Short: "Persist a fresh LAN snapshot, current job metadata, and lifecycle baseline",
		Example: strings.Trim(`
  bambu-pp-cli sync --resources snapshots,jobs,transitions --since 24h
  bambu-pp-cli sync --agent`, "\n"),
		RunE: func(cmd *cobra.Command, _ []string) error {
			selected := map[string]bool{}
			for _, resource := range strings.Split(resources, ",") {
				resource = strings.TrimSpace(resource)
				switch resource {
				case "snapshots", "jobs", "transitions":
					selected[resource] = true
				case "":
					return usageErr(fmt.Errorf("--resources cannot be empty"))
				default:
					return usageErr(fmt.Errorf("unsupported --resources value %q; use snapshots,jobs,transitions", resource))
				}
			}
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"would_sync": selected, "since": since}, flags)
			}
			var window time.Duration
			if since != "" {
				var err error
				window, err = cliutil.ParseDurationLoose(since)
				if err != nil {
					return usageErr(fmt.Errorf("invalid --since: %w", err))
				}
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			device, snapshot, err := fetchBambuStatus(ctx, flags, host)
			if err != nil {
				return err
			}
			result := map[string]any{"snapshots": 0, "jobs": 0, "transitions": 0, "metadata": 0, "observed_at": snapshot.ObservedAt}
			if selected["snapshots"] {
				if err := persistSnapshot(ctx, snapshot); err != nil {
					return configErr(fmt.Errorf("persist snapshot: %w", err))
				}
				result["snapshots"] = 1
			}
			if selected["jobs"] {
				result["jobs"] = 1
				if !selected["snapshots"] {
					if err := persistSnapshot(ctx, snapshot); err != nil {
						return configErr(fmt.Errorf("persist current job: %w", err))
					}
				}
			}
			if selected["jobs"] && (snapshot.State == "RUNNING" || snapshot.State == "PAUSE" || snapshot.State == "PREPARE") {
				metadata, metaErr := fetchLifecycleMetadata(ctx, device, snapshot)
				if metaErr == nil {
					if err := persistMetadata(ctx, snapshot, metadata); err != nil {
						return configErr(fmt.Errorf("persist metadata: %w", err))
					}
					result["metadata"] = 1
					result["weight_grams"] = metadata.WeightGrams
				} else {
					result["metadata_warning"] = sanitizePrinterError(metaErr, device.Serial)
				}
			}
			if selected["transitions"] {
				result["transitions_note"] = "LAN MQTT exposes current state only; run events watch to record future transitions"
			}
			if window > 0 {
				result["local_reporting_window"] = since
				result["since_note"] = "the window applies to subsequent local history queries; sync records only the current LAN observation"
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	addHostFlag(cmd, &host)
	cmd.Flags().StringVar(&resources, "resources", "snapshots,jobs,transitions", "Comma-separated local entities to refresh: snapshots,jobs,transitions")
	cmd.Flags().StringVar(&since, "since", "", "Local reporting window such as 24h or 7d; LAN sync records only the current observation")
	return cmd
}

func newBambuHistoryRunE(flags *rootFlags) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, _ []string) error {
		limit, _ := cmd.Flags().GetInt("limit")
		if limit < 1 || limit > 1000 {
			return usageErr(fmt.Errorf("--limit must be between 1 and 1000"))
		}
		if dryRunOK(flags) {
			return printJSONFiltered(cmd.OutOrStdout(), []any{}, flags)
		}
		sinceText, _ := cmd.Flags().GetString("since")
		duration := 30 * 24 * time.Hour
		if sinceText != "" {
			parsed, err := cliutil.ParseDurationLoose(sinceText)
			if err != nil {
				return usageErr(fmt.Errorf("invalid --since: %w", err))
			}
			duration = parsed
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
		jobs, err := history.JobsForPrinter(ctx, time.Now().UTC().Add(-duration), printerKey, limit)
		if err != nil {
			return configErr(err)
		}
		return printJSONFiltered(cmd.OutOrStdout(), jobs, flags)
	}
}

func newBambuMaintenanceCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "maintenance", Short: "Track local maintenance completion records and usage-derived context", Annotations: map[string]string{"mcp:local-write": "true"}, RunE: func(cmd *cobra.Command, _ []string) error {
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
		items, err := history.Maintenance(ctx, printerKey)
		if err != nil {
			return configErr(err)
		}
		completed, legacyIgnored := scopedMaintenanceBaselines(items)
		thresholds := []struct {
			Task  string
			Hours float64
		}{{"nozzle-clean", 50}, {"carbon-rods", 100}, {"lead-screws", 200}}
		forecast := make([]map[string]any, 0, len(thresholds))
		for _, threshold := range thresholds {
			since := time.Time{}
			if item, ok := completed[threshold.Task]; ok {
				since, _ = time.Parse(time.RFC3339Nano, fmt.Sprint(item["completed_at"]))
			}
			seconds, aggregateErr := history.CompletedPrintSeconds(ctx, since, printerKey)
			if aggregateErr != nil {
				return configErr(aggregateErr)
			}
			hours := float64(seconds) / 3600
			remaining := math.Max(0, threshold.Hours-hours)
			forecast = append(forecast, map[string]any{"task": threshold.Task, "interval_print_hours": threshold.Hours, "print_hours_since_completion": hours, "remaining_print_hours": remaining, "due": hours >= threshold.Hours, "last_completion": completed[threshold.Task]})
		}
		return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"forecasts": forecast, "completion_records": items, "legacy_baselines_ignored": legacyIgnored, "basis": "completed print duration recorded by lifecycle events; ambiguous legacy maintenance is visible but not used as a per-printer reset"}, flags)
	}}
	cmd.AddCommand(newBambuMaintenanceCompleteCmd(flags))
	return cmd
}

func scopedMaintenanceBaselines(items []map[string]any) (map[string]map[string]any, []string) {
	completed := map[string]map[string]any{}
	legacyIgnored := make([]string, 0)
	for _, item := range items {
		if legacy, _ := item["legacy_unscoped"].(bool); legacy {
			legacyIgnored = append(legacyIgnored, fmt.Sprint(item["task"]))
			continue
		}
		completed[fmt.Sprint(item["task"])] = item
	}
	return completed, legacyIgnored
}

func newBambuMaintenanceCompleteCmd(flags *rootFlags) *cobra.Command {
	var task, note string
	cmd := &cobra.Command{Use: "complete", Short: "Record completion of a named maintenance task", Example: "  bambu-pp-cli maintenance complete --task nozzle-clean --dry-run", Annotations: map[string]string{"pp:typed-exit-codes": "0,2", "mcp:local-write": "true"}, RunE: func(cmd *cobra.Command, _ []string) error {
		if dryRunOK(flags) {
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"would_complete": task}, flags)
		}
		if strings.TrimSpace(task) == "" {
			return usageErr(fmt.Errorf("--task is required"))
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
		if err := history.CompleteMaintenance(ctx, printerKey, task, note); err != nil {
			return configErr(err)
		}
		return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"printer_key": printerKey, "task": task, "completed": true, "completed_at": time.Now().UTC(), "note": note}, flags)
	}}
	cmd.Flags().StringVar(&task, "task", "", "Maintenance task name such as carbon-rods or nozzle-clean")
	cmd.Flags().StringVar(&note, "note", "", "Optional completion note")
	return cmd
}

func newObservationsPromotedCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{Use: "observations", Short: "List locally persisted Bambu observations", Example: "  bambu-pp-cli observations --agent", Annotations: map[string]string{"mcp:read-only": "true"}, RunE: func(cmd *cobra.Command, _ []string) error {
		if limit < 1 || limit > 1000 {
			return usageErr(fmt.Errorf("--limit must be between 1 and 1000"))
		}
		if dryRunOK(flags) {
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"would_list": "observations", "printer": flags.printerName, "limit": limit}, flags)
		}
		ctx, cancel := boundCtx(cmd.Context(), flags)
		defer cancel()
		s, err := store.OpenReadOnlyContext(ctx, defaultDBPath("bambu-pp-cli"))
		if err != nil {
			return configErr(err)
		}
		defer s.Close()
		var items []json.RawMessage
		if flags.printerName == "" {
			items, err = s.List("observations", limit)
		} else {
			serial, serialErr := selectedPrinterSerial(flags)
			if serialErr != nil {
				return serialErr
			}
			items, err = listObservationsForPrinter(ctx, s, bambu.PrinterKey(serial), limit)
		}
		if err != nil {
			return configErr(err)
		}
		decoded := make([]any, 0, len(items))
		for _, item := range items {
			var value any
			if json.Unmarshal(item, &value) == nil {
				decoded = append(decoded, value)
			}
		}
		return printJSONFiltered(cmd.OutOrStdout(), decoded, flags)
	}}
	cmd.Flags().IntVar(&limit, "limit", 100, "Maximum persisted observations to return (1-1000)")
	return cmd
}

func listObservationsForPrinter(ctx context.Context, s *store.Store, printerKey string, limit int) ([]json.RawMessage, error) {
	rows, err := s.DB().QueryContext(ctx, `SELECT data FROM "observations" WHERE printer_serial = ? ORDER BY synced_at DESC LIMIT ?`, printerKey, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]json.RawMessage, 0)
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		items = append(items, json.RawMessage(data))
	}
	return items, rows.Err()
}
