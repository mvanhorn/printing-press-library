package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/bambu/internal/bambu"
)

func TestEventWatchContextHasNoDefaultDeadline(t *testing.T) {
	ctx, cancel := eventWatchContext(context.Background(), &rootFlags{timeout: time.Minute}, false)
	defer cancel()
	if _, ok := ctx.Deadline(); ok {
		t.Fatal("default events watch context unexpectedly has a deadline")
	}
}

func TestLifecycleOutputFiltersIntermediateTransitions(t *testing.T) {
	for _, kind := range []string{"started", "finished", "failed", "canceled"} {
		if !emitsLifecyclePayload(kind) {
			t.Fatalf("%s should emit", kind)
		}
	}
	for _, kind := range []string{"paused", "resumed"} {
		if emitsLifecyclePayload(kind) {
			t.Fatalf("%s should be persisted but not emitted", kind)
		}
	}
}

func TestLifecycleThumbnailNamesDoNotCollide(t *testing.T) {
	now := time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)
	plate := 1
	metadata := bambu.Metadata{PlateNumber: &plate}
	first := lifecycleThumbnailFilename(deviceContext{Serial: "PRINTERSERIAL001"}, bambu.Snapshot{TaskID: "task-a", ObservedAt: now}, metadata)
	second := lifecycleThumbnailFilename(deviceContext{Serial: "PRINTERSERIAL002"}, bambu.Snapshot{TaskID: "task-a", ObservedAt: now}, metadata)
	third := lifecycleThumbnailFilename(deviceContext{Serial: "PRINTERSERIAL001"}, bambu.Snapshot{TaskID: "task-b", ObservedAt: now}, metadata)
	if first == second || first == third || second == third {
		t.Fatalf("attachment names collided: %q %q %q", first, second, third)
	}
	for _, name := range []string{first, second, third} {
		if !strings.HasSuffix(name, "_plate_1.png") || strings.Contains(name, "PRINTERSERIAL") {
			t.Fatalf("unsafe attachment name %q", name)
		}
	}
}

func TestLifecyclePersistenceFailureDoesNotSuppressTerminalPayload(t *testing.T) {
	transition := bambu.Event{Kind: "finished", Snapshot: bambu.Snapshot{ObservedAt: time.Now().UTC(), State: "FINISH", JobName: "done"}}
	payload, err := prepareLifecycleTransition(context.Background(), deviceContext{}, transition, "", time.Time{}, func(context.Context, bambu.Event) error {
		return errors.New("store locked")
	})
	if err == nil || payload == nil || payload.Type != "print.finished" || len(payload.Warnings) != 1 || !strings.Contains(payload.Warnings[0], "store locked") {
		t.Fatalf("payload=%#v err=%v", payload, err)
	}
	intermediate, err := prepareLifecycleTransition(context.Background(), deviceContext{}, bambu.Event{Kind: "paused"}, "", time.Time{}, func(context.Context, bambu.Event) error {
		return errors.New("store locked")
	})
	if err == nil || intermediate != nil {
		t.Fatalf("intermediate=%#v err=%v", intermediate, err)
	}
}

func TestEventWatchContextHonorsExplicitTimeout(t *testing.T) {
	ctx, cancel := eventWatchContext(context.Background(), &rootFlags{timeout: time.Minute}, true)
	defer cancel()
	if _, ok := ctx.Deadline(); !ok {
		t.Fatal("explicit events watch timeout did not create a deadline")
	}
}

func TestEventConnectTimeoutUsesFallbackForUnboundedWatch(t *testing.T) {
	if got := eventConnectTimeout(&rootFlags{timeout: 0}); got != time.Minute {
		t.Fatalf("connect timeout = %v, want 1m", got)
	}
}

func TestLifecycleMonitorRunDirIsPrivateRunScopedPath(t *testing.T) {
	now := time.Date(2026, 7, 11, 18, 30, 45, 123, time.UTC)
	base := filepath.Join(t.TempDir(), "chosen run")
	got, err := lifecycleMonitorRunDir(base, now)
	if err != nil {
		t.Fatal(err)
	}
	want, err := filepath.Abs(base)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("run dir = %q, want %q", got, want)
	}
}

func TestLifecycleMonitorDryRunDoesNotCreateOutputDirectory(t *testing.T) {
	outputDir := filepath.Join(t.TempDir(), "must-not-exist")
	flags := &rootFlags{dryRun: true, asJSON: true}
	cmd := newBambuEventsMonitorCmd(flags)
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetArgs([]string{"--output-dir", outputDir})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(output.String(), outputDir) {
		t.Fatalf("dry-run leaked output directory: %s", output.String())
	}
	if _, err := os.Stat(outputDir); !os.IsNotExist(err) {
		t.Fatalf("dry-run created output directory: %v", err)
	}
}

