package cli

import "testing"

func TestNewCompareCmd(t *testing.T) {
	cmd := newNovelCompareCmd(&rootFlags{})
	if cmd.Use != "compare <appId> <appId> [appId...]" {
		t.Errorf("Use = %q", cmd.Use)
	}
}

func TestCompareRequiresTwoArgs(t *testing.T) {
	cmd := newNovelCompareCmd(&rootFlags{})
	if err := cmd.RunE(cmd, []string{"com.only.one"}); err == nil {
		t.Error("expected an error when fewer than two appIds are given")
	}
}

func TestCompareDryRun(t *testing.T) {
	cmd := newNovelCompareCmd(&rootFlags{dryRun: true})
	if err := cmd.RunE(cmd, []string{"a", "b"}); err != nil {
		t.Errorf("dry-run should not error, got %v", err)
	}
}
