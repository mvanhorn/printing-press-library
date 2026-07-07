package cli

import "testing"

func TestNewKeywordHistoryCmd(t *testing.T) {
	cmd := newNovelKeywordHistoryCmd(&rootFlags{})
	if cmd.Use != "keyword-history <term>" {
		t.Errorf("Use = %q", cmd.Use)
	}
	for _, f := range []string{"app", "db"} {
		if cmd.Flag(f) == nil {
			t.Errorf("keyword-history missing --%s", f)
		}
	}
}

func TestKeywordHistoryRequiresApp(t *testing.T) {
	cmd := newNovelKeywordHistoryCmd(&rootFlags{})
	if err := cmd.RunE(cmd, []string{"merge puzzle"}); err == nil {
		t.Error("expected an error when --app is not provided")
	}
}

func TestKeywordHistoryDryRun(t *testing.T) {
	cmd := newNovelKeywordHistoryCmd(&rootFlags{dryRun: true})
	if err := cmd.RunE(cmd, []string{"merge puzzle"}); err != nil {
		t.Errorf("dry-run should not error, got %v", err)
	}
}