func TestLifecycleStartWaitsForInitializedETA(t *testing.T) {
	startedAt := time.Date(2026, 7, 11, 17, 58, 36, 0, time.UTC)
	zero := 0
	remaining := 38
	start := bambu.Snapshot{ObservedAt: startedAt, State: "RUNNING", TaskID: "task", RemainingMinutes: &zero}
	pending := pendingLifecycleStart{snapshot: start, occurredAt: startedAt}
	if !lifecycleStartNeedsETA(start) {
		t.Fatal("zero remaining time should wait for ETA telemetry")
	}
	early := start
	early.ObservedAt = startedAt.Add(10 * time.Second)
	if lifecycleStartReady(pending, early) {
		t.Fatal("start became ready before ETA or fallback deadline")
	}
	initialized := early
	initialized.RemainingMinutes = &remaining
	if !lifecycleStartReady(pending, initialized) {
		t.Fatal("initialized ETA did not release start payload")
	}
	fallback := start
	fallback.ObservedAt = startedAt.Add(30 * time.Second)
	if !lifecycleStartReady(pending, fallback) {
		t.Fatal("fallback deadline did not release start payload")
	}
	terminal := start
	terminal.ObservedAt = startedAt.Add(time.Second)
	terminal.State = "FINISH"
	if !lifecycleStartReady(pending, terminal) {
		t.Fatal("terminal state did not release pending start payload")
	}
}

func TestLifecycleObjectJSONPreserves3MFIdentity(t *testing.T) {
	payload := lifecyclePayload{Job: lifecycleJob{Objects: []lifecycleObject{{ID: "170", Name: "3DBenchy.stl", Skipped: false}}}}
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"objects"`, `"id":"170"`, `"name":"3DBenchy.stl"`, `"skipped":false`} {
		if !bytes.Contains(encoded, []byte(want)) {
			t.Fatalf("payload %s missing %s", encoded, want)
		}
	}
}

func TestLifecycleMetadataUsesSolePrintableObjectAsDisplayName(t *testing.T) {
	payload := lifecyclePayload{Job: lifecycleJob{Name: "14min44s, Bambu PLA Basic, A1"}}
	weight := 10.29
	applyLifecycleMetadata(&payload, bambu.Metadata{ProjectName: "Benchy Bambu Pla Basic", ProfileName: "14min44s, Bambu PLA Basic, A1", WeightGrams: &weight, Objects: []bambu.Object{{ID: "170", Name: "3DBenchy.stl"}}})
	if payload.Job.Name != "Benchy Bambu Pla Basic" || payload.Job.SourceName != "14min44s, Bambu PLA Basic, A1" || payload.Job.ProjectName != "Benchy Bambu Pla Basic" || payload.Job.ProfileName != "14min44s, Bambu PLA Basic, A1" {
		t.Fatalf("job identity = %#v", payload.Job)
	}
	if len(payload.Job.Objects) != 1 || payload.Job.Objects[0].Name != "3DBenchy.stl" {
		t.Fatalf("objects = %#v", payload.Job.Objects)
	}
}

func TestLifecycleMetadataRetainsProjectNameForMultipleObjects(t *testing.T) {
	payload := lifecyclePayload{Job: lifecycleJob{Name: "Colored Accents"}}
	applyLifecycleMetadata(&payload, bambu.Metadata{Objects: []bambu.Object{{Name: "left.stl"}, {Name: "right.stl"}}})
	if payload.Job.Name != "Colored Accents" || payload.Job.SourceName != "" {
		t.Fatalf("job identity = %#v", payload.Job)
	}
}

func TestTerminalCarriesMatchingStartIdentity(t *testing.T) {
	weight := 9.89
	started := lifecycleJob{TaskID: "task", Name: "Benchy Bambu Pla Basic", SourceName: "14min44s, Bambu PLA Basic, A1", ProjectName: "Benchy Bambu Pla Basic", ProfileName: "14min44s, Bambu PLA Basic, A1", WeightGrams: &weight, Objects: []lifecycleObject{{ID: "170", Name: "3DBenchy.stl"}}}
	terminal := lifecycleJob{TaskID: "task", Name: "14min44s, Bambu PLA Basic, A1"}
	carryLifecycleJobIdentity(&terminal, &started)
	if terminal.Name != started.Name || terminal.SourceName != started.SourceName || terminal.ProjectName != started.ProjectName || terminal.WeightGrams == nil || *terminal.WeightGrams != weight || len(terminal.Objects) != 1 {
		t.Fatalf("terminal identity = %#v", terminal)
	}

	other := lifecycleJob{TaskID: "other", Name: "other"}
	carryLifecycleJobIdentity(&other, &started)
	if other.Name != "other" || len(other.Objects) != 0 {
		t.Fatalf("cross-job identity leaked: %#v", other)
	}
}
