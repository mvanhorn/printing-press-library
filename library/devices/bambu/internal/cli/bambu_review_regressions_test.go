package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/bambu/internal/bambu"
	"github.com/spf13/cobra"
)

func TestTerminalLifecyclePayloadOmitsStaleEstimate(t *testing.T) {
	remaining := 42
	observedAt := time.Date(2026, 7, 11, 8, 0, 0, 0, time.UTC)
	snapshot := bambu.Snapshot{ObservedAt: observedAt, State: "FINISH", JobName: "done", RemainingMinutes: &remaining}
	payload := buildLifecyclePayload(context.Background(), deviceContext{}, snapshot, "finished", "", observedAt.Add(-time.Hour))
	if payload.Job.RemainingMinutes != nil || payload.Job.EstimatedFinishAt != nil {
		t.Fatalf("terminal payload retained stale estimate: %#v", payload.Job)
	}
	if payload.Outcome == nil || payload.Outcome.Status != "finished" {
		t.Fatalf("terminal payload outcome = %#v", payload.Outcome)
	}
}

func TestStartedLifecyclePayloadOmitsUninitializedTelemetry(t *testing.T) {
	zero := 0
	staleLayer := 146
	observedAt := time.Date(2026, 7, 11, 17, 6, 29, 0, time.UTC)
	snapshot := bambu.Snapshot{ObservedAt: observedAt, State: "RUNNING", JobName: "new print", Percent: &zero, RemainingMinutes: &zero, CurrentLayer: &staleLayer}
	payload := buildLifecyclePayload(context.Background(), deviceContext{}, snapshot, "started", "", time.Time{})
	if payload.Job.RemainingMinutes != nil || payload.Job.CurrentLayer != nil || payload.Job.EstimatedFinishAt != nil {
		t.Fatalf("start payload retained uninitialized telemetry: %#v", payload.Job)
	}
	if len(payload.Warnings) == 0 || !strings.Contains(payload.Warnings[0], "not initialized") {
		t.Fatalf("start payload warnings = %#v", payload.Warnings)
	}
}

