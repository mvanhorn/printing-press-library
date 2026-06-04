package cli

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestApplyProfileToFlags_AppliesUnsetFlagsOnly(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	var limit, orderBy string
	cmd.Flags().StringVar(&limit, "limit", "", "limit")
	cmd.Flags().StringVar(&orderBy, "order-by", "", "order-by")

	// User set --limit explicitly; profile must not override it.
	if err := cmd.Flags().Set("limit", "100"); err != nil {
		t.Fatal(err)
	}
	profile := &Profile{Values: map[string]string{"limit": "50", "order-by": "created_datetime"}}
	if err := ApplyProfileToFlags(cmd, profile); err != nil {
		t.Fatalf("ApplyProfileToFlags: %v", err)
	}
	if limit != "100" {
		t.Errorf("limit: profile must not override user-set flag (got %q, want 100)", limit)
	}
	if orderBy != "created_datetime" {
		t.Errorf("order-by: profile must apply when flag is unset (got %q, want created_datetime)", orderBy)
	}
}

func TestApplyProfileToFlags_SkipsReservedFlags(t *testing.T) {
	// Reserved: profile, config, help.
	cmd := &cobra.Command{Use: "test"}
	var prof, cfgPath string
	cmd.Flags().StringVar(&prof, "profile", "", "profile name")
	cmd.Flags().StringVar(&cfgPath, "config", "", "config path")

	p := &Profile{Values: map[string]string{"profile": "evil", "config": "/etc/shadow"}}
	if err := ApplyProfileToFlags(cmd, p); err != nil {
		t.Fatal(err)
	}
	if prof != "" {
		t.Error("profile flag must be reserved — a profile cannot self-reload")
	}
	if cfgPath != "" {
		t.Error("config flag must be reserved — a profile cannot redirect config path")
	}
}

func TestApplyProfileToFlags_IgnoresUnknownFlags(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	var x string
	cmd.Flags().StringVar(&x, "x", "", "x")

	// "y" doesn't exist on this command; must be silently skipped, not error.
	p := &Profile{Values: map[string]string{"x": "ok", "y": "ignored"}}
	if err := ApplyProfileToFlags(cmd, p); err != nil {
		t.Fatalf("ApplyProfileToFlags: %v", err)
	}
	if x != "ok" {
		t.Errorf("x: want ok, got %q", x)
	}
}

func TestApplyProfileToFlags_NilProfileNoop(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	if err := ApplyProfileToFlags(cmd, nil); err != nil {
		t.Errorf("nil profile: want nil, got %v", err)
	}
}
