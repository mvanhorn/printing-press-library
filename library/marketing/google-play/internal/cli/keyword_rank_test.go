package cli

import "testing"

func TestNewKeywordRankCmd(t *testing.T) {
	cmd := newNovelKeywordRankCmd(&rootFlags{})
	if cmd.Use != "keyword-rank <term>" {
		t.Errorf("Use = %q", cmd.Use)
	}
	for _, f := range []string{"app", "scan", "db"} {
		if cmd.Flag(f) == nil {
			t.Errorf("keyword-rank missing --%s", f)
		}
	}
}

func TestKeywordRankRequiresApp(t *testing.T) {
	cmd := newNovelKeywordRankCmd(&rootFlags{})
	// term provided, but --app missing -> usage error
	err := cmd.RunE(cmd, []string{"merge puzzle"})
	if err == nil {
		t.Error("expected an error when --app is not provided")
	}
}

func TestKeywordRankDryRun(t *testing.T) {
	flags := &rootFlags{dryRun: true}
	cmd := newNovelKeywordRankCmd(flags)
	if err := cmd.RunE(cmd, []string{"merge puzzle"}); err != nil {
		t.Errorf("dry-run should not error, got %v", err)
	}
}
