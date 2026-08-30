package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestMealAddApplyRequiresLocalAuthenticatedStore(t *testing.T) {
	t.Parallel()

	flags := &rootFlags{configPath: t.TempDir() + "/config.toml"}
	cmd := newMealAddCmd(flags)
	cmd.SetArgs([]string{"--date", "2099-12-31", "--apply"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "not authenticated") {
		t.Fatalf("error = %v, want authentication preflight error", err)
	}
}

func TestMealDeleteApplyRequiresLocalAuthenticatedStore(t *testing.T) {
	t.Parallel()

	flags := &rootFlags{configPath: t.TempDir() + "/config.toml"}
	cmd := newMealDeleteCmd(flags)
	cmd.SetArgs([]string{"--event-id", "disposable-event", "--apply"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "not authenticated") {
		t.Fatalf("error = %v, want authentication preflight error", err)
	}
}

func TestMealUpdateApplyRequiresLocalAuthenticatedStore(t *testing.T) {
	t.Parallel()

	flags := &rootFlags{configPath: t.TempDir() + "/config.toml"}
	cmd := newMealUpdateCmd(flags)
	cmd.SetArgs([]string{"--event-id", "disposable-event", "--title", "Updated", "--apply"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "not authenticated") {
		t.Fatalf("error = %v, want authentication preflight error", err)
	}
}

func TestMealUpdateDryRunStillPreviewsWithoutWriting(t *testing.T) {
	t.Parallel()

	flags := &rootFlags{asJSON: true, dryRun: true, configPath: t.TempDir() + "/config.toml"}
	cmd := newMealUpdateCmd(flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--event-id", "disposable-event", "--title", "Updated"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("dry-run returned error: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("dry-run output is not JSON: %v\n%s", err, out.String())
	}
	if got, ok := payload["dry_run"].(bool); !ok || !got {
		t.Fatalf("dry_run = %#v, want true", payload["dry_run"])
	}
}

func TestMealAddDryRunStillPreviewsWithoutWriting(t *testing.T) {
	t.Parallel()

	flags := &rootFlags{asJSON: true, dryRun: true, configPath: t.TempDir() + "/config.toml"}
	cmd := newMealAddCmd(flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--date", "2099-12-31"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("dry-run returned error: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("dry-run output is not JSON: %v\n%s", err, out.String())
	}
	if got, ok := payload["dry_run"].(bool); !ok || !got {
		t.Fatalf("dry_run = %#v, want true", payload["dry_run"])
	}
}

func TestMealAddDefaultsToPreviewWithoutApply(t *testing.T) {
	t.Parallel()

	flags := &rootFlags{asJSON: true, configPath: t.TempDir() + "/config.toml"}
	cmd := newMealAddCmd(flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--date", "2099-12-31"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("default preview returned error: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("default preview output is not JSON: %v\n%s", err, out.String())
	}
	if got, ok := payload["dry_run"].(bool); !ok || !got {
		t.Fatalf("dry_run = %#v, want true", payload["dry_run"])
	}
}

func TestMealAddScaleFactorDryRunIncludesFactor(t *testing.T) {
	flags := &rootFlags{asJSON: true, dryRun: true, configPath: t.TempDir() + "/config.toml"}
	cmd := newMealAddCmd(flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--date", "2099-12-31", "--recipe", "Chicken", "--scale-factor", "1.5"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("dry-run with scale factor returned error: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("dry-run output is not JSON: %v\n%s", err, out.String())
	}
	if got, ok := payload["scale_factor"].(float64); !ok || got != 1.5 {
		t.Fatalf("scale_factor = %v, want 1.5", payload["scale_factor"])
	}
}

func TestMealAddScaleFactorStdinParses(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	oldStdin := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = oldStdin; r.Close(); w.Close() }()
	if _, err := w.Write([]byte(`{"date":"2099-12-31","recipe":"Chicken","scale_factor":2.0}`)); err != nil {
		t.Fatalf("write stdin: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close stdin: %v", err)
	}

	flags := &rootFlags{asJSON: true, dryRun: true, configPath: t.TempDir() + "/config.toml"}
	cmd := newMealAddCmd(flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--stdin"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("error: %v\n%s", err, out.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, out.String())
	}
	if got, ok := payload["scale_factor"].(float64); !ok || got != 2.0 {
		t.Fatalf("scale_factor = %v, want 2.0", payload["scale_factor"])
	}
}

func TestMealAddScaleFactorRequiresRecipe(t *testing.T) {
	flags := &rootFlags{configPath: t.TempDir() + "/config.toml"}
	cmd := newMealAddCmd(flags)
	cmd.SetArgs([]string{"--date", "2099-12-31", "--scale-factor", "1.5"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "scale factor requires a recipe") {
		t.Fatalf("error = %v, want scale factor requires a recipe error", err)
	}
}

func TestMealAddRejectsZeroAndNegativeScaleFactors(t *testing.T) {
	for _, value := range []string{"0", "-1"} {
		flags := &rootFlags{configPath: t.TempDir() + "/config.toml"}
		cmd := newMealAddCmd(flags)
		cmd.SetArgs([]string{"--date", "2099-12-31", "--recipe", "Chicken", "--scale-factor", value})
		err := cmd.Execute()
		if err == nil || !strings.Contains(err.Error(), "positive") {
			t.Errorf("value %s: error = %v, want positive scale factor error", value, err)
		}
	}
}

func TestMealUpdateScaleFactorDryRunIncludesFactor(t *testing.T) {
	flags := &rootFlags{asJSON: true, dryRun: true, configPath: t.TempDir() + "/config.toml"}
	cmd := newMealUpdateCmd(flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--event-id", "disposable-event", "--scale-factor", "0.5"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("dry-run with scale factor returned error: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("dry-run output is not JSON: %v\n%s", err, out.String())
	}
	if got, ok := payload["scale_factor"].(float64); !ok || got != 0.5 {
		t.Fatalf("scale_factor = %v, want 0.5", payload["scale_factor"])
	}
}

func TestMealUpdateOmissionLeavesScaleFactorOutOfPreview(t *testing.T) {
	flags := &rootFlags{asJSON: true, dryRun: true, configPath: t.TempDir() + "/config.toml"}
	cmd := newMealUpdateCmd(flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--event-id", "disposable-event", "--title", "Updated"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("dry-run without scale factor returned error: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("dry-run output is not JSON: %v\n%s", err, out.String())
	}
	if _, ok := payload["scale_factor"]; ok {
		t.Fatal("scale_factor should be absent when not provided on update")
	}
}

func TestMealDeleteDryRunStillPreviewsWithoutWriting(t *testing.T) {
	t.Parallel()

	flags := &rootFlags{asJSON: true, dryRun: true, configPath: t.TempDir() + "/config.toml"}
	cmd := newMealDeleteCmd(flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--event-id", "disposable-event"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("dry-run returned error: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("dry-run output is not JSON: %v\n%s", err, out.String())
	}
	if got, ok := payload["dry_run"].(bool); !ok || !got {
		t.Fatalf("dry_run = %#v, want true", payload["dry_run"])
	}
}

func TestMealDeleteDefaultsToPreviewWithoutApply(t *testing.T) {
	t.Parallel()

	flags := &rootFlags{asJSON: true, configPath: t.TempDir() + "/config.toml"}
	cmd := newMealDeleteCmd(flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--event-id", "disposable-event"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("default preview returned error: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("default preview output is not JSON: %v\n%s", err, out.String())
	}
	if got, ok := payload["dry_run"].(bool); !ok || !got {
		t.Fatalf("dry_run = %#v, want true", payload["dry_run"])
	}
}
