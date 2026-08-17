package cli

import (
	"io"
	"strings"
	"testing"
)

func TestRootExposesProviderSurfaceWithoutSync(t *testing.T) {
	cmd, _, err := RootCmd().Find([]string{"sync"})
	if err == nil || cmd == nil || cmd.Name() == "sync" {
		t.Fatalf("sync must be absent: command=%v err=%v", cmd, err)
	}
	if cmd, _, err := RootCmd().Find([]string{"regimen", "snapshot"}); err != nil || cmd == nil {
		t.Fatalf("regimen snapshot missing: %v", err)
	} else {
		if cmd.Use != "snapshot <date>" {
			t.Fatalf("regimen snapshot use = %q", cmd.Use)
		}
		for _, forbidden := range []string{"products-file", "nutrients-file", "provider-schedule-file", "user-override-file", "as-of"} {
			if cmd.Flags().Lookup(forbidden) != nil {
				t.Fatalf("regimen snapshot exposes forbidden --%s flag", forbidden)
			}
		}
	}
	if cmd, _, err := RootCmd().Find([]string{"stack", "products"}); err != nil || cmd == nil || cmd.Parent().Hidden {
		t.Fatalf("stack products must be discoverable: %v", err)
	}
}

func TestSuppCoSurfaceHidesSearchAndRejectsInvalidSnapshotDate(t *testing.T) {
	root := RootCmd()
	search, _, err := root.Find([]string{"search"})
	if err == nil || search == nil || search.Name() == "search" {
		t.Fatalf("search must be absent: command=%v err=%v", search, err)
	}

	snapshot, _, err := root.Find([]string{"regimen", "snapshot"})
	if err != nil {
		t.Fatal(err)
	}
	if err := snapshot.RunE(snapshot, []string{"07/19/2026"}); err == nil || !strings.Contains(err.Error(), "YYYY-MM-DD") {
		t.Fatalf("invalid date error = %v", err)
	}
}

func TestSuppCoSurfaceRemovesUnownedCommandsAndRejectsRiskyGlobalOptions(t *testing.T) {
	for _, name := range []string{"api", "export", "feedback", "profile", "search", "sync", "which", "workflow"} {
		cmd, _, err := RootCmd().Find([]string{name})
		if err == nil || cmd == nil || cmd.Name() == name {
			t.Fatalf("%s must be absent: command=%v err=%v", name, cmd, err)
		}
	}
	agentContext, _, err := RootCmd().Find([]string{"agent-context"})
	if err != nil || agentContext == nil || !agentContext.Hidden {
		t.Fatalf("agent-context must remain available only as hidden PrintingPress metadata: command=%v err=%v", agentContext, err)
	}

	for _, tc := range []struct {
		name   string
		flags  rootFlags
		needle string
	}{
		{"deliver", rootFlags{dataSource: "live", deliverSpec: "webhook:https://example.test"}, "--deliver"},
		{"insecure", rootFlags{dataSource: "live", insecure: true}, "--insecure"},
		{"local data", rootFlags{dataSource: "local"}, "live stateless"},
		{"profile", rootFlags{dataSource: "live", profileName: "saved"}, "--profile"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := rejectSuppCoGlobalOptions(&tc.flags); err == nil || !strings.Contains(err.Error(), tc.needle) {
				t.Fatalf("option error = %v", err)
			}
		})
	}
}

func TestSuppCoSurfaceRejectsUnsupportedOutputTransformations(t *testing.T) {
	for _, args := range [][]string{
		{"--compact", "stack", "products"},
		{"--csv", "stack", "products"},
		{"--human-friendly", "stack", "products"},
		{"--plain", "stack", "products"},
		{"--quiet", "stack", "products"},
		{"--select", "id", "stack", "products"},
	} {
		root := RootCmd()
		root.SetOut(io.Discard)
		root.SetErr(io.Discard)
		root.SetArgs(args)
		if err := root.Execute(); err == nil || !strings.Contains(err.Error(), "complete normalized JSON contract") {
			t.Fatalf("args %v error = %v", args, err)
		}
	}
}

func TestSuppCoAgentModeDoesNotImplyAnUnsupportedExplicitCompactFlag(t *testing.T) {
	root := RootCmd()
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	root.SetArgs([]string{"--agent", "--dry-run", "stack", "products"})
	if err := root.Execute(); err != nil {
		t.Fatalf("agent dry-run error = %v", err)
	}
}

func TestSuppCoSurfaceHidesUnsupportedOutputFlags(t *testing.T) {
	root := RootCmd()
	for _, name := range []string{"compact", "csv", "human-friendly", "no-cache", "plain", "quiet", "select"} {
		flag := root.PersistentFlags().Lookup(name)
		if flag == nil || !flag.Hidden {
			t.Fatalf("--%s must be hidden", name)
		}
	}
}

func TestSuppCoDatedCommandsAllowNoArgumentDryRun(t *testing.T) {
	for _, args := range [][]string{
		{"--dry-run", "schedule"},
		{"--dry-run", "regimen", "snapshot"},
	} {
		root := RootCmd()
		root.SetOut(io.Discard)
		root.SetErr(io.Discard)
		root.SetArgs(args)
		if err := root.Execute(); err != nil {
			t.Fatalf("args %v dry-run error = %v", args, err)
		}
	}
}
