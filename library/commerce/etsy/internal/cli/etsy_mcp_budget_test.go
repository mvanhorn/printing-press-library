package cli

import (
	"bytes"
	"context"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/cliutil"
)

func TestEtsyMCPBudgetRefusal(t *testing.T) {
	for _, prefix := range [][]string{{"sync"}, {"market", "snapshot"}, {"listings", "bulk-edit"}, {"inventory", "low-stock", "--refresh"}} {
		t.Run(strings.Join(prefix, "-"), func(t *testing.T) {
			mktTestEnv(t)
			fake := newMktFakeEtsy(t)
			mktPointAtFake(t, fake)
			for _, child := range []string{"1", ""} {
				t.Setenv(cliutil.MCPChildEnvVar, child)
				args := append(append([]string{}, prefix...), "--ignore-budget", "--dry-run")
				_, err := mktRunCLI(t, args...)
				if child == "1" {
					if err == nil || ExitCode(err) != 2 || !strings.Contains(err.Error(), "--ignore-budget is not available over MCP") {
						t.Fatalf("got %v", err)
					}
				} else if err != nil {
					t.Fatalf("CLI behavior changed: %v", err)
				}
			}
			if fake.pageGets != 0 || fake.shopTotal() != 0 || fake.revGets != 0 {
				t.Fatal("budget refusal/dry-run made requests")
			}
		})
	}
}

func TestEtsyMCPBudgetClamps(t *testing.T) {
	for _, child := range []string{"1", ""} {
		for _, values := range [][]string{{"0", "0"}, {"-1", "-1"}, {"900", "80"}, {"12", "2"}} {
			t.Run(child+strings.Join(values, "-"), func(t *testing.T) {
				mktTestEnv(t)
				t.Setenv(cliutil.MCPChildEnvVar, child)
				root := RootCmd()
				var out, stderr bytes.Buffer
				root.SetOut(&out)
				root.SetErr(&stderr)
				root.SetArgs([]string{"sync", "--max-calls", values[0], "--max-pages", values[1], "--dry-run"})
				if err := root.Execute(); err != nil {
					t.Fatal(err)
				}
				cmd, _, err := root.Find([]string{"sync"})
				if err != nil {
					t.Fatal(err)
				}
				calls, _ := cmd.Flags().GetInt("max-calls")
				pages, _ := cmd.Flags().GetInt("max-pages")
				// These are the IntVar-backed values captured by the sync planner and
				// execution guard, inspected after RunE's hermetic dry-run branch.
				wantCalls, wantPages := 0, 0
				switch values[0] {
				case "-1":
					wantCalls, wantPages = -1, -1
				case "900":
					wantCalls, wantPages = 900, 80
				case "12":
					wantCalls, wantPages = 12, 2
				}
				changed := child == "1" && values[0] != "12"
				if changed {
					wantCalls, wantPages = 500, 50
				}
				if calls != wantCalls || pages != wantPages {
					t.Fatalf("effective sync caps=%d/%d want %d/%d", calls, pages, wantCalls, wantPages)
				}
				if strings.Contains(stderr.String(), "clamped") != changed {
					t.Fatalf("clamp note=%q", stderr.String())
				}
				if child == "1" && changed {
					fake := &syncBudgetFake{}
					guard := &etsySyncGuard{inner: fake, maxCalls: calls}
					for i := 0; i < 501; i++ {
						_, _ = guard.Get(context.Background(), "synthetic", nil)
					}
					if fake.calls.Load() != 500 {
						t.Fatalf("effective sync request cap=%d", fake.calls.Load())
					}
				}
			})
		}
	}
}

func TestEtsyMCPSnapshotClampPlan(t *testing.T) {
	for _, child := range []string{"1", ""} {
		t.Run("child="+child, func(t *testing.T) {
			mktTestEnv(t)
			t.Setenv(cliutil.MCPChildEnvVar, child)
			root := RootCmd()
			var out, stderr bytes.Buffer
			root.SetOut(&out)
			root.SetErr(&stderr)
			root.SetArgs([]string{"market", "snapshot", "--term", "synthetic", "--pages", "5", "--max-calls", "900", "--dry-run", "--json"})
			if err := root.Execute(); err != nil {
				t.Fatal(err)
			}
			plan := mktDecode(t, out.String())["plan"].(map[string]any)
			wantCalls, wantPages := float64(900), float64(5)
			if child == "1" {
				wantCalls, wantPages = 500, 3
			}
			if plan["max_calls"] != wantCalls || plan["pages"] != wantPages {
				t.Fatalf("effective snapshot plan: %v", plan)
			}
			if strings.Contains(stderr.String(), "clamped") != (child == "1") {
				t.Fatalf("note=%q", stderr.String())
			}
		})
	}
}

func TestEtsyMCPSnapshotResumeClamp(t *testing.T) {
	mktTestEnv(t)
	db := mktOpenFixtureStore(t)
	id := mktFxSnapshot(t, db.DB(), time.Now(), []string{"synthetic"}, 5, "partial", mktFxProg{})
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	t.Setenv(cliutil.MCPChildEnvVar, "1")
	out, err := mktRunCLI(t, "market", "snapshot", "--resume", strconv.FormatInt(id, 10), "--dry-run")
	if err != nil {
		t.Fatal(err)
	}
	plan := mktDecode(t, out)["plan"].(map[string]any)
	if plan["pages"] != float64(3) {
		t.Fatalf("saved pages bypassed ceiling: %v", plan)
	}
}