func TestAgentOutputHonorsExplicitLiveSource(t *testing.T) {
	flags := &rootFlags{agent: true, asJSON: true, outputSource: "live"}
	var output bytes.Buffer
	if err := printJSONFiltered(&output, map[string]any{"state": "RUNNING"}, flags); err != nil {
		t.Fatal(err)
	}
	var envelope struct {
		Meta map[string]any `json:"meta"`
	}
	if err := json.Unmarshal(output.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Meta["source"] != "live" {
		t.Fatalf("meta.source = %#v, want live", envelope.Meta["source"])
	}
}

func TestMirroredObservationsRemainDistinctAcrossPrinters(t *testing.T) {
	observedAt := time.Date(2026, 7, 11, 8, 0, 0, 0, time.UTC)
	firstKey := bambu.PrinterKey("PRINTERSERIAL006")
	secondKey := bambu.PrinterKey("PRINTERSERIAL007")
	firstData, err := mirroredObservationData("status", observedAt, firstKey, map[string]any{"state": "RUNNING"})
	if err != nil {
		t.Fatal(err)
	}
	secondData, err := mirroredObservationData("status", observedAt, secondKey, map[string]any{"state": "RUNNING"})
	if err != nil {
		t.Fatal(err)
	}
	var first, second map[string]any
	if err := json.Unmarshal(firstData, &first); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(secondData, &second); err != nil {
		t.Fatal(err)
	}
	if first["id"] == second["id"] || first["printer_serial"] != firstKey || second["printer_serial"] != secondKey {
		t.Fatalf("printer observations collided: first=%v second=%v", first, second)
	}
	if strings.Contains(string(firstData), "PRINTERSERIAL006") || strings.Contains(string(secondData), "PRINTERSERIAL007") {
		t.Fatal("mirrored observation leaked a raw printer serial")
	}
}

func TestMaintenanceChildRejectsLiveDataSource(t *testing.T) {
	root := &cobra.Command{Use: "bambu-pp-cli"}
	maintenance := &cobra.Command{Use: "maintenance"}
	complete := &cobra.Command{Use: "complete"}
	root.AddCommand(maintenance)
	maintenance.AddCommand(complete)
	err := validateBambuDataSource(complete, &rootFlags{dataSource: "live"})
	if err == nil || !strings.Contains(err.Error(), "persisted local history") {
		t.Fatalf("maintenance complete live validation error = %v", err)
	}
}

func TestSyncRejectsLocalDataSource(t *testing.T) {
	root := &cobra.Command{Use: "bambu-pp-cli"}
	syncCmd := &cobra.Command{Use: "sync"}
	root.AddCommand(syncCmd)
	err := validateBambuDataSource(syncCmd, &rootFlags{dataSource: "local"})
	if err == nil || !strings.Contains(err.Error(), "requires the printer LAN") {
		t.Fatalf("sync local validation error = %v", err)
	}
}

func TestMaintenanceCompleteCarriesLocalWriteAnnotation(t *testing.T) {
	cmd := newBambuMaintenanceCompleteCmd(&rootFlags{})
	if cmd.Annotations["mcp:local-write"] != "true" {
		t.Fatalf("annotations = %#v", cmd.Annotations)
	}
}

func TestObservationsRejectsUnboundedLimits(t *testing.T) {
	for _, limit := range []string{"-1", "0", "1001"} {
		cmd := newObservationsPromotedCmd(&rootFlags{dryRun: true})
		if err := cmd.Flags().Set("limit", limit); err != nil {
			t.Fatal(err)
		}
		err := cmd.RunE(cmd, nil)
		if err == nil || !strings.Contains(err.Error(), "between 1 and 1000") {
			t.Fatalf("--limit %s error = %v", limit, err)
		}
	}
}

func TestBambuNumericFlagsRejectInvalidBounds(t *testing.T) {
	flags := &rootFlags{dryRun: true}
	tests := []struct {
		name      string
		command   func() *cobra.Command
		flag      string
		value     string
		wantError string
	}{
		{name: "events max negative", command: func() *cobra.Command { return newBambuEventsWatchCmd(flags) }, flag: "max-events", value: "-1", wantError: "zero or greater"},
		{name: "printer max negative", command: func() *cobra.Command { return newBambuPrinterWatchCmd(flags) }, flag: "max-events", value: "-1", wantError: "zero or greater"},
		{name: "health zero", command: func() *cobra.Command { return newBambuPrinterHealthCmd(flags) }, flag: "history-limit", value: "0", wantError: "between 1 and 1000"},
		{name: "health oversized", command: func() *cobra.Command { return newBambuPrinterHealthCmd(flags) }, flag: "history-limit", value: "1001", wantError: "between 1 and 1000"},
		{name: "files list zero", command: func() *cobra.Command { return newBambuFilesListCmd(flags) }, flag: "limit", value: "0", wantError: "between 1 and 1000"},
		{name: "files list oversized", command: func() *cobra.Command { return newBambuFilesListCmd(flags) }, flag: "limit", value: "1001", wantError: "between 1 and 1000"},
		{name: "download zero", command: func() *cobra.Command { return newBambuFilesDownloadCmd(flags) }, flag: "max-bytes", value: "0", wantError: "must be between 1"},
		{name: "download oversized", command: func() *cobra.Command { return newBambuFilesDownloadCmd(flags) }, flag: "max-bytes", value: "134217729", wantError: "must be between 1"},
		{name: "history zero", command: func() *cobra.Command { return newNovelHistoryCmd(flags) }, flag: "limit", value: "0", wantError: "between 1 and 1000"},
		{name: "history oversized", command: func() *cobra.Command { return newNovelHistoryCmd(flags) }, flag: "limit", value: "1001", wantError: "between 1 and 1000"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cmd := test.command()
			if err := cmd.Flags().Set(test.flag, test.value); err != nil {
				t.Fatal(err)
			}
			err := cmd.RunE(cmd, nil)
			if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("--%s %s error = %v", test.flag, test.value, err)
			}
		})
	}
}
